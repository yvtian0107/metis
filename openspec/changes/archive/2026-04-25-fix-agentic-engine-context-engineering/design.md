## Context

Agentic ITSM 引擎的决策智能体（decision agent）在审批/驳回场景下存在多处上下文工程断裂，导致 `direct_first` 模式完全退化为 `ai_only` 的语义猜测。核心问题链：

1. **NodeID 断链** — `executeSinglePlan`、`executeParallelPlan`、`pendManualHandlingPlan` 三个活动创建站点均不设置 `activityModel.NodeID`，导致 `buildWorkflowContext` 的 `related_step` 和 `findRejectedEdgeTarget` 永远走 fallback 分支
2. **通过/驳回上下文不对称** — `buildInitialSeed` 只在驳回时注入 `rejected_activity_policy`，通过后 Agent 拿不到"下一步是什么"的结构化信息
3. **Activity form_data 不透出** — `activityFactMap` 从不返回活动级表单数据，驳回恢复时 Agent 无法知道"上次提交了什么"
4. **Seed 信息爆炸** — seed 与 `ticket_context` 工具返回存在大量重叠，浪费 token 预算
5. **formSchema 双轨** — PathBuilder prompt 只定义 6 种字段类型，intake form schema 有 17+ 种，排他网关条件引用 `form.xxx` 无法跨体系校验
6. **参与人预检路径分裂** — 服务台 `ValidateParticipants` 独立解析 workflow_json，与引擎的 `ParticipantResolver` 逻辑不共享

当前代码状态：`activityModel` 已有 `NodeID` 列（`gorm:"column:node_id;size:64"`），`FormData` 列也已存在；不需要数据库 migration，只需在写入时填充。

## Goals / Non-Goals

**Goals:**

- 恢复 Activity ↔ workflow_json Node 的双向映射，让 `buildWorkflowContext` 在 `direct_first` 模式下输出精确的 `related_step`、approved 出边目标、rejected 出边目标
- 让通过和驳回获得对称的路径引导上下文
- 将活动级 `form_data` 透出给 Agent，支撑驳回后的"知道上次提交了什么"场景
- 消除 seed 与 `ticket_context` 之间的重复信息
- 统一 formSchema 字段类型体系，让验证器能跨体系校验排他网关条件
- 统一参与人预检路径，消除 drift 风险

**Non-Goals:**

- 不改动经典引擎（classic engine）的任何逻辑
- 不做 Agent system prompt 的大规模重写（只补充 `node_id` 输出格式说明）
- 不引入新的数据库 migration
- 不改变 DecisionPlan 现有字段的语义（`node_id` 为 optional 新增）
- 不改变 confidence threshold 或 ReAct 轮次上限

## Decisions

### D1: NodeID 来源 — Agent 声明 + 路径合规验证

**选择**：在 `DecisionActivity` 结构体增加 `NodeID string` 字段（`json:"node_id,omitempty"`），由 Agent 在输出 DecisionPlan 时声明每个 activity 对应的 workflow_json 节点 ID。引擎在 `executeSinglePlan` / `executeParallelPlan` / `pendManualHandlingPlan` 写入 `activityModel.NodeID = da.NodeID`。

**验证层**：在 `validatePlan()` 中增加 node_id 合规检查：
- 若 `node_id` 非空，必须在 workflow_json 的 nodeMap 中存在
- 节点类型必须与 `da.Type` 兼容（如 process 节点对应 process 活动）
- 若 `node_id` 不合法，降级为空（log warning），不阻断执行

**备选 1（引擎自动匹配）**：引擎根据 activity type + participant 自动在 workflow_json 中查找匹配节点。

**为什么不选**：workflow_json 中可能有多个同类型节点（如两个 process 节点分别处理不同岗位），自动匹配产生歧义。让 Agent 声明 node_id 是最自然的——Agent 已经在读 workflow_context.human_steps 做推理，声明它选的是哪个节点只是把隐式推理显式化。

**备选 2（强制要求 node_id）**：node_id 为必填，缺失时拒绝执行。

**为什么不选**：破坏向后兼容性。现有 Agent prompt 不输出 node_id，升级期间所有决策都会失败。optional + 降级是更安全的过渡策略。

### D2: Approved 路径上下文注入 — 与 rejected_activity_policy 对称

**选择**：在 `buildInitialSeed` 中，当 completed activity 的 outcome 为正向时，查找其 NodeID 对应节点的 approved 出边目标，注入 `approved_next_step` 到 seed：

```
seed["approved_next_step"] = map[string]any{
    "target_node_id":    targetNodeID,
    "target_node_label": targetLabel,
    "target_node_type":  targetType,
    "instruction":       "workflow_json 的 approved 出边指向 {label}，应遵循此路径继续",
}
```

同时 `buildWorkflowContext` 中如果 completed.NodeID 有值且 outcome 为正向，也附加 `approved_edge_target` 到 `related_step` 中。

**为什么这样**：与 rejected 路径完全对称的设计，Agent 在通过后拿到结构化的"下一步是什么"，不再需要从 human_steps 全量列表中猜测。

### D3: Activity form_data 透出

**选择**：在 `activityFactMap` 中增加：

```go
if a.FormData != "" {
    entry["form_data"] = json.RawMessage(a.FormData)
}
```

**范围**：只在 `activityFactMap` 中添加，不额外在 seed 中重复。Agent 通过 `ticket_context` 工具看到每个活动的 form_data。

**考量**：form_data 可能较大，但只在有值时才返回，且只是 JSON 字段级别的增量。对于驳回恢复场景，这是不可或缺的信息（Agent 需要知道"申请人上次填了什么被驳回了"）。

### D4: Seed 去重策略

**选择**：seed 中只保留轻量锚点：

```
seed["completed_activity"] = map[string]any{
    "id":               completed.ID,
    "outcome":          completed.TransitionOutcome,
    "operator_opinion": completed.DecisionReasoning,
}
```

完整的活动事实（type、name、status、participants、form_data、source_decision 等）全部由 `ticket_context` 工具提供。

**原因**：当前 seed 中的 `completed_activity` 通过 `activityFactMap` 生成，包含 15+ 字段，与 `ticket_context` 返回的 `completed_activity` 完全重复。Agent 的 ReAct 循环第一步几乎总是调用 `ticket_context`，seed 中的冗余信息白白消耗 token。保留 `id + outcome + operator_opinion` 足以让 Agent 理解触发语境，完整事实按需通过工具获取。

**降级兼容**：`rejected_activity_policy` 和 `approved_next_step` 仍留在 seed 中（它们是策略指令，不是事实数据）。

### D5: formSchema 字段类型统一

**选择**：扩展 PathBuilder prompt 中的 formSchema 字段类型，与 intake form schema 对齐。在 prompt 的 `字段 type 可选值` 说明中增加：`email, url, radio, datetime, user_picker, dept_picker, rich_text, switch, multi_select, date_range, table`。

**验证器增强**：在 `ValidateWorkflow` 中新增一项检查——遍历排他网关出边的 condition，若 `field` 以 `form.` 开头，则回溯到网关的上游路径找到最近的 form 节点，检查其 `formSchema.fields` 是否包含对应的 key。缺失时产生 warning（不阻断，因为 form_data 也可能来自 intake form 或变量映射）。

**为什么 warning 不是 error**：排他网关条件可能引用 ticket 级 form_data（通过 intake form 提交），不一定来自 workflow_json 内的 form 节点。warning 已足够帮助 LLM 在 retry 时修正。

### D6: 参与人预检路径统一

**选择**：`tools/operator.go` 的 `ValidateParticipants` 方法改为调用 `engine.ParticipantResolver` 进行预检，而非自行解析 workflow_json。

具体做法：
1. `Operator` 结构体增加 `resolver *engine.ParticipantResolver` 字段
2. `ValidateParticipants` 遍历 workflow_json 的 process/form 节点，对每个 participant 调用 `resolver.Resolve(tx, 0, participant)` 进行试解析（ticketID=0 时跳过 requester 解析，只检查 position/user 可达性）
3. 删除 `ValidateParticipants` 中的独立 workflow 解析和逐类型检查逻辑

**备选（引入 ValidateOnly 方法）**：在 ParticipantResolver 上新增 `ValidateOnly(p Participant) error` 方法，只做可达性检查不实际分配。

**为什么不选**：现有 `Resolve` 的 ticketID=0 fallback 已能覆盖验证场景（requester 类型在无 ticket 时跳过是合理的——服务台提单前无法验证 requester）。新增方法带来的抽象不划算。

## Risks / Trade-offs

**[Agent 不输出 node_id]** → 降级为当前行为（NodeID 为空，related_step 走 fallback note）。这是可接受的——node_id 是渐进增强，不是硬性依赖。可通过 Agent system prompt 中增加输出格式说明来引导。

**[formSchema 扩展后 LLM 生成更复杂的表单]** → 类型虽然扩展了但 LLM 只在协作规范明确提及时才应使用高级类型。通过 prompt 约束"根据协作规范中描述的业务场景推断合理字段"即可控制。

**[Seed 去重后 Agent 依赖 ticket_context 工具]** → 如果 Agent 跳过 ticket_context 直接决策，会丢失 completed_activity 的详细信息。但 seed 中仍保留 outcome 和 operator_opinion 作为最小锚点，加上 rejected_activity_policy / approved_next_step 的策略指令，Agent 在简单场景下仍可直接决策。

**[ValidateParticipants 复用 Resolve 但 ticketID=0]** → requester 类型参与人在提单前无法验证，但这本就是合理的——提单前不知道 requester 是谁。当前独立路径也跳过了 requester 验证。

**[validator formSchema 引用检查回溯上游路径]** → 需要在 workflow 图上做 BFS 反向遍历找上游 form 节点。对于复杂工作流（多网关嵌套）可能找到多个 form 节点。策略：取所有可达 form 节点的 formSchema.fields 并集，只要 key 在并集中即通过。

## Open Questions

- **Agent prompt 中 node_id 输出说明的措辞**：需要平衡引导力度和避免过度约束。建议在 decision agent system prompt 的输出格式说明中加一句 `"node_id": "对应 workflow_json 中的节点 ID（可选，有 workflow_json 时建议填写）"`，具体措辞在实现时根据测试效果调整。

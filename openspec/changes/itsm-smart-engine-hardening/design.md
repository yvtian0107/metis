## Context

智能引擎（SmartEngine）基于 ReAct 循环实现 Agentic 决策，当前已具备完整的工具链（8 个 decision tools）和多种执行模式（串行/并签）。代码审查发现 6 类系统性问题需要加固：

1. **JSON 解析脆弱性**：ITSM 的 `extractJSON` 是简单括号计数，而 AI 知识编译模块已有 `jsonrepair` 增强版，两者未统一
2. **LLM 驱动层缺结构化输出**：`llm.ChatRequest` 无 `ResponseFormat`，无法利用 OpenAI 的 `json_schema` 或 Anthropic 的 prefill 技巧
3. **知识装载断路**：基础设施就绪但 `ServiceDefinition` 缺 `knowledge_base_ids` 字段
4. **direct_first 模式空转**：仅在 prompt 中提及"workflow_hints"，无实质注入
5. **BDD 对话层场景缺失**：引擎层覆盖完善，但从用户对话到创单到引擎触发的 E2E 未测试
6. **无重启恢复**：in_progress 票据在 server 重启后决策循环不会自动恢复

现有相关 spec：`ai-llm-client`（统一接口）、`itsm-smart-engine`（核心引擎）、`itsm-smart-react`（ReAct 循环）、`itsm-decision-tools`（8 个工具）、`itsm-service-definition`（服务定义模型）、`itsm-bdd-infrastructure`（BDD 框架）。

## Goals / Non-Goals

**Goals:**
- 统一 JSON 提取函数，所有 LLM 输出解析复用 jsonrepair 增强版
- llm.ChatRequest 支持 ResponseFormat，各驱动层按协议能力翻译
- ServiceDefinition 关联知识库，decision.knowledge_search 真正生效
- direct_first 模式从 WorkflowJSON 提取结构化 hints 注入 Agent
- 补齐 BDD 对话全链路、多对话模式、知识驱动路由场景
- 智能引擎决策循环可在 server 重启后自动恢复

**Non-Goals:**
- 不改造服务匹配算法（当前关键词匹配在服务数量少时够用）
- 不引入 Embedding/向量搜索的语义匹配
- 不为经典引擎添加恢复机制（已有 execution token）
- 不增加新的 LLM Provider 协议（如 Ollama），仅完善现有 OpenAI + Anthropic
- 不修改 Agent Runtime（gateway/executor_react），仅改智能引擎内嵌的轻量 ReAct 循环

## Decisions

### D1: extractJSON 统一到 `internal/llm/json.go`

**选择**：将 `knowledge_compile_longdoc.go` 的增强版提取到 `internal/llm/json.go` 导出为 `llm.ExtractJSON()`。

**替代方案**：
- A) 放到 `internal/pkg/jsonutil/` → 多一层包，实际只有 LLM 输出需要这个函数
- B) ITSM 直接 import AI 模块的函数 → 引入跨 App 依赖，违反架构分层

**理由**：所有需要 extractJSON 的场景都是处理 LLM 输出，放在 `llm` 包最自然。两个消费者（知识编译 + 智能引擎）都已依赖 `llm` 包。

### D2: ResponseFormat 分层策略

**选择**：在 `llm.ChatRequest` 增加 `ResponseFormat *ResponseFormat` 字段，各驱动层实现最优策略：

```
llm.ResponseFormat{
    Type:   "json_object" | "json_schema"
    Schema: any  // JSON Schema，仅 json_schema 时使用
}
```

驱动层翻译：
| 协议 | json_object | json_schema |
|------|------------|-------------|
| OpenAI | `response_format: {type: "json_object"}` | `response_format: {type: "json_schema", json_schema: {schema: ...}}` |
| Anthropic | system prompt 追加 JSON 约束 + assistant prefill `{` | 同 json_object（Anthropic 无 schema 约束） |

**替代方案**：
- A) 用 Tool-as-Output 模式（定义伪工具接收决策）→ 会混淆 ReAct 循环的终止条件（tool_call 意味着继续循环）
- B) 完全不做，只靠 extractJSON + jsonrepair → 对 Anthropic 够用，但放弃 OpenAI json_schema 的强保证

**理由**：分层策略让每个 provider 用最优方式，同时 extractJSON 作为 fallback 兜底。不破坏现有接口。

**关键约束**：ResponseFormat 仅在 LLM 返回文本（非 tool_call）时生效。ReAct 循环中间轮次走 tool_call，最终轮走 ResponseFormat。OpenAI 原生支持 tools + response_format 共存；Anthropic 的 prefill 技巧在存在 tool_use stop_reason 时不生效（仅在 end_turn 时生效），天然兼容。

### D3: ServiceDefinition 增加 knowledge_base_ids

**选择**：在 `itsm_service_definitions` 表增加 `knowledge_base_ids` TEXT 字段（JSON 数组），存储关联的知识库 ID 列表。

**理由**：
- 知识库数量少（通常 1-3 个），不需要关联表
- 与 `collaboration_spec`、`agent_config` 等 JSON 字段风格一致
- `decision.knowledge_search` 工具读取该字段传给 KnowledgeSearcher

### D4: direct_first 模式的 workflow_hints 提取

**选择**：从 WorkflowJSON 提取结构化步骤摘要，作为 `## 工作流参考路径` section 注入 system prompt。

提取逻辑：
1. 遍历 WorkflowJSON 的 nodes，按边（edges）关系构建执行顺序
2. 对每个 approve/process/action 节点，提取 label + participantType + positionCode
3. 对 exclusive_gateway 节点，提取条件分支
4. 组装为结构化文本（不是完整 BPMN，而是决策参考）

示例输出：
```
## 工作流参考路径

1. [审批] 直属主管审批 (participant: position_department, position: dept_manager)
2. [网关] 按 request_type 分支:
   - "网络支持" → [处理] 网络管理员处理 (participant: position_department, position: network_admin, dept: it)
   - "安全合规" → [处理] 安全管理员处理 (participant: position_department, position: security_admin, dept: it)
3. [完成] 流程结束

优先按此路径执行。如果路径无法覆盖当前场景，使用 AI 推理决定下一步。
```

**替代方案**：
- A) 将完整 WorkflowJSON 注入 → 太冗长，浪费 token，Agent 难以解析原始 BPMN
- B) 删除 direct_first 模式，只保留 ai_only → 放弃了对确定性场景的优化

### D5: 智能引擎恢复机制

**选择**：注册一个 scheduler 启动任务 `itsm-smart-recovery`，在 server 启动时执行一次。

逻辑：
1. 查询所有 `status = 'in_progress' AND engine_type = 'smart'` 的票据
2. 对每个票据，检查是否有 pending/in_progress 状态的活动
3. 如果没有活跃活动（可能是上次决策循环中断），提交 `itsm-smart-progress` 异步任务重新触发决策
4. 如果有活跃活动，跳过（等待正常 Progress 流程）

**替代方案**：
- A) 用 execution token 模式（类似经典引擎）→ 过度设计，智能引擎的"状态"就是最后一次活动完成
- B) 不做恢复，靠人工干预 → 不可接受，生产环境 server 重启是常态

### D6: BDD 新增场景规划

新增 feature 文件：

| Feature | 场景数 | 标签 | 对应 bklite-cloud 规划 |
|---------|--------|------|----------------------|
| `vpn_e2e_dialog_flow.feature` | 2 | @llm | VPN Request Main Flow |
| `vpn_dialog_coverage.feature` | 6 | @llm | VPN Dialog Coverage (scenario outline) |
| `service_desk_session_isolation.feature` | 2 | | Session Isolation |
| `service_knowledge_routing.feature` | 3 | @llm | Knowledge Change Flow |
| `smart_engine_recovery.feature` | 2 | | 新增：恢复机制 |

**不在本次覆盖**：AI Stress Test（需要大量服务目录数据，单独做）、Draft Confirmation Gating（现有 draft_confirm 状态机已有保障，优先级低）。

## Risks / Trade-offs

**[ResponseFormat + Tools 共存]** → OpenAI 文档明确支持；Anthropic 通过 prefill 实现，在 tool_use stop_reason 时 prefill 不生效，但这正好是我们要的行为（tool_call 轮次不需要 JSON 格式）。需要在两个 provider 上都写集成测试验证。

**[knowledge_base_ids 用 JSON 字段]** → 不支持外键约束和级联删除。如果知识库被删除，需要在知识库删除逻辑中清理引用，或在 knowledge_search 时容忍不存在的 KB ID。选择容忍（搜索时忽略不存在的 KB），简单且安全。

**[workflow_hints 提取的准确性]** → WorkflowJSON 格式多样（不同网关类型、嵌套分支），提取逻辑可能无法覆盖所有情况。兜底策略：提取失败时退化为 ai_only 模式，记录 warning 日志。

**[恢复机制的幂等性]** → 重新触发决策循环时，Agent 会重新调用 `decision.ticket_context` 获取完整上下文（包括已完成的活动），因此天然幂等。风险在于如果上次循环创建了活动但未来得及记录 timeline，可能重复创建。mitigation: `runDecisionCycle` 入口处检查是否已有 pending 活动。

**[BDD @llm 测试的稳定性]** → 依赖真实 LLM API，可能因模型输出变化导致偶发失败。现有的 deterministic 测试框架（mock LLM 返回固定 DecisionPlan）已覆盖引擎逻辑，@llm 测试重点验证 prompt + tool 协作的正确性，允许一定的不稳定性。

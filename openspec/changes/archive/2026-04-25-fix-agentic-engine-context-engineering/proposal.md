## Why

Agentic 引擎的决策智能体在审批/驳回场景下存在多处上下文工程断裂：Activity 创建时不写入 NodeID 导致 workflow_json 路径导航完全失效；通过和驳回的上下文投入严重不对称；活动级表单数据不透出给 Agent；流程图生成器的表单 schema 与 intake form schema 是两套体系；ValidateParticipants 和 ParticipantResolver 是两条独立路径。这些问题叠加后，决策智能体在 direct_first 模式下无法利用 workflow_json 做精确路径推理，驳回恢复和通过续行都退化为纯语义猜测。

## What Changes

- **NodeID 断链修复**：`executeSinglePlan`、`executeParallelPlan`、`pendManualHandlingPlan` 创建 Activity 时写入 NodeID，让 `buildWorkflowContext` 的 `related_step` 和 `findRejectedEdgeTarget` 生效
- **DecisionPlan 增加 node_id 字段**：Agent 输出 DecisionPlan 时声明活动对应的 workflow_json 节点 ID，建立 Activity↔Node 的双向绑定
- **通过路径上下文对称化**：新增 approved 出边目标节点注入（对称于 rejected_activity_policy），让 Agent 在通过后也能拿到结构化的"下一步是什么"
- **Activity form_data 透出**：`activityFactMap` 返回活动级 form_data，让 Agent 在驳回后能看到"上次提交了什么"
- **Seed 信息去重**：seed 中只放轻量锚点（completed_activity_id + outcome + operator_opinion），完整事实由 ticket_context 工具提供，减少 Agent 上下文噪声
- **流程图生成器 formSchema 与 intake_form_schema 对齐**：PathBuilder prompt 的 formSchema 字段类型扩展至与 intake form schema 一致，验证器增加 formSchema 可执行性检查
- **参与人预检路径统一**：服务台的 `ValidateParticipants` 复用 `ParticipantResolver`，消除两条独立解析路径的 drift 风险
- **验证器增加 formSchema 引用校验**：排他网关条件引用的 `form.xxx` 必须在上游 form 节点的 formSchema 中存在

## Capabilities

### New Capabilities
- `itsm-decision-node-binding`: Activity 与 workflow_json 节点的双向绑定机制（DecisionPlan node_id 字段 + Activity NodeID 写入 + 路径合规验证）
- `itsm-decision-context-symmetry`: 通过/驳回上下文对称化 + seed 去重 + activity form_data 透出

### Modified Capabilities
- `itsm-smart-engine`: 决策引擎核心增加 node_id 映射逻辑、approved 路径注入、seed 结构优化
- `itsm-smart-recovery`: 驳回恢复利用 NodeID 获取精确 rejected 出边目标，不再依赖泛化 allowed_recovery_paths
- `itsm-decision-tools`: ticket_context 工具返回活动级 form_data、减少与 seed 的重复
- `itsm-workflow-generate`: formSchema 类型扩展 + 验证器增加 formSchema 引用校验
- `itsm-service-desk-toolkit`: ValidateParticipants 统一到 ParticipantResolver 路径

## Impact

- **后端核心改动**：`internal/app/itsm/engine/smart.go`（executeSinglePlan、executeParallelPlan、pendManualHandlingPlan、buildInitialSeed）、`smart_tools.go`（activityFactMap、toolTicketContext）、`smart_workflow_context.go`、`validator.go`
- **后端定义改动**：`internal/app/itsm/definition/workflow_generate_prompt.go`（formSchema 扩展）、`workflow_generate_service.go`
- **后端工具改动**：`internal/app/itsm/tools/operator.go`（ValidateParticipants 重构）、`provider.go`（Agent system prompt 输出格式更新）
- **数据模型**：DecisionPlan/DecisionActivity 结构体增加 node_id 字段，无数据库 migration（NodeID 列已存在）
- **无前端改动**：所有改动在后端引擎层和 Agent prompt 层
- **无 Breaking Change**：node_id 字段为 optional，现有 Agent 输出不带 node_id 时降级为当前行为

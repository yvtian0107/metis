## Why

Agentic ITSM 引擎的 spec 设计完整度达 90%+，但实现层面存在"信仰断层"：后端不校验 Agent 流程产出的表单数据、不阻塞有缺陷的 AI 生成物、不兜底并行超时、不持续自愈。这些缺陷会导致审批卡死、流程死循环、脏数据污染决策、权限形同虚设。现在修复是因为核心 Agentic 链路（决策→审批→表单→生成→恢复）已基本就位，继续叠加功能只会加剧技术债的爆炸半径。

## What Changes

### 致命级修复（P0）

- **激活后端表单验证**：在 `variable_writer.go` 写入流程变量前调用 `form.ValidateFormData()`，拒绝非法数据进入决策上下文
- **补齐 `ensureContinuation` 调用**：`Start()` 触发首次决策循环、`Cancel()` 触发清理决策，确保所有状态变更都驱动下一步
- **工作流生成验证阻塞**：ValidateWorkflow 有 blocking error 时不持久化到数据库；新增环路检测（DFS）、死端检测（可达性分析）、参与者类型合法性校验

### 高危级修复（P1）

- **并行会签收敛超时**：`ensureContinuation` 增加超时检测，超时后触发 SLA 升级或自动取消兄弟活动
- **表单字段权限后端校验**：`variable_writer.go` 在写入前检查当前活动节点是否有权修改目标字段
- **恢复机制周期化**：`itsm-smart-recovery` 从 `@reboot` 改为周期调度（如每 10 分钟），持续扫描孤儿工单
- **user_picker / dept_picker 组件实现**：替换纯文本 Input 为 Combobox + UserAPI / TreeSelect + OrgAPI

### 中低危修复（P2）

- **SLA 紧急度阈值外置**：从硬编码移至 `EngineConfigProvider`
- **ExecutionMode 校验**：解析时验证只接受 `""` / `"single"` / `"parallel"`，非法值 warn 并降级为 single
- **handleComplete 写 NodeID**：终态活动绑定 end 节点 ID，恢复和追踪可定位结束点
- **批准/拒绝路径对称性增强**：批准路径增加等价的 `"应遵循此路径"` 约束
- **FormDesigner 权限编辑器 UI**：新增 permissions 面板，允许按节点配置字段 editable/readonly/hidden

## Capabilities

### New Capabilities

- `itsm-workflow-topology-validation`: 工作流拓扑完整性验证 — 环路检测、死端检测、节点可达性分析、参与者类型合法性校验
- `itsm-parallel-convergence-timeout`: 并行会签收敛超时机制 — 超时检测、SLA 感知升级、兄弟活动自动取消
- `itsm-form-permission-enforcement`: 表单字段权限后端强制执行 — 节点级写入校验、权限编辑器 UI

### Modified Capabilities

- `itsm-smart-engine`: `ensureContinuation` 补齐 Start/Cancel 路径调用；`handleComplete` 写 NodeID；`ExecutionMode` 校验；批准路径约束对称性增强
- `itsm-smart-recovery`: 从 `@reboot` 改为周期调度，增加持续自愈能力
- `itsm-workflow-generate`: 验证错误阻塞持久化，blocking vs warning 分级
- `itsm-form-validator`: 激活后端校验调用链，variable_writer 集成
- `itsm-form-renderer`: user_picker / dept_picker 组件真正实现（Combobox + API / TreeSelect + API）
- `itsm-form-designer`: 新增字段权限编辑器面板
- `itsm-decision-tools`: SLA 紧急度阈值外置到 EngineConfigProvider；similar_history limit 可配置
- `itsm-decision-context-symmetry`: 批准路径约束对称性增强
- `itsm-sla-monitor`: 与并行收敛超时机制集成

## Impact

- **后端**：`engine/smart.go`、`engine/variable_writer.go`、`engine/validator.go`、`engine/tasks.go`、`engine/smart_tools.go`、`engine/smart_workflow_context.go`、`definition/workflow_generate_service.go`、`form/validator.go`
- **前端**：`form-engine/field-renderers.tsx`、`form-engine/designer/field-property-editor.tsx`（新增 permissions panel）
- **API**：无新增端点，现有行为变更（工作流保存可能返回 400、表单提交可能返回 422）
- **数据库**：无 schema 变更，行为变更（验证失败拒绝写入）
- **配置**：新增 `EngineConfigProvider` 可选配置项（SLA 阈值、recovery 间隔、convergence timeout）

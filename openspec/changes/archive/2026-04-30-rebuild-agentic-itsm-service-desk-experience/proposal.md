## Why

当前 ITSM 服务台虽已完成部分 Agentic 语义修复，但仍存在明显体验断层：用户动作后的状态反馈不够实时、智能决策缺少可读解释、失败恢复仍偏工程视角。为了达到 OpenAI 级协作体验，需要一次性重构核心交互链路，而不是继续叠加兼容补丁。

## What Changes

- **BREAKING**: 重建 ITSM 服务台的状态与结果语义模型，彻底移除历史兼容映射与旧展示口径。
- **BREAKING**: 将审批/驳回/动作完成后的推进主路径统一为事件驱动决策调度，轮询仅保留故障恢复兜底。
- 新增“决策说明卡”能力：每轮 AI 决策输出可读依据、触发条件、下一步与可人工介入点。
- 新增“失败恢复编排”能力：失败态统一提供重试、转人工、撤回等可审计操作入口。
- 新增“服务台对话追问”能力：提单与补充信息阶段基于上下文主动追问缺失信息。
- 新增“决策质量观测”能力：提供通过率、驳回率、重试率、决策时长、恢复成功率等指标。
- 统一列表、详情、历史、审批待办的状态展示合同与刷新行为，确保跨页面语义一致。

## Capabilities

### New Capabilities
- `itsm-decision-explanation-ui`: 定义决策说明卡的展示合同、字段语义与可读性要求。
- `itsm-recovery-orchestration`: 定义失败态的统一恢复动作模型、权限边界与审计要求。
- `itsm-service-desk-conversational-intake`: 定义服务台提单阶段的对话式追问与信息补全行为。
- `itsm-decision-quality-observability`: 定义决策质量指标、口径一致性与运营可观测能力。

### Modified Capabilities
- `itsm-ticket-lifecycle`: 工单状态/终态结果语义重构为产品语义单一事实源。
- `itsm-approval-ui`: 审批提交后的即时反馈、可解释展示与恢复入口要求升级。
- `itsm-ticket-list-views`: 列表/历史/详情的状态口径、刷新行为与终态区分规则重构。
- `itsm-smart-continuation`: 主路径从轮询任务推进改为事件驱动的直接决策调度。
- `itsm-smart-recovery`: 从主推进职责收敛为故障恢复兜底，并对接新调度入口。
- `itsm-service-desk-toolkit`: 提单流程增加对话式追问与缺失信息补全协议。

## Impact

- 后端：`internal/app/itsm/domain`、`internal/app/itsm/ticket`、`internal/app/itsm/engine`、`internal/app/itsm/tools`。
- 前端：`web/src/apps/itsm/pages/tickets/*`、`web/src/apps/itsm/pages/tickets/approvals/*`、`web/src/apps/itsm/pages/service-desk/*`、`web/src/apps/itsm/components/*`、`web/src/apps/itsm/api.ts`。
- 数据与迁移：状态与结果字段属于破坏性变更，需一次性迁移旧数据，不保留兼容别名。
- 运行时：引入事件驱动主链路后，需要统一 timeout、panic recover、fresh DB session 与幂等防重策略。
- 运营治理：新增决策质量指标采集与报表读取路径，需要与现有监控体系对齐。

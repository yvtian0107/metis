## Why

智能引擎在审批/处理节点参与者解析为空时（岗位无人、部门不存在等），AI Agent 只能看到空 candidates 列表，无法有效分配工单。当前行为不确定——AI 可能 escalate、也可能反复重试直到失败。需要一个明确的、可配置的兜底策略，确保参与者缺失时工单始终有人处理。

## What Changes

- 在引擎配置（`EngineGeneralConfig`）中新增 `fallback_assignee`（用户 ID），管理员可在引擎配置页面设置兜底处理人
- 智能引擎执行决策时，若 AI 指定的 `participant_id` 无效或为 0，检查 `fallback_assignee` 配置：
  - 有配置 → 替换为兜底处理人，timeline 标记"参与者缺失，已转派兜底处理人"
  - 无配置 → 保持现有行为（AI 自行处理）
- 新增 BDD 场景验证：配置兜底人后参与者缺失自动转派、未配置时保持原有行为

## Capabilities

### New Capabilities
- `itsm-smart-fallback-assignee`: 智能引擎参与者缺失时的可配置兜底分配机制及 BDD 验证

### Modified Capabilities
- `itsm-engine-config`: `EngineGeneralConfig` 新增 `fallback_assignee` 字段，API 读写支持
- `itsm-smart-engine`: 决策执行层新增参与者兜底检查逻辑

## Impact

- `internal/app/itsm/engine_config_service.go` — EngineGeneralConfig 增加 FallbackAssignee 字段 + 读写
- `internal/app/itsm/engine/smart.go` — 决策执行时检查 participant_id 有效性，无效则查 fallback 配置
- `internal/app/itsm/features/vpn_participant_validation.feature` (new) — BDD 场景
- `internal/app/itsm/steps_vpn_participant_test.go` (new) — BDD steps 实现
- `internal/app/itsm/bdd_test.go` — 注册新 steps

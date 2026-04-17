## 1. 引擎配置层

- [x] 1.1 `engine_config_service.go`：`EngineGeneralConfig` 新增 `FallbackAssignee uint` 字段，`GetConfig` 用 `getConfigInt("itsm.engine.general.fallback_assignee", 0)` 读取
- [x] 1.2 `engine_config_service.go`：`UpdateConfigRequest.General` 新增 `FallbackAssignee uint`，`UpdateConfig` 中保存前校验用户存在且 active（0 允许直接保存表示清除），写入 `setConfigValue`
- [x] 1.3 `engine_config_service.go`：新增 `FallbackAssigneeID() uint` 方法，使 `EngineConfigService` 实现 `EngineConfigProvider` 接口

## 2. SmartEngine 兜底逻辑

- [x] 2.1 `engine/smart.go`：定义 `EngineConfigProvider` 接口（`FallbackAssigneeID() uint`），`SmartEngine` 增加 `configProvider EngineConfigProvider` 字段，`NewSmartEngine` 增加该参数
- [x] 2.2 `engine/smart.go`：`executeDecisionPlan` 中，对 `approve`/`process`/`form` 类型 Activity 且 `ParticipantID` 为 nil 或 0 时，调用 `configProvider.FallbackAssigneeID()` 获取兜底用户 ID
- [x] 2.3 `engine/smart.go`：兜底分配逻辑 — 校验兜底用户 active → 创建 assignment + 更新 assignee_id + 记录 `participant_fallback` timeline；用户无效则记录 warning timeline
- [x] 2.4 `app.go`：IOC 注入处更新 `NewSmartEngine` 调用，传入 `EngineConfigService` 作为 `EngineConfigProvider`

## 3. BDD Feature + Steps

- [x] 3.1 创建 `features/vpn_participant_validation.feature`：Background + 2 个 Scenario（兜底转派 + 参与者完整走完全流程）
- [x] 3.2 创建 `steps_vpn_participant_test.go`：`registerParticipantSteps(sc, bc)` 实现
- [x] 3.3 实现 Given 步骤：配置引擎兜底处理人（写 SystemConfig `itsm.engine.general.fallback_assignee`）
- [x] 3.4 实现 Given 步骤：创建使用缺失参与者工作流的 VPN 工单（复用 `missingParticipantWorkflowJSON` fixture）
- [x] 3.5 实现 Then 步骤：工单分配人为兜底处理人、timeline 包含 `participant_fallback` 事件

## 4. 注册与验证

- [x] 4.1 `bdd_test.go`：`initializeScenario` 调用 `registerParticipantSteps(sc, bc)`
- [x] 4.2 运行 `go test ./internal/app/itsm/ -run TestBDD -v` 验证新 scenarios 全部 green（LLM 端点恢复后重跑）

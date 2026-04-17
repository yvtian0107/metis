## Why

验证 VPN 开通申请在经典引擎和智能引擎下的完整行为链路。

经典引擎：申请人提交 → 排他网关根据 `request_kind` 路由到正确审批人（网络管理员 vs 安全管理员）→ 审批通过 → 工单完成。确定性路径，断言精确。

智能引擎：申请人创建工单 → SmartEngine 调用真实 LLM 生成 DecisionPlan → 高置信度自动执行 / 低置信度人工确认 → 审批 → 完成。非确定性路径，断言合法性。

两条赛道共享参与人、协作规范和 LLM 工作流生成基础设施，但引擎行为和断言策略完全不同。

参考来源：bklite-cloud `tests/bdd/itsm/features/vpn_request_main_flow.feature` + `vpn_workflow_progression_participant.feature`

## What Changes

### 经典引擎 BDD（2 scenarios）

- 创建 `features/vpn_classic_flow.feature`：网络支持路由 + 安全合规路由
- 创建 `steps_vpn_classic_test.go`：提交工单 → engine.Start() → engine.Progress() → 断言状态和分配

### 智能引擎 BDD（5 scenarios）

- 创建 `features/vpn_smart_flow.feature`：正常决策、低置信度 pending_approval、参与者缺失兜底、完整审批链路
- 创建 `steps_vpn_smart_test.go`：创建工单 → smartEngine.Start() → 断言决策合法性、活动状态、时间线

### 共享基础设施扩展

- 扩展 `bddContext`：增加 SmartEngine、llmConfig、决策断言辅助
- 新增 test doubles：`testAgentProvider`（DB Agent 记录 + env LLM config）、`testUserProvider`（内存 DB 查活跃用户）
- 新增共享步骤：协作规范定义、工单状态断言、活动类型断言
- 扩展 `vpn_support_test.go`：新增 `publishVPNSmartService`

### LLM 调用策略

全部使用真实 LLM（`.env.test` 提供 LLM_TEST_* 环境变量）：
- 经典引擎：Background 阶段 LLM 生成工作流 JSON（每 scenario 1 次）
- 智能引擎：Background 阶段 LLM 生成工作流上下文 + When 阶段 LLM 执行决策（每 scenario 2-3 次）
- 全量 BDD 一次运行约 10-16 次 LLM 调用

## Capabilities

### Modified Capabilities
- `itsm-bdd-infrastructure`: 扩展 bddContext 支持 SmartEngine、新增 test doubles 和共享步骤

### New Capabilities
- `vpn-classic-engine-bdd`: 经典引擎 VPN 主链路 BDD 测试（feature + steps）
- `vpn-smart-engine-bdd`: 智能引擎 VPN 决策 BDD 测试（feature + steps）

## Impact

- `internal/app/itsm/features/vpn_classic_flow.feature` (new)
- `internal/app/itsm/features/vpn_smart_flow.feature` (new)
- `internal/app/itsm/steps_vpn_classic_test.go` (new)
- `internal/app/itsm/steps_vpn_smart_test.go` (new)
- `internal/app/itsm/steps_common_test.go` (modified — bddContext 扩展 + 共享步骤)
- `internal/app/itsm/bdd_test.go` (modified — 注册新步骤)
- `internal/app/itsm/vpn_support_test.go` (modified — publishVPNSmartService)

## Dependencies

- vpn-bdd-infrastructure (已完成)
- vpn-bdd-workflow-fixture (已完成)

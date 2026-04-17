## 1. Feature 文件

- [x] 1.1 创建 `features/vpn_smart_engine_deterministic.feature`：Background 复用系统初始化 + 参与人 + 协作规范 + 发布智能服务
- [x] 1.2 Scenario: process 类型决策创建处理活动并指派参与者
- [x] 1.3 Scenario: action 类型决策创建自动动作活动（无参与者）
- [x] 1.4 Scenario: notify 类型决策创建通知活动
- [x] 1.5 Scenario: form 类型决策创建表单填写活动并指派参与者
- [x] 1.6 Scenario: escalate 类型决策创建升级活动
- [x] 1.7 Scenario: complete 类型决策直接完结工单
- [x] 1.8 Scenario: AI 熔断 — ai_failure_count 达到上限后拒绝决策
- [x] 1.9 Scenario: Cancel 取消有活跃活动的智能工单
- [x] 1.10 Scenario: 低置信度决策被管理员拒绝后不执行
- [x] 1.11 Scenario: 兜底用户已停用时记录 warning 而非分配

## 2. Steps 实现

- [x] 2.1 创建 `steps_vpn_smart_deterministic_test.go`，定义 `registerDeterministicSteps(sc, bc)`
- [x] 2.2 实现 When 步骤：执行指定类型的 crafted DecisionPlan（参数化 type + participant），调用 `ExecuteConfirmedPlan`
- [x] 2.3 实现 When 步骤：执行 complete 类型 crafted DecisionPlan
- [x] 2.4 实现 When 步骤：模拟 AI 熔断（设置 `ai_failure_count = MaxAIFailureCount`，调用 `RunDecisionCycleForTicket`）
- [x] 2.5 实现 When 步骤：Cancel 智能引擎工单（调用 `SmartEngine.Cancel`）
- [x] 2.6 实现 When 步骤：管理员拒绝 pending_approval 活动（直接更新 activity status = rejected）
- [x] 2.7 实现 Given 步骤：配置兜底处理人为已停用用户（创建 is_active=false 的 user，重建 SmartEngine）
- [x] 2.8 实现 Then 步骤：验证最新活动类型和状态（参数化 activity_type + status）
- [x] 2.9 实现 Then 步骤：验证 assignment 存在/不存在
- [x] 2.10 实现 Then 步骤：验证 timeline 包含指定 event_type
- [x] 2.11 实现 Then 步骤：验证工单 assignee_id 变化/不变
- [x] 2.12 实现 Then 步骤：验证 ai_failure_count 值
- [x] 2.13 实现 Then 步骤：验证所有活动和 assignment 状态为 cancelled

## 3. 注册与验证

- [x] 3.1 `bdd_test.go`：`initializeScenario` 中调用 `registerDeterministicSteps(sc, bc)`
- [x] 3.2 运行 `go test ./internal/app/itsm/ -run TestBDD -v` 验证所有 scenario green

## 1. 基础设施扩展（steps_common_test.go）

- [x] 1.1 bddContext 新增 `smartEngine *engine.SmartEngine` 和 `llmCfg llmConfig` 字段
- [x] 1.2 实现 `testAgentProvider`：从 DB 读 Agent 记录 + 从 llmCfg 读 LLM 连接信息，返回 `SmartAgentConfig`
- [x] 1.3 实现 `testUserProvider`：查 BDD 内存 DB 的 is_active 用户 + UserPosition + Position + Department，返回 `[]ParticipantCandidate`
- [x] 1.4 `reset()` 中构建 SmartEngine：`NewSmartEngine(testAgentProvider, nil, testUserProvider, noopSubmitter)`
- [x] 1.5 `newBDDContext()` 初始化 llmCfg（从环境变量读取 LLM_TEST_*）
- [x] 1.6 新增共享 Given 步骤 `^已定义 VPN 开通申请协作规范$`：将 `vpnCollaborationSpec` 存入 bddContext
- [x] 1.7 新增共享 Then 步骤 `^工单状态为 "([^"]*)"$`：从 DB 刷新 ticket 并断言 status
- [x] 1.8 新增共享 Then 步骤 `^工单状态不为 "([^"]*)"$`：断言 status 不等于给定值
- [x] 1.9 将现有步骤抽取为 `registerCommonSteps(sc, bc)` 函数，包含上述共享步骤

## 2. 经典引擎 BDD

- [x] 2.1 创建 `features/vpn_classic_flow.feature`：Background（参与人 + 协作规范 + 发布服务）+ 2 scenarios（网络支持路由 + 安全合规路由）
- [x] 2.2 创建 `steps_vpn_classic_test.go` 并实现 `registerClassicSteps(sc, bc)`
- [x] 2.3 实现 Given 步骤 `^已基于协作规范发布 VPN 开通服务（经典引擎）$`：调用 `publishVPNService(bc, bc.llmCfg)`
- [x] 2.4 实现 When 步骤 `^"([^"]*)" 提交 VPN 申请，访问原因为 "([^"]*)"$`：在 DB 创建 Ticket + classicEngine.Start()
- [x] 2.5 实现 Then 步骤 `^当前活动类型为 "([^"]*)"$`：查 TicketActivity 断言 activity_type
- [x] 2.6 实现 Then 步骤 `^当前活动分配给 "([^"]*)" 所属的 ([^/]+)/([^$]+)$`：查 TicketAssignment 断言 position.code + department.code
- [x] 2.7 实现 Then 步骤 `^当前活动未分配给 "([^"]*)"$`：断言 assignment 的 user_id 不等于指定用户
- [x] 2.8 实现 When 步骤 `^"([^"]*)" 认领并审批通过当前工单$`：查当前 Activity + classicEngine.Progress(outcome="approved")

## 3. 智能引擎 BDD

- [x] 3.1 在 `vpn_support_test.go` 中新增 `publishVPNSmartService(bc *bddContext) error`：LLM 生成 workflow_json + seed Agent 记录 + 创建 ServiceDefinition(engine_type=smart, agent_id)
- [x] 3.2 创建 `features/vpn_smart_flow.feature`：Background（参与人 + 协作规范 + 发布智能服务）+ 5 scenarios（正常决策×2 + pending_approval + 缺失参与者 + e2e 完整链路）
- [x] 3.3 创建 `steps_vpn_smart_test.go` 并实现 `registerSmartSteps(sc, bc)`
- [x] 3.4 实现 Given 步骤 `^已基于协作规范发布 VPN 开通服务（智能引擎）$`：调用 `publishVPNSmartService(bc)`
- [x] 3.5 实现 Given 步骤 `^"([^"]*)" 已创建 VPN 工单，访问原因为 "([^"]*)"$`：在 DB 创建 Ticket（不启动引擎）
- [x] 3.6 实现 Given 步骤 `^智能引擎置信度阈值设为 ([0-9.]+)$`：更新当前 service 的 AgentConfig
- [x] 3.7 实现 Given 步骤 `^"([^"]*)" 已创建 VPN 工单（使用缺失参与者的工作流）$`：使用静态 fixture workflow_json（approval 节点无 participant_type）
- [x] 3.8 实现 When 步骤 `^智能引擎执行决策循环$`：调用 `smartEngine.Start(ctx, bc.db, params)` 或 `smartEngine.RunDecisionCycleForTicket()`
- [x] 3.9 实现 Then 步骤 `^存在至少一个活动$`：查 TicketActivity 表 count > 0
- [x] 3.10 实现 Then 步骤 `^活动类型在允许列表内$`：断言 activity_type ∈ AllowedSmartStepTypes
- [x] 3.11 实现 Then 步骤 `^决策置信度在合法范围内$`：查 TicketActivity.ai_confidence ∈ [0, 1]
- [x] 3.12 实现 Then 步骤 `^若指定了参与人则参与人在候选列表内$`：查 TicketAssignment.user_id，若非 nil 则断言在 testUserProvider.ListActiveUsers() 结果内
- [x] 3.13 实现 Then 步骤 `^时间线应包含 AI 决策相关事件$`：查 TicketTimeline 存在 event_type 包含 "ai_decision" 的记录
- [x] 3.14 实现 Then 步骤 `^当前活动状态为 "([^"]*)"$`：查最新 TicketActivity 断言 status
- [x] 3.15 实现 Then 步骤 `^当前活动状态不为 "([^"]*)"$`：断言 status 不等于给定值
- [x] 3.16 实现 Then 步骤 `^活动记录中包含 AI 推理说明$`：断言 TicketActivity.ai_reasoning 非空
- [x] 3.17 实现 When 步骤 `^管理员确认该待确认决策$`：查 pending_approval Activity → 解析 ai_decision JSON → smartEngine.ExecuteConfirmedPlan()
- [x] 3.18 实现 When 步骤 `^当前活动的被分配人认领并审批通过$`：查当前 Activity 的 Assignment → smartEngine.Progress(outcome="approved")
- [x] 3.19 实现 When 步骤 `^智能引擎再次执行决策循环$`：调用 `smartEngine.RunDecisionCycleForTicket()`

## 4. 注册与集成

- [x] 4.1 修改 `bdd_test.go`：`initializeScenario` 调用 `registerCommonSteps(sc, bc)` + `registerClassicSteps(sc, bc)` + `registerSmartSteps(sc, bc)`
- [x] 4.2 删除 `features/example.feature` 中的 @wip 模板场景（如果不再需要）
- [x] 4.3 运行 `go test ./internal/app/itsm/ -run TestBDD -v` 验证 7 个 scenarios 全部 green

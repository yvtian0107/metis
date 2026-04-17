## MODIFIED Requirements

### Requirement: 共享 BDD test context
系统 SHALL 提供 `steps_common_test.go`，定义 `bddContext` 结构体作为所有 step definitions 的共享状态容器。bddContext SHALL 包含以下字段组：

**核心字段（已有）：**
- `db` (*gorm.DB) — 每 Scenario 独立的内存 SQLite
- `lastErr` (error) — 最近一次操作的错误

**引擎字段（扩展）：**
- `engine` (*engine.ClassicEngine) — 可工作的经典引擎实例
- `smartEngine` (*engine.SmartEngine) — 可工作的智能引擎实例
- `llmCfg` (llmConfig) — LLM 连接配置（从环境变量读取，跨 scenario 共享）

**参与人字段（已有）：**
- `users` (map[string]*model.User) — key 为身份标签
- `usersByName` (map[string]*model.User) — key 为 username
- `positions` (map[string]*org.Position) — key 为 position code
- `departments` (map[string]*org.Department) — key 为 department code

**工单生命周期字段（已有）：**
- `service` (*ServiceDefinition) — 当前场景的服务定义
- `priority` (*Priority) — 当前场景的优先级
- `ticket` (*Ticket) — 当前场景的工单
- `tickets` (map[string]*Ticket) — 多工单场景用，key 为别名

#### Scenario: bddContext 包含全部字段组
- **WHEN** 查看 `bddContext` 结构体定义
- **THEN** SHALL 包含核心、引擎（含 SmartEngine）、参与人、工单生命周期四组字段

#### Scenario: reset 清理所有字段并构建双引擎
- **WHEN** `reset()` 被调用
- **THEN** 所有 map 字段 SHALL 被重新初始化为空 map
- **AND** ClassicEngine SHALL 被重新构建（testOrgService + noopSubmitter）
- **AND** SmartEngine SHALL 被重新构建（testAgentProvider + testUserProvider + nil KnowledgeSearcher + noopSubmitter）
- **AND** ticket 和 service SHALL 被设为 nil

### Requirement: ClassicEngine 在 reset 中实例化
`bddContext.reset()` SHALL 创建可工作的 `ClassicEngine`，使用 `testOrgService`（查 BDD 内存 DB）和 `noopSubmitter`。

#### Scenario: engine 可驱动工单流转
- **WHEN** reset 完成后调用 `bc.engine.Start(ctx, bc.db, params)`
- **THEN** 引擎 SHALL 能解析 workflow JSON 并创建 Activity

## ADDED Requirements

### Requirement: testAgentProvider 实现
系统 SHALL 提供 `testAgentProvider` struct，实现 `engine.AgentProvider` 接口。

`GetAgentConfig(agentID)` SHALL：
1. 从 BDD 内存 DB 的 `ai_agents` 表读取 Agent 记录（system_prompt, temperature, max_tokens）
2. 从 `llmConfig` 获取 LLM 连接信息（baseURL, apiKey, model）
3. Protocol 固定为 "openai"
4. 返回 `SmartAgentConfig` 结合两者

#### Scenario: testAgentProvider 返回 DB+env 混合配置
- **WHEN** DB 中存在 Agent{ID:1, SystemPrompt:"你是...", Temperature:0.2, MaxTokens:2048}
- **AND** llmCfg 有 baseURL/apiKey/model
- **THEN** `GetAgentConfig(1)` SHALL 返回 SystemPrompt="你是..."、Temperature=0.2、BaseURL=llmCfg.baseURL

#### Scenario: testAgentProvider agent 不存在时返回错误
- **WHEN** DB 中不存在 agentID=99 的 Agent
- **THEN** `GetAgentConfig(99)` SHALL 返回错误

### Requirement: testUserProvider 实现
系统 SHALL 提供 `testUserProvider` struct，实现 `engine.UserProvider` 接口。

`ListActiveUsers()` SHALL 查询 BDD 内存 DB 中所有 `is_active=true` 的用户，关联 UserPosition → Position + Department，返回 ParticipantCandidate 列表。

#### Scenario: testUserProvider 返回所有活跃用户
- **WHEN** DB 中有 3 个 is_active=true 的用户，其中 2 个有 UserPosition 关联
- **THEN** `ListActiveUsers()` SHALL 返回 3 个 ParticipantCandidate
- **AND** 有 UserPosition 关联的用户 SHALL 包含 Department 和 Position 信息

### Requirement: SmartEngine 在 reset 中实例化
`bddContext.reset()` SHALL 创建可工作的 `SmartEngine`，使用：
- `testAgentProvider`（db + llmCfg）
- `testUserProvider`（db）
- `nil`（KnowledgeSearcher，测试不需要知识库）
- `noopSubmitter`（已有）

#### Scenario: smartEngine 可调用 LLM 执行决策
- **WHEN** reset 完成后，DB 中有 Agent 和 Ticket 记录
- **THEN** `bc.smartEngine.Start(ctx, bc.db, params)` SHALL 能完成决策循环

### Requirement: 共享步骤——协作规范定义
系统 SHALL 注册 godog Given 步骤 `^已定义 VPN 开通申请协作规范$`，将 `vpnCollaborationSpec` 常量存入 bddContext。

#### Scenario: 协作规范步骤可匹配
- **WHEN** feature 文件包含 `Given 已定义 VPN 开通申请协作规范`
- **THEN** godog SHALL 匹配到该步骤
- **AND** bddContext 中 SHALL 持有协作规范文本

### Requirement: 共享断言步骤——工单状态
系统 SHALL 注册 godog Then 步骤 `^工单状态为 "([^"]*)"$`，从 DB 刷新 ticket 并断言 status 字段。

#### Scenario: 工单状态精确匹配
- **WHEN** DB 中 ticket.status 为 "in_progress"
- **THEN** 步骤 `工单状态为 "in_progress"` SHALL pass
- **AND** 步骤 `工单状态为 "completed"` SHALL fail

### Requirement: 共享断言步骤——工单状态否定
系统 SHALL 注册 godog Then 步骤 `^工单状态不为 "([^"]*)"$`，断言 ticket.status 不等于给定值。

#### Scenario: 工单状态否定匹配
- **WHEN** DB 中 ticket.status 为 "in_progress"
- **THEN** 步骤 `工单状态不为 "failed"` SHALL pass

### Requirement: 步骤注册函数化
每个 steps_*_test.go SHALL 导出一个 `registerXxxSteps(sc *godog.ScenarioContext, bc *bddContext)` 函数。`initializeScenario` SHALL 调用 `registerCommonSteps`、`registerClassicSteps`、`registerSmartSteps`。

#### Scenario: initializeScenario 注册全部步骤组
- **WHEN** 查看 `initializeScenario` 函数
- **THEN** SHALL 调用三个 register 函数

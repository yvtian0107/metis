## MODIFIED Requirements

### Requirement: 共享 BDD test context

系统 SHALL 提供 `steps_common_test.go`，定义 `bddContext` 结构体作为所有 step definitions 的共享状态容器。

#### Scenario: bddContext 包含核心字段
- **WHEN** 查看 `bddContext` 结构体
- **THEN** 包含以下字段：db (*gorm.DB)、lastErr (error)
- **AND** 提供 `reset()` 方法在每个 Scenario 前重置状态

## ADDED Requirements

### Requirement: dialog 测试框架记录 toolResults

`dialogTestState` SHALL 新增 `toolResults []toolResultRecord` 字段，在 `EventTypeToolResult` 事件中记录工具名称、输出内容和是否为错误。用于测试失败时的调试诊断。

#### Scenario: toolResults 在 agent 执行后可用
- **WHEN** 服务台 Agent 处理用户消息，执行过程中产生工具调用和结果
- **THEN** `dialogTestState.toolResults` SHALL 包含每次工具调用的结果记录
- **AND** 每条记录包含 Name (string)、Output (string)、IsError (bool)

### Requirement: 工具调用计数断言 step

系统 SHALL 提供 BDD step `{tool} 被调用至少 {n} 次`，断言指定工具在当前 scenario 的 toolCalls 中出现次数 ≥ n。

#### Scenario: 计数断言通过
- **WHEN** toolCalls 中 "itsm.service_load" 出现 3 次
- **AND** 断言 "service_load 被调用至少 2 次"
- **THEN** 断言 SHALL 通过

#### Scenario: 计数断言失败
- **WHEN** toolCalls 中 "itsm.service_load" 出现 1 次
- **AND** 断言 "service_load 被调用至少 2 次"
- **THEN** 断言 SHALL 失败并报告实际调用次数

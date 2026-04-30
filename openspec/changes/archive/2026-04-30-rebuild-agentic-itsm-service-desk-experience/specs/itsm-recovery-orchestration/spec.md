## ADDED Requirements

### Requirement: 统一恢复动作模型
系统 SHALL 在失败态暴露统一恢复动作：`retry`、`handoff_human`、`withdraw`，并对每个动作定义可执行状态、权限与幂等规则。

#### Scenario: 失败态展示可执行恢复动作
- **WHEN** 工单进入 failed 或 decision_failed 状态
- **THEN** 系统仅展示当前用户有权限执行的恢复动作

#### Scenario: 重复点击恢复动作幂等
- **WHEN** 用户在短时间内重复触发同一恢复动作
- **THEN** 系统 SHALL 保证仅生效一次并返回同一结果

### Requirement: 恢复动作审计闭环
每次恢复动作 SHALL 写入审计记录与时间线，至少包含操作者、动作、参数摘要、执行结果、时间戳。

#### Scenario: 恢复动作完成后可追溯
- **WHEN** 用户执行转人工恢复
- **THEN** 时间线与审计日志均记录该操作与结果

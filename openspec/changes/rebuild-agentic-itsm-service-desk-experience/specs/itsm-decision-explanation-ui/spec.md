## ADDED Requirements

### Requirement: 决策说明卡结构化输出
系统 SHALL 为每一轮 SmartEngine 决策生成结构化说明卡，至少包含 `basis`（依据）、`trigger`（触发原因）、`decision`（本轮决策）、`next_step`（下一步）、`human_override`（人工介入点）字段，并随工单详情返回。

#### Scenario: 决策成功时生成说明卡
- **WHEN** SmartEngine 完成一轮有效决策
- **THEN** 系统返回并持久化包含完整字段的说明卡

#### Scenario: 决策降级为人工时仍有说明卡
- **WHEN** SmartEngine 因校验或策略约束降级为人工处置
- **THEN** 说明卡 SHALL 明确降级原因与建议人工动作

### Requirement: 说明卡与时间线一致性
说明卡内容 SHALL 与 ticket timeline 的关键事件保持一致，且支持按活动节点追溯。

#### Scenario: 时间线与说明一致
- **WHEN** 用户查看某次活动完成后的说明卡
- **THEN** 用户可在时间线找到对应触发事件与决策结果

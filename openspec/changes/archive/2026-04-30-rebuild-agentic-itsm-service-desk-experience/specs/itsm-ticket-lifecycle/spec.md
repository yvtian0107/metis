## MODIFIED Requirements

### Requirement: 工单状态枚举
工单状态 SHALL 使用产品语义状态集合：`submitted`、`waiting_human`、`approved_decisioning`、`rejected_decisioning`、`decisioning`、`executing_action`、`completed`、`rejected`、`withdrawn`、`cancelled`、`failed`。系统 MUST 同时维护终态结果字段 `outcome`，其值 SHALL 为 `approved`、`rejected`、`fulfilled`、`withdrawn`、`cancelled`、`failed` 之一；非终态时 MUST 为空。

#### Scenario: 初始状态
- **WHEN** 工单创建
- **THEN** 状态 SHALL 为 submitted
- **AND** outcome SHALL 为空

#### Scenario: 审批通过后进入决策中
- **WHEN** 人工审批活动提交 outcome=approved 并事务提交成功
- **THEN** 工单状态 SHALL 变为 approved_decisioning
- **AND** outcome SHALL 保持为空直到进入终态

#### Scenario: 终态一致性
- **WHEN** 工单进入 completed、rejected、withdrawn、cancelled 或 failed
- **THEN** outcome SHALL 与终态语义一致
- **AND** 系统 SHALL 禁止再进入非终态

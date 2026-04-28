## MODIFIED Requirements

### Requirement: 我的待办列表增强
"我的待办"、"我的工单"、"历史工单"和审批列表 SHALL 共享同一状态展示合同（`status`、`statusLabel`、`statusTone`、`outcome`），并提供统一的主动刷新入口。自动刷新 SHALL 仅作为观察兜底，不得作为流程推进机制。

#### Scenario: 列表状态语义一致
- **WHEN** 同一工单同时出现在待办与详情视图
- **THEN** 两处展示的 status/statusLabel/outcome SHALL 一致

#### Scenario: 主动刷新不改变筛选上下文
- **WHEN** 用户在待办列表点击刷新
- **THEN** 系统 SHALL 保持当前筛选、分页与关键词条件
- **AND** 刷新后仅更新数据结果

#### Scenario: 自动刷新仅兜底
- **WHEN** 自动刷新定时触发
- **THEN** 系统 SHALL 仅重取当前视图数据
- **AND** 不得触发流程推进 API 或隐式状态流转

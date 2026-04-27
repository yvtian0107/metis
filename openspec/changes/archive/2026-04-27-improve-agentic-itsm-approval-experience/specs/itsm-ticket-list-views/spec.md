## ADDED Requirements

### Requirement: 列表统一业务状态展示
工单列表、我的工单、我的待办、审批待办、历史工单和监控列表 SHALL 使用统一状态展示合同渲染状态。列表 SHALL 显示已同意决策中、已驳回决策中、AI 决策中、自动执行中、已通过、已驳回、已撤回、已取消、失败等业务状态，不得把驳回和通过都显示为“已完成”。

#### Scenario: 历史列表区分通过和驳回
- **WHEN** 历史列表中同时存在 outcome=approved 和 outcome=rejected 的终态工单
- **THEN** 通过工单 SHALL 显示“已通过”
- **AND** 驳回工单 SHALL 显示“已驳回”

#### Scenario: 撤回和取消分开展示
- **WHEN** 历史列表中同时存在 status=withdrawn 和 status=cancelled 的工单
- **THEN** 撤回工单 SHALL 显示“已撤回”
- **AND** 取消工单 SHALL 显示“已取消”

### Requirement: 列表刷新入口
工单列表类页面 SHALL 提供刷新按钮，点击后 SHALL 使用当前筛选、分页和搜索条件重新请求数据。刷新按钮 SHALL 显示加载态，且不得重置用户当前筛选条件。

#### Scenario: 刷新不重置筛选
- **WHEN** 用户在历史工单页面选择某个状态筛选并点击刷新
- **THEN** 系统 SHALL 按原状态筛选重新请求数据
- **AND** 当前页码和搜索关键词 SHALL 保持不变

### Requirement: 状态筛选使用业务状态
工单状态筛选 SHALL 使用新的业务状态枚举。系统 SHALL 移除用户可见的泛化 `completed` 筛选文案，改为已通过、已驳回、已撤回、已取消、失败等明确结果。

#### Scenario: 用户筛选已驳回
- **WHEN** 用户选择“已驳回”筛选
- **THEN** 列表 SHALL 请求并展示 status=rejected 或 outcome=rejected 的工单
- **AND** 不应混入 outcome=approved 的工单

## MODIFIED Requirements

### Requirement: Inline approve/deny actions
"我的审批"与工单详情审批交互 SHALL 统一为即时反馈模型：用户提交同意/驳回后，界面 MUST 立即关闭输入面板并进入对应决策中态展示；后端结果回写 SHALL 仅用于确认或纠正，不得阻塞即时状态反馈。详情页还 MUST 展示本轮决策说明入口与可执行恢复动作（当状态允许时）。

#### Scenario: 审批后立即进入决策中
- **WHEN** 用户在审批界面提交“通过”
- **THEN** 当前界面 SHALL 立即显示“通过后决策中”状态
- **AND** 行项或按钮状态 SHALL 同步更新为不可重复提交

#### Scenario: 驳回后立即进入决策中
- **WHEN** 用户提交“驳回”并附带意见
- **THEN** 当前界面 SHALL 立即显示“驳回后决策中”状态
- **AND** 系统 SHALL 在后台继续处理 API 回写

#### Scenario: 后端失败后的状态纠正
- **WHEN** 提交后 API 返回失败
- **THEN** 系统 SHALL 自动重取真实工单状态并提示错误
- **AND** 不得自动恢复到可重复提交的脏状态

#### Scenario: 决策说明与恢复入口可见
- **WHEN** 工单处于决策中或失败恢复相关状态
- **THEN** 详情页 SHALL 提供决策说明入口
- **AND** 若当前用户有权限 SHALL 显示恢复动作入口

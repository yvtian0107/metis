## ADDED Requirements

### Requirement: Token 管理页面
系统 SHALL 提供 API Tokens 管理页面（路径 `/observe/tokens`），展示当前用户所有未撤销 Token 的列表，并提供新建和撤销操作入口。

#### Scenario: 页面加载展示 Token 列表
- **WHEN** 用户导航至 `/observe/tokens`
- **THEN** 系统 SHALL 展示每个 Token 的卡片，包含：名称、前缀（`itk_xxxx••••••••`）、scope 标签（Personal）、创建时间、最近使用时间（或 "从未使用"）

#### Scenario: 无 Token 时展示空态
- **WHEN** 用户尚未创建任何 Token
- **THEN** 系统 SHALL 展示空态插图和说明文案，以及 "新建 Token" 主操作按钮

### Requirement: 新建 Token 流程
新建 Token 操作 SHALL 通过 Sheet（抽屉）展示表单，表单仅含 `name` 字段。创建成功后 SHALL 以高亮方式展示完整明文 Token，并配有醒目的一次性提示（"此 Token 仅显示一次，请立即复制保存"），同时提供一键复制按钮。

#### Scenario: 填写名称并提交
- **WHEN** 用户在 Sheet 中填写 Token 名称并点击 "生成 Token"
- **THEN** 系统 SHALL 调用 POST `/api/v1/observe/tokens`，成功后 Sheet 内容切换为明文展示区域

#### Scenario: 明文 Token 展示与复制
- **WHEN** Token 创建成功，Sheet 展示明文区域
- **THEN** 系统 SHALL 展示完整 Token 字符串、一键复制按钮和一次性提示；点击复制后 SHALL 有视觉反馈；关闭 Sheet 前 SHALL 再次提示"已复制了吗？"确认

#### Scenario: 超出数量上限
- **WHEN** 用户已有 10 个 Token，点击 "新建 Token"
- **THEN** 系统 SHALL 展示提示说明已达上限（10个），新建按钮 SHALL 禁用或显示为灰色状态

### Requirement: 撤销 Token 确认流程
撤销 Token 操作 SHALL 要求二次确认，确认文案 SHALL 明确说明撤销后所有使用该 Token 的接入服务将立即断开。

#### Scenario: 点击撤销触发确认
- **WHEN** 用户点击某 Token 卡片上的 "撤销" 按钮
- **THEN** 系统 SHALL 展示确认 Dialog，文案包含 Token 名称和 "撤销后无法恢复，所有使用此 Token 的服务将立即断开接入" 的警告

#### Scenario: 确认撤销
- **WHEN** 用户在确认 Dialog 中点击 "确认撤销"
- **THEN** 系统 SHALL 调用 DELETE `/api/v1/observe/tokens/:id`，成功后从列表中移除该 Token 卡片，Toast 提示 "Token 已撤销"

#### Scenario: 取消撤销
- **WHEN** 用户在确认 Dialog 中点击 "取消"
- **THEN** 系统 SHALL 关闭 Dialog，Token 状态不变

### Requirement: Token 管理页面菜单入口
Token 管理页 SHALL 通过侧边菜单导航访问，作为 Integrations 模块下的子菜单项。

#### Scenario: 菜单项可见
- **WHEN** 用户拥有 `observe:token:list` 权限
- **THEN** 侧边菜单 SHALL 显示 Integrations 目录及其下的 "API Tokens" 子菜单项

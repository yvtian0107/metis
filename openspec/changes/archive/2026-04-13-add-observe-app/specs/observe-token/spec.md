## ADDED Requirements

### Requirement: Token 创建
用户 SHALL 能够创建 Integration Token，每次创建时系统一次性返回完整明文 Token，此后不可再次获取。系统 SHALL 在同一响应中返回 `token`（完整明文，仅此一次）、`id`、`name`、`prefix`、`created_at`。

#### Scenario: 成功创建 Token
- **WHEN** 已认证用户 POST `/api/v1/observe/tokens`，body 含有效 `name`
- **THEN** 系统 SHALL 生成格式为 `itk_<32byte-hex>` 的 Token，将 bcrypt hash 和前8字符前缀写入数据库，返回 HTTP 201，响应 data 包含完整明文 `token` 字段

#### Scenario: 超出 Token 数量上限
- **WHEN** 用户已拥有 10 个未撤销的 Token，再次请求创建
- **THEN** 系统 SHALL 返回 HTTP 422，错误信息说明已达上限

#### Scenario: name 为空
- **WHEN** 请求 body 中 `name` 为空字符串或缺失
- **THEN** 系统 SHALL 返回 HTTP 400

### Requirement: Token 列表
系统 SHALL 提供 Token 列表接口，仅返回调用者自己的未撤销 Token，每条记录 SHALL 包含 `id`、`name`、`prefix`、`scope`、`last_used_at`（可为 null）、`created_at`，不得包含明文 Token 或 bcrypt hash。

#### Scenario: 获取 Token 列表
- **WHEN** 已认证用户 GET `/api/v1/observe/tokens`
- **THEN** 系统 SHALL 返回该用户所有未撤销 Token 的列表，每条记录不含 `token` 或 `token_hash` 字段

#### Scenario: 无 Token 时返回空列表
- **WHEN** 用户尚未创建任何 Token
- **THEN** 系统 SHALL 返回 HTTP 200，data 为空数组

### Requirement: Token 撤销
用户 SHALL 能够撤销自己的 Token。撤销为软删除（写入 `revoked_at` 时间戳），撤销后该 Token 立即失效，后续 ForwardAuth 验证 SHALL 拒绝。

#### Scenario: 成功撤销
- **WHEN** 已认证用户 DELETE `/api/v1/observe/tokens/:id`，且该 Token 属于该用户
- **THEN** 系统 SHALL 写入 `revoked_at`，返回 HTTP 200；该 Token 立即不可用于 ForwardAuth 验证

#### Scenario: 撤销他人 Token
- **WHEN** 用户尝试 DELETE 不属于自己的 Token ID
- **THEN** 系统 SHALL 返回 HTTP 404（不泄露他人 Token 存在信息）

#### Scenario: 撤销已撤销的 Token
- **WHEN** 用户尝试撤销已有 `revoked_at` 的 Token
- **THEN** 系统 SHALL 返回 HTTP 404

### Requirement: Token 数据模型
`integration_tokens` 表 SHALL 包含以下字段，并预留组织扩展字段：`id`（uint PK）、`user_id`（uint，外键 users）、`org_id`（*uint，nullable，未来 org token）、`scope`（string，默认 "personal"）、`name`（string）、`token_hash`（string，bcrypt）、`token_prefix`（string，8字符，用于前缀查找）、`last_used_at`（*time.Time）、`revoked_at`（*time.Time）、`created_at`（time.Time）。

#### Scenario: 数据库表自动创建
- **WHEN** 系统启动时 App.Models() 被调用
- **THEN** GORM AutoMigrate SHALL 确保 `integration_tokens` 表及所有字段存在

#### Scenario: org_id 字段预留
- **WHEN** 当前阶段创建 Token
- **THEN** `org_id` SHALL 为 NULL，`scope` SHALL 为 "personal"，逻辑保持正确

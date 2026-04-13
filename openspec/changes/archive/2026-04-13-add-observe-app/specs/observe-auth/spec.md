## ADDED Requirements

### Requirement: ForwardAuth 验证端点
系统 SHALL 提供 `GET /api/v1/observe/auth/verify` 端点，该端点 SHALL 不经过 JWT 或 Casbin 中间件，独立使用 Integration Token 鉴权，供 Traefik ForwardAuth 调用。

#### Scenario: 有效 Token 验证通过
- **WHEN** 请求携带 `Authorization: Bearer itk_<valid_token>` header
- **THEN** 系统 SHALL 返回 HTTP 200，并在 response header 中写入：`X-Metis-User-Id: <user_id>`、`X-Metis-Token-Id: <token_id>`、`X-Metis-Scope: personal`；同时更新该 Token 的 `last_used_at` 为当前时间

#### Scenario: Token 不存在或已撤销
- **WHEN** 请求携带的 Token 不存在、已撤销（`revoked_at` 不为 null）或格式错误
- **THEN** 系统 SHALL 返回 HTTP 401，不写入任何归属 header

#### Scenario: 缺少 Authorization header
- **WHEN** 请求不包含 `Authorization` header
- **THEN** 系统 SHALL 返回 HTTP 401

### Requirement: 数据归属 Header 注入
验证成功时，系统 SHALL 在 response header 中注入完整的数据归属信息，以支持下游 OTel Collector 将归属元数据写入 ClickHouse resource attributes。

#### Scenario: 个人 Token 归属 header
- **WHEN** 验证通过的 Token `scope` 为 "personal"
- **THEN** response header SHALL 包含 `X-Metis-User-Id`（string，用户 ID）、`X-Metis-Token-Id`（string，Token ID）、`X-Metis-Scope: personal`；`X-Metis-Org-Id` SHALL 为空字符串或不写入

### Requirement: 验证端点路由隔离
ForwardAuth 验证端点 SHALL 注册在独立的 Gin RouterGroup 上，不经过主 API 的 JWT 认证和 Casbin 权限检查中间件链。

#### Scenario: 无 JWT 的请求可正常鉴权
- **WHEN** 外部服务使用 Integration Token 调用 verify 端点，无 JWT token
- **THEN** 系统 SHALL 正常执行 Token 验证逻辑，不因缺少 JWT 而返回 401

### Requirement: Token 验证性能缓存
为降低 bcrypt 计算开销，系统 SHALL 对验证通过的 Token 结果使用内存短期缓存（TTL 60秒），缓存 key 为 token 明文，缓存 value 为 user_id 和 token_id。

#### Scenario: 缓存命中
- **WHEN** 同一 Token 在 60 秒内再次调用 verify 端点
- **THEN** 系统 SHALL 直接使用缓存结果，跳过 bcrypt 计算，返回 HTTP 200

#### Scenario: Token 撤销后缓存失效
- **WHEN** Token 被撤销后，缓存 TTL 内再次调用 verify
- **THEN** 系统 SHALL 在 TTL 到期后（最长 60 秒）返回 401；撤销操作 SHALL 主动清除该 Token 的缓存条目

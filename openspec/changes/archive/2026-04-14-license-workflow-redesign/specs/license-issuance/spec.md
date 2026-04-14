## ADDED Requirements

### Requirement: License 生命周期操作 API
系统 SHALL 在 License 路由下新增 Renewal、Upgrade、Suspend、Reactivate 端点，具体行为由 `license-lifecycle` 能力定义，本能力负责将其集成到现有签发服务中并记录审计日志。

#### Scenario: 续期 API 集成
- **WHEN** 用户调用 `POST /api/v1/license/licenses/:id/renew`
- **THEN** 系统 MUST 调用 lifecycle service 执行续期，成功后记录审计日志 action=`renew`

#### Scenario: 升级 API 集成
- **WHEN** 用户调用 `POST /api/v1/license/licenses/:id/upgrade`
- **THEN** 系统 MUST 调用 lifecycle service 执行升级，成功后记录审计日志 action=`upgrade`

### Requirement: 注册码来源校验
系统在签发 License 时，Registration Code MUST 来自 `license_registrations` 表，且未被其他 License 占用。

#### Scenario: 签发使用预注册码
- **WHEN** 用户提交签发请求，registrationCode 对应一条存在的 `LicenseRegistration` 记录
- **THEN** 系统 MUST 校验该 code 未被占用，创建 License 后将该注册码标记为已绑定

#### Scenario: 签发使用自动生成码
- **WHEN** 用户选择自动生成模式
- **THEN** 系统 MUST 先创建 `LicenseRegistration`（source=auto_generated），再执行签发

#### Scenario: 注册码已被占用
- **WHEN** 用户提交的 registrationCode 已被其他 License 使用
- **THEN** 系统 MUST 返回错误 "注册码已被使用"

### Requirement: 列表与详情扩展
系统 SHALL 在 License 列表和详情响应中包含 `lifecycleStatus` 字段，并在列表查询中支持按 `lifecycleStatus` 筛选。

#### Scenario: 列表返回 lifecycleStatus
- **WHEN** 用户请求 License 列表
- **THEN** 每个列表项 MUST 包含 `lifecycleStatus` 字段

#### Scenario: 按生命周期状态筛选
- **WHEN** 用户传入 `lifecycleStatus=active` 查询参数
- **THEN** 系统 MUST 仅返回 `lifecycle_status='active'` 的记录

## MODIFIED Requirements

### Requirement: License data model
系统 SHALL 提供 License 数据模型，存储于 `license_licenses` 表，包含以下字段：
- `ProductID` (uint, nullable FK → license_products) — 关联商品
- `LicenseeID` (uint, nullable FK → license_licensees) — 关联授权主体
- `PlanID` (uint, nullable) — 关联套餐（可选）
- `PlanName` (string, required) — 套餐名称快照
- `RegistrationCode` (string, required) — 客户端注册码，MUST 对应 `license_registrations.code`
- `ConstraintValues` (JSONText) — 功能约束值快照
- `ValidFrom` (time.Time, required) — 生效时间
- `ValidUntil` (*time.Time, nullable) — 过期时间，null 表示永久有效
- `ActivationCode` (text, unique) — base64url 编码的完整许可（payload + 签名）
- `KeyVersion` (int) — 签名使用的密钥版本
- `Signature` (text) — Ed25519 签名（base64url）
- `Status` (string) — `issued` 或 `revoked`，保留兼容
- `LifecycleStatus` (string) — `pending` / `active` / `expired` / `suspended` / `revoked`
- `OriginalLicenseID` (*uint, nullable) — 升级来源 License ID
- `SuspendedAt` (*time.Time, nullable) — 暂停时间
- `SuspendedBy` (*uint, nullable) — 暂停操作人
- `IssuedBy` (uint) — 签发操作人 ID
- `RevokedAt` (*time.Time, nullable) — 吊销时间
- `RevokedBy` (*uint, nullable) — 吊销操作人 ID
- `Notes` (text, optional) — 备注

#### Scenario: License record creation
- **WHEN** 系统创建一条 License 记录
- **THEN** 所有必填字段 MUST 被设置，`lifecycleStatus` 根据 `validFrom` 自动推导为 `active` 或 `pending`

#### Scenario: License with permanent validity
- **WHEN** `ValidUntil` 为 null
- **THEN** 该许可被视为永久有效，`lifecycleStatus` 不会自动变为 `expired`

### Requirement: Issue license
系统 SHALL 提供许可签发功能，通过 `POST /api/v1/license/licenses` 调用。

签发流程：
1. 校验商品 MUST 存在且 status 为 `published`
2. 校验授权主体 MUST 存在且 status 为 `active`
3. 获取商品当前密钥（isCurrent=true）MUST 存在
4. 校验 `registrationCode` MUST 存在于 `license_registrations` 且未被占用
5. 构建 LicensePayload
6. Canonicalize → Ed25519 签名 → 生成 ActivationCode
7. 在事务内创建 License 记录，`status` 设为 `issued`，`lifecycleStatus` 设为 `active` 或 `pending`
8. 将对应的 `LicenseRegistration` 标记为已绑定

#### Scenario: Successful issuance
- **WHEN** 用户提交有效的签发请求（已发布商品、活跃授权主体、有效注册码）
- **THEN** 系统 MUST 创建 License 记录，`lifecycleStatus` 为 `active` 或 `pending`，activationCode 包含有效签名

#### Scenario: Invalid registration code
- **WHEN** 用户提交的 registrationCode 不存在于 `license_registrations` 或已被占用
- **THEN** 系统 MUST 返回错误 "无效的注册码或注册码已被使用"

### Requirement: Export .lic file
系统 SHALL 提供 `.lic` 文件导出功能，通过 `GET /api/v1/license/licenses/:id/export` 调用。

#### Scenario: Successful export
- **WHEN** 导出状态为 `active` 的许可
- **THEN** 系统 MUST 返回包含 activationCode、publicKey（对应 License 的 `key_version`）和 meta 信息的 JSON 文件

#### Scenario: Export revoked license
- **WHEN** 用户尝试导出已吊销的许可
- **THEN** 系统 MUST 返回错误 "已吊销的许可不能导出"

#### Scenario: Export expired or suspended license
- **WHEN** 用户尝试导出 `lifecycleStatus` 为 `expired` 或 `suspended` 的许可
- **THEN** 系统 MUST 允许导出，但响应中 MUST 包含 `warning` 字段提示当前状态

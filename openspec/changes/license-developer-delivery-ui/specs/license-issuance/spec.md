# Capability: license-issuance

## Purpose

许可签发与管理后端能力 — 提供 License 数据模型、签名算法（Ed25519 + JSON canonicalization）、签发/吊销/导出/列表/详情 API。

## MODIFIED Requirements

### Requirement: License data model

系统 SHALL 提供 License 数据模型，存储于 `license_licenses` 表，包含以下字段：
- `ProductID` (uint, nullable FK → license_products) — 关联商品
- `LicenseeID` (uint, nullable FK → license_licensees) — 关联授权主体
- `PlanID` (uint, nullable) — 关联套餐（可选）
- `PlanName` (string, required) — 套餐名称快照
- `RegistrationCode` (string, required) — 客户端注册码
- `LicenseKey` (string, nullable) — 独立的 License 解密密钥（base64url，32 字符）
- `ConstraintValues` (JSONText) — 功能约束值快照
- `ValidFrom` (time.Time, required) — 生效时间
- `ValidUntil` (*time.Time, nullable) — 过期时间，null 表示永久有效
- `ActivationCode` (text, unique) — base64url 编码的完整许可（payload + 签名）
- `KeyVersion` (int) — 签名使用的密钥版本
- `Signature` (text) — Ed25519 签名（base64url）
- `Status` (string) — `issued` 或 `revoked`
- `IssuedBy` (uint) — 签发操作人 ID
- `RevokedAt` (*time.Time, nullable) — 吊销时间
- `RevokedBy` (*uint, nullable) — 吊销操作人 ID
- `Notes` (text, optional) — 备注

License 模型 SHALL 嵌入 `model.BaseModel` 以获得 ID、CreatedAt、UpdatedAt、DeletedAt 字段。

#### Scenario: License record creation
- **WHEN** 系统创建一条 License 记录
- **THEN** 所有必填字段（ProductID, LicenseeID, PlanName, RegistrationCode, ValidFrom, ActivationCode, KeyVersion, Signature, Status, IssuedBy）MUST 被设置，且 `LicenseKey` MUST 同时被生成

#### Scenario: License with permanent validity
- **WHEN** `ValidUntil` 为 null
- **THEN** 该许可被视为永久有效

### Requirement: Issue license

系统 SHALL 提供许可签发功能，通过 `POST /api/v1/license/licenses` 调用。

签发流程：
1. 校验商品 MUST 存在且 status 为 `published`
2. 校验授权主体 MUST 存在且 status 为 `active`
3. 获取商品当前密钥（isCurrent=true）MUST 存在
4. 生成独立的 `LicenseKey`
5. 构建 LicensePayload
6. Canonicalize → Ed25519 签名 → 生成 ActivationCode
7. 在事务内创建 License 记录，status 设为 `issued`

#### Scenario: Successful issuance
- **WHEN** 用户提交有效的签发请求（已发布商品、活跃授权主体、有效注册码）
- **THEN** 系统 MUST 创建 License 记录，status 为 `issued`，activationCode 包含有效签名，且 `licenseKey` 非空

#### Scenario: Product not published
- **WHEN** 尝试对未发布商品签发许可
- **THEN** 系统 MUST 返回错误 "只能对已发布商品签发许可"

#### Scenario: Licensee not active
- **WHEN** 尝试对已归档授权主体签发许可
- **THEN** 系统 MUST 返回错误 "授权主体必须为活跃状态"

#### Scenario: No current key
- **WHEN** 商品没有当前有效密钥
- **THEN** 系统 MUST 返回错误 "商品密钥不存在"

### Requirement: Export .lic file

系统 SHALL 提供 `.lic` 文件导出功能，通过 `GET /api/v1/license/licenses/:id/export` 调用，并支持可选查询参数 `format`：
- `format= v1`（或省略）：使用旧的单密钥派生 `SHA256(fileToken + ":" + registrationCode)` 加密。
- `format=v2`：使用双重密钥派生 `SHA256(fileToken + ":" + licenseKey + ":" + registrationCode)` 加密；若 `licenseKey` 为空，则 fallback 到 v1 逻辑。

`.lic` 文件 JSON 结构：
```json
{
  "version": 1,
  "activationCode": "<base64url>",
  "publicKey": "<base64 public key>",
  "meta": {
    "productCode": "...",
    "productName": "...",
    "licenseeName": "...",
    "validFrom": "2026-01-01T00:00:00Z",
    "validUntil": null,
    "issuedAt": "2026-04-10T12:00:00Z"
  }
}
```

响应 MUST 设置 `Content-Type: application/json` 和 `Content-Disposition: attachment; filename="<productCode>_<YYYYMMDD>.lic"`。

#### Scenario: Successful v2 export
- **WHEN** 导出状态为 `issued` 的许可并指定 `format=v2`
- **THEN** 系统 MUST 返回使用双重密钥加密的 `.lic` 文件，其内容包含 activationCode、publicKey 和 meta 信息

#### Scenario: Export revoked license
- **WHEN** 尝试导出已吊销的许可
- **THEN** 系统 MUST 返回错误 "已吊销的许可不能导出"

#### Scenario: Fallback to v1 for legacy licenses
- **WHEN** 对 `licenseKey` 为空的旧许可请求 `format=v2`
- **THEN** 系统 MUST 自动降级为 v1 单密钥加密导出

### Requirement: Bulk reissue licenses

系统 SHALL 提供批量重签功能，通过 `POST /api/v1/license/products/:id/bulk-reissue` 调用。请求体 `licenseIds` 为待重签的许可 ID 数组。当 `licenseIds` 为空数组时，系统 SHALL 自动选择该商品下所有使用旧版本密钥的生效许可（非 revoked 状态）进行重签；当 `licenseIds` 非空时，仅处理指定 ID。

重签过程中，若被重签的许可 `licenseKey` 为空，系统 SHALL 为其补发一个新的 `licenseKey`。

#### Scenario: 批量重签全部受影响许可
- **WHEN** 用户提交 `licenseIds: []` 的批量重签请求
- **THEN** 系统查询该商品下所有 key_version < 当前版本且 lifecycle_status != revoked 的许可，重新签名并更新 activationCode 和 signature；对无 licenseKey 的记录补发 licenseKey，返回实际处理的条数

#### Scenario: 批量重签指定许可
- **WHEN** 用户提交非空的 `licenseIds` 数组
- **THEN** 仅对数组中指定的许可执行重签，跳过已吊销或属于其他商品的记录；无 licenseKey 的记录补发 licenseKey

#### Scenario: 批量重签超限
- **WHEN** 用户提交的 `licenseIds` 数组长度超过 100
- **THEN** 系统返回 400 错误，提示单次处理数量超限

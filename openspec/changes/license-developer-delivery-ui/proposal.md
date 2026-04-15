## Why

当前 License 系统的 `.lic` 文件仅用 `registrationCode` 做对称加密，但前端没有向管理员明确说明交付给开发者的完整内容，导致管理员误以为需要把 `metis.yaml` 里的 `license_key_secret` 发给开发者。同时，为了增强 license 文件的安全性并建立更清晰的"License Key"概念，需要为每个 License 独立生成一个 `licenseKey`，并与 `registrationCode` 共同派生 `.lic` 文件的解密密钥，最后在前端统一展示所有开发者交付物。

## What Changes

- 为每个 License 增加独立的 `licenseKey` 字段（生成规则：随机 32 字符 base64url）。
- 修改 `.lic` 文件加密逻辑：解密密钥由 `fileToken`、`licenseKey`、`registrationCode` 三者共同派生（`SHA256(fileToken + ":" + licenseKey + ":" + registrationCode)`）。
- 在 License 详情页新增"开发者交付"区块，集中展示：
  - `licenseKey`（可复制）
  - `registrationCode`（可复制）
  - `publicKey`（可复制）
  - `.lic` 文件下载按钮
  - 验证示例代码（Go/JS）
- **BREAKING**: 旧版 `.lic` 文件使用旧派生逻辑，仍可通过旧接口或兼容模式下载；新的导出接口默认使用双重密钥加密。

## Capabilities

### New Capabilities
- `license-developer-delivery`: License 开发者交付物管理与前端展示能力，包括独立 licenseKey 生成、开发者交付卡片 UI、验证示例代码。

### Modified Capabilities
- `license-issuance`: License 签发时额外生成 `licenseKey`；`.lic` 导出时的加密密钥派生逻辑改为 `fileToken + licenseKey + registrationCode` 三重派生。
- `license-product`: 增加 `/api/v1/license/licenses/:id/verify-example` 公开/内部接口（或挂载在现有 License API 下）返回该 License 对应的验证示例代码模板。

## Impact

- 后端：`license_licenses` 表新增 `license_key` 字段；`LicenseService.IssueLicense`、`LicenseService.ExportLicFile`、`EncryptLicenseFile` 逻辑变更。
- 前端：`license/licenses/[id].tsx` 新增"开发者交付"卡片；新增复制按钮与代码展示组件。
- 兼容性：旧 `.lic` 文件不受影响；新的双重密钥 `.lic` 需要开发者同时获得 `licenseKey` 与 `registrationCode`。

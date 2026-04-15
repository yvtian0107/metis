## Context

当前 Metis License 系统的 `.lic` 文件使用 `registrationCode` 单一对称密钥派生 AES 密钥进行加密。管理员在将 License 交付给第三方开发者时，前端没有明确说明需要交付哪些内容，导致管理员误以为需要把服务器配置文件里的 `license_key_secret` 发给开发者。此外，单密钥模型在安全性上也不够清晰——`registrationCode` 同时承担了"注册绑定"和"解密文件"两个职责。

## Goals / Non-Goals

**Goals:**
- 为每个 License 引入独立的 `licenseKey`，建立明确的"开发者密钥"概念。
- `.lic` 文件加密改用 `licenseKey` + `registrationCode` 双重派生，增强安全性。
- 在前端 License 详情页提供统一的"开发者交付"区块，集中展示 licenseKey、registrationCode、publicKey 和 .lic 下载。
- 提供可复制的验证示例代码，降低开发者集成成本。

**Non-Goals:**
- 不修改 Ed25519 签名逻辑和 activationCode 结构。
- 不引入外部 License Server 或在线验证服务。
- 不废弃旧的 `.lic` 下载接口（保持向后兼容，但新 UI 走新接口）。

## Decisions

### 1. licenseKey 的生成方式
- **Decision**: 签发 License 时自动生成 32 字符的 base64url 随机字符串作为 `licenseKey`。
- **Rationale**: 长度足够抵抗暴力破解，base64url 无 padding 便于复制粘贴；无需用户手动输入，减少出错。
- **Alternative considered**: 使用 UUID v4。拒绝原因：base64url 更短，且 `registrationCode` 已经是 `RG-` 前缀，两者视觉区分度更好。

### 2. .lic 文件加密密钥的派生公式
- **Decision**: 新密钥派生公式为 `SHA256(fileToken + ":" + licenseKey + ":" + registrationCode)`。
- **Rationale**: 既保留 `fileToken` 作为产品标识 salt，又让 `licenseKey` 和 `registrationCode` 共同成为必要输入。缺少任意一个都无法解密，形成真正的"双因子"保护。
- **Alternative considered**: 只把 `licenseKey` 作为新密钥。拒绝原因：`registrationCode` 已经是已知交付物，直接废弃会导致老用户理解成本上升；两者共存更符合"注册码 + License Key"的传统认知。

### 3. 向后兼容策略
- **Decision**: 现有 `GET /api/v1/license/licenses/:id/export` 行为不变（仍用旧派生逻辑）；新增 `GET /api/v1/license/licenses/:id/export?format=v2` 使用双重密钥加密。
- **Rationale**: 避免破坏已经集成旧 `.lic` 格式的第三方系统；前端默认调用 v2。
- **Migration**: 旧 `.lic` 文件继续可用旧接口下载；新签发的 License 同时拥有 `licenseKey` 字段，但旧 `.lic` 导出时不使用它。

### 4. 前端"开发者交付"区块的位置
- **Decision**: 放在 License 详情页（`license/licenses/[id].tsx`）的"基本信息"下方，作为一个独立卡片。
- **Rationale**: 管理员进入 License 详情页的首要目的之一就是"把这个 License 交给开发者"，放在首屏最直观。

### 5. 验证示例代码的提供方式
- **Decision**: 前端硬编码一段 Go 和 JS 的示例代码（通过 Tabs 切换展示），代码逻辑与当前 `.lic` 解密流程完全一致。
- **Rationale**: 不需要额外后端接口，展示即时可用；代码只涉及标准库（Go: crypto/aes, crypto/sha256, crypto/cipher; JS: Web Crypto API）。

## Risks / Trade-offs

| Risk | Mitigation |
|---|---|
| 旧 `.lic` 与新 `.lic` 格式混淆 | 新 UI 明确标注"新版 License 文件（需要 License Key + 注册码）"；旧接口保留供兼容。 |
| licenseKey 泄露导致 `.lic` 被解密 | 风险与单密钥模型相同，但攻击面未扩大；licenseKey 和 registrationCode 需同时泄露才能解密。 |
| 数据库新增字段导致现有 License 记录 licenseKey 为空 | 批量重签或导出时若 `licenseKey` 为空，自动 fallback 到旧单密钥派生逻辑，或提示管理员先执行重签。 |
| 前端示例代码与后端实现不一致 | 示例代码直接复用后端 `crypto.go` 中的 `DeriveLicenseFileKey`、`decryptAESGCM`、`DecodeActivationCode` 逻辑，确保一致。 |

## Migration Plan

1. **数据库**: 新增 `license_key` 字段，允许旧记录为空。
2. **后端**: 修改 `IssueLicense` 生成 `licenseKey`；`ExportLicFile` 支持 `format=v2`；`EncryptLicenseFile` 增加支持双重派生的重载。
3. **前端**: 在 License 详情页新增"开发者交付"卡片；默认调用 `export?format=v2`。
4. **部署**: 无停机需求，纯新增字段和接口。
5. **Rollback**: 回滚前端到旧版 UI，或回滚后端接口去掉 `format=v2` 分支即可，不影响已存储的数据。

## Open Questions

- 是否需要在 License 列表页增加"复制 licenseKey"的快捷操作？（当前 scope 限制在详情页，可后续迭代）
- 是否需要为旧 License（无 licenseKey）提供一键"补发 licenseKey"的批量任务？

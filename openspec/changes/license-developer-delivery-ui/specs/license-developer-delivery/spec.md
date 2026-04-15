# Capability: license-developer-delivery

## Purpose

为 License 系统提供完整的"开发者交付"能力：为每个 License 生成独立的 `licenseKey`，修改 `.lic` 文件为双重密钥加密，并在前端提供统一的开发者交付卡片，展示 licenseKey、registrationCode、publicKey、.lic 下载及验证示例代码。

## ADDED Requirements

### Requirement: License key generation

系统 SHALL 在签发 License 时自动生成一个独立的 `licenseKey`。`licenseKey` 为 32 字符的 base64url（无 padding）随机字符串，存储于 `license_licenses.license_key` 字段。

#### Scenario: License includes licenseKey on creation
- **WHEN** 系统成功签发一条新 License
- **THEN** `licenseKey` 字段 MUST 被自动设置为非空的 32 字符 base64url 字符串

#### Scenario: License key uniqueness
- **WHEN** 系统生成 `licenseKey`
- **THEN** 该值 SHOULD 在概率上保证全局唯一（使用 crypto/rand 生成 24 字节后 base64url 编码）

### Requirement: Dual-key .lic file encryption

系统 SHALL 支持使用 `licenseKey` 与 `registrationCode` 共同派生 `.lic` 文件的 AES 解密密钥。派生公式为 `SHA256(fileToken + ":" + licenseKey + ":" + registrationCode)`。

#### Scenario: Export license file with dual-key encryption
- **WHEN** 用户请求导出 `format=v2` 的 `.lic` 文件
- **THEN** 系统 MUST 使用双重密钥派生逻辑加密 `.lic` 内容

#### Scenario: Decrypt v2 .lic file
- **WHEN** 开发者使用 `licenseKey`、`registrationCode` 和 `fileToken` 对 v2 `.lic` 执行解密
- **THEN** 解密 MUST 成功还原出 `{"activationCode": "...", "publicKey": "..."}`

#### Scenario: Missing licenseKey falls back to single-key
- **WHEN** 导出一条旧 License（`licenseKey` 为空）的 `.lic` 文件
- **THEN** 系统 MUST fallback 到旧的单密钥派生逻辑 `SHA256(fileToken + ":" + registrationCode)`

### Requirement: Developer delivery UI

系统 SHALL 在 License 详情页提供"开发者交付"区块，集中展示以下信息并提供一键复制功能：
- `licenseKey`
- `registrationCode`
- `publicKey`
- `.lic` 文件下载按钮
- 验证示例代码（Go / JavaScript Tabs 切换）

#### Scenario: View developer delivery card
- **WHEN** 用户打开 License 详情页
- **THEN** 页面 MUST 显示"开发者交付"卡片，包含上述所有内容

#### Scenario: Copy licenseKey
- **WHEN** 用户点击 licenseKey 旁的复制按钮
- **THEN** `licenseKey` MUST 被写入剪贴板，并弹出复制成功提示

#### Scenario: Download v2 .lic file
- **WHEN** 用户点击".lic 文件下载"按钮
- **THEN** 浏览器 MUST 下载使用双重密钥加密的 `.lic` 文件

### Requirement: Verification example code

系统 SHALL 在"开发者交付"卡片中展示可复制的验证示例代码，代码 MUST 覆盖以下步骤：
1. 从 `.lic` 文件名提取 `fileToken`
2. 使用 `licenseKey` + `registrationCode` + `fileToken` 派生 AES 密钥
3. AES-GCM 解密得到 `activationCode` 和 `publicKey`
4. Base64url 解码 `activationCode` 得到 payload 和 `sig`
5. 使用 `publicKey` 对 canonicalized payload 进行 Ed25519 签名验证

#### Scenario: Show Go example
- **WHEN** 用户切换到 Go 示例 Tab
- **THEN** 页面 MUST 展示完整的 Go 验证代码片段

#### Scenario: Show JavaScript example
- **WHEN** 用户切换到 JavaScript 示例 Tab
- **THEN** 页面 MUST 展示完整的 JavaScript 验证代码片段

## 1. Database & Model

- [x] 1.1 在 `internal/app/license/model.go` 的 `License` 结构体中新增 `LicenseKey string` 字段（gorm 标签 `size:43;index`）
- [x] 1.2 创建数据库 migration（或依赖 GORM AutoMigrate），确保 `license_licenses` 表存在 `license_key` 列

## 2. Backend Crypto & Export

- [x] 2.1 在 `internal/app/license/crypto.go` 中新增 `DeriveLicenseFileKeyV2(registrationCode, fileToken, licenseKey string) []byte`，实现 `SHA256(fileToken + ":" + licenseKey + ":" + registrationCode)` 派生
- [x] 2.2 修改 `EncryptLicenseFile` 或新增 `EncryptLicenseFileV2`，支持传入 `licenseKey` 并使用 `DeriveLicenseFileKeyV2` 加密；当 `licenseKey` 为空时 fallback 到旧逻辑
- [x] 2.3 修改 `internal/app/license/license_service.go` 的 `ExportLicFile`，支持 `format` 参数：`v1`（默认）走旧逻辑，`v2` 走双重密钥逻辑；`v2` 且 `licenseKey` 为空时自动 fallback 到 v1
- [x] 2.4 修改 `internal/app/license/license_handler.go` 的 `Export`，从 query 参数读取 `format` 并透传给 `ExportLicFile`

## 3. Backend Issuance & Reissue

- [x] 3.1 在 `internal/app/license/license_service.go` 中新增 `generateLicenseKey() (string, error)`，使用 `crypto/rand` 生成 24 字节后 base64url 编码为 32 字符
- [x] 3.2 修改 `issueLicenseInTx`，在创建 License 记录前调用 `generateLicenseKey()` 并赋值给 `license.LicenseKey`
- [x] 3.3 修改 `BulkReissueLicenses`，在重签前检查 `detail.LicenseKey`，若为空则生成并补发新的 `licenseKey`
- [x] 3.4 更新 `LicenseResponse`（如需要）将 `LicenseKey` 加入响应，或确认详情接口已通过 `License.ToResponse()` 暴露该字段

## 4. Frontend Developer Delivery UI

- [x] 4.1 在 `web/src/apps/license/pages/licenses/[id].tsx` 新增"开发者交付"卡片（`DeveloperDeliveryCard` 或内联 JSX），位于"基本信息"下方
- [x] 4.2 在卡片中展示 `licenseKey`、`registrationCode`、`publicKey`，每个字段配复制按钮（使用 `navigator.clipboard` + toast）
- [x] 4.3 在卡片中新增".lic 文件下载"按钮，调用 `/api/v1/license/licenses/${id}/export?format=v2`
- [x] 4.4 在卡片中新增验证示例代码区块，使用 Tabs 组件切换 Go / JavaScript 代码片段；代码覆盖 `.lic` 解密、activationCode 解码、Ed25519 验签完整流程

## 5. Build & Verification

- [x] 5.1 运行 `go build -tags dev ./cmd/server/` 确认后端编译通过
- [x] 5.2 运行 `go test ./internal/app/license/...` 确认现有测试通过（如新增测试则一并运行）
- [x] 5.3 运行 `cd web && bun run build` 和 `bun run lint` 确认前端编译和 lint 通过
- [x] 5.4 手动验证：签发新 License → 详情页可见 licenseKey → 下载 v2 .lic → 使用提供的 JS/Go 示例代码成功解密并验证签名

## 6. Frontend Layout Refactor (方向 C: 左右分栏)

- [x] 6.1 重构 `web/src/apps/license/pages/licenses/[id].tsx` 为 lg:grid-cols-12 左右分栏布局
- [x] 6.2 左侧 8 列：状态 Banner（suspended/revoked Alert）、基础信息 2×2 Grid、功能约束
- [x] 6.3 右侧 4 列：Sticky 开发者快捷交付面板，包含 LicenseKey（默认 masked，可切换显示）、注册码、公钥、下载 .lic、查看验证示例按钮
- [x] 6.4 新增 Sheet Drawer，点击"查看验证示例"后从右侧滑出，展示密钥、下载按钮、Go/JS/Python Tabs 代码
- [x] 6.5 响应式适配：平板/移动端右侧边栏变为普通卡片，位于主内容之后
- [x] 6.6 移除页面内原有的内联验证示例代码区块，统一收进 Sheet

## ADDED Requirements

### Requirement: License 生命周期状态展示与操作
系统 SHALL 在许可列表页和详情页展示 `lifecycleStatus`，并提供 Renewal（续期）、Upgrade（升级）、Suspend（暂停）、Reactivate（恢复）操作入口。

#### Scenario: 列表页状态标签
- **WHEN** 用户查看许可列表
- **THEN** 状态 Badge MUST 按 `lifecycleStatus` 显示颜色：`active`=绿色、`expired`=黄色、`suspended`=灰色、`revoked`=红色、`pending`=蓝色

#### Scenario: 详情页生命周期操作
- **WHEN** 用户查看 `active` 状态许可详情
- **THEN** 页面 MUST 显示"暂停"和"吊销"按钮，以及"续期"和"升级"按钮

#### Scenario: 续期弹窗
- **WHEN** 用户点击"续期"按钮
- **THEN** 系统 MUST 弹出日期选择 Dialog，确认后调用 Renewal API

#### Scenario: 升级弹窗
- **WHEN** 用户点击"升级"按钮
- **THEN** 系统 MUST 弹出升级 Dialog，允许选择新套餐或自定义约束值，确认后调用 Upgrade API

### Requirement: Registration Code 交互重构
系统 SHALL 在 Issue License Sheet 中将 Registration Code 从文本输入改为"选择预注册码"或"自动生成"的组合交互。

#### Scenario: 选择预注册码
- **WHEN** 用户展开 Registration Code 选择器
- **THEN** 系统 MUST 展示当前商品 + 授权主体下未绑定的预注册码列表（含 fingerprint 信息）

#### Scenario: 自动生成注册码
- **WHEN** 用户点击"自动生成"按钮
- **THEN** 系统 MUST 调用生成 API，将生成的 code 填入表单并禁用选择器

#### Scenario: 无可用预注册码
- **WHEN** 当前商品 + 授权主体下没有预注册码
- **THEN** 选择器 MUST 显示空状态提示，并引导用户点击"自动生成"

## MODIFIED Requirements

### Requirement: License list page
系统 SHALL 在「许可管理」目录下提供「许可签发」菜单页面，路径为 `/license/licenses`。

页面 MUST 包含：
- 搜索框：按 `planName` 和 `registrationCode` 搜索
- 筛选器：按商品（productId）、授权主体（licenseeId）、生命周期状态（pending/active/expired/suspended/revoked）筛选
- 数据表：显示 `planName`、商品名称、授权主体名称、生命周期状态（Badge）、生效时间、过期时间、签发时间、注册码（缩短显示）
- 状态标签：按 `lifecycleStatus` 显示颜色映射
- 签发按钮：权限 `license:license:issue`
- 行操作：查看详情、续期、升级、暂停/恢复、吊销（根据状态显示可用操作）、导出 .lic（仅 active 状态）
- 标准分页

#### Scenario: Filter by lifecycle status
- **WHEN** 用户选择状态筛选为 "生效中"
- **THEN** 列表 MUST 仅显示 `lifecycleStatus=active` 的记录

#### Scenario: 显示注册码列
- **WHEN** 列表加载完成
- **THEN** 每一行 MUST 显示该 License 的 `registrationCode` 前 8 位 + 省略号

### Requirement: Issue license form
系统 SHALL 提供许可签发表单，使用右侧 Sheet（抽屉）展示。

表单字段：
1. **商品选择** (required) — 下拉选择已发布（published）商品
2. **授权主体选择** (required) — 下拉选择活跃（active）授权主体
3. **套餐选择** (optional) — 选择商品下的套餐，或选"自定义"手动配置约束
4. **约束值配置** — 选择预设套餐后以**只读摘要卡片**展示，自定义时展开完整编辑器
5. **注册码** (required) — 下拉选择预注册码 或 点击"自动生成"
6. **生效日期** (required) — 日期选择器
7. **过期日期** (optional) — 日期选择器，留空表示永久有效
8. **备注** (optional) — 文本域

#### Scenario: Select plan shows read-only summary
- **WHEN** 用户选择一个非自定义套餐
- **THEN** 约束值配置区域 MUST 显示该套餐的只读摘要卡片，不再展示 disabled 的编辑器控件

#### Scenario: Custom constraints
- **WHEN** 用户选择"自定义"套餐
- **THEN** 表单 MUST 展开约束值编辑器，根据商品 constraintSchema 渲染模块和功能配置

#### Scenario: Successful submission
- **WHEN** 用户填写完整表单并提交
- **THEN** 系统 MUST 调用签发 API，成功后关闭 Sheet 并刷新列表

### Requirement: License detail page
系统 SHALL 提供许可详情页面，路径为 `/license/licenses/:id`。

页面 MUST 包含以下信息区块：
- **基本信息**: 生命周期状态、商品名称/代码、授权主体名称/代码、套餐名称、注册码（完整显示并支持复制）
- **有效期**: 生效时间、过期时间（永久有效显示为"永久"）
- **约束值**: 以结构化方式展示 constraintValues（模块和功能值）
- **签发信息**: 签发人、签发时间、密钥版本
- **升级链路** (仅 Upgrade 产生): 若 `originalLicenseId` 存在，显示"由许可 #X 升级而来"
- **吊销信息** (仅 revoked): 吊销人、吊销时间
- **暂停信息** (仅 suspended): 暂停人、暂停时间
- **操作按钮**: 导出 .lic（仅 active）、续期、升级、暂停/恢复、吊销（根据 lifecycleStatus 显示）

#### Scenario: View active license
- **WHEN** 查看状态为 `active` 的许可详情
- **THEN** 页面 MUST 显示"导出"、"续期"、"升级"、"暂停"和"吊销"操作按钮

#### Scenario: View expired license
- **WHEN** 查看状态为 `expired` 的许可详情
- **THEN** 页面 MUST 显示"续期"按钮，不显示"导出"、"升级"、"暂停"按钮

#### Scenario: View revoked license
- **WHEN** 查看状态为 `revoked` 的许可详情
- **THEN** 页面 MUST 显示吊销信息，不显示"导出"、"续期"、"升级"、"暂停/恢复"、"吊销"按钮

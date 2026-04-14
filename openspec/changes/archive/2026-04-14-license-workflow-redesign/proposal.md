## Why

当前 License 模块的底层模型（Product → Plan → License）已经具备了支撑复杂 B2B 授权场景的能力，但产品工作流存在明显断点：License 只有 `issued/revoked` 两种状态，缺少生命周期管理（过期、续期、升级）；Registration Code 是必填自由文本，容易产生脏数据；Key Rotation 缺少影响范围提示，操作员可能在无意识的情况下破坏所有客户的激活状态。这些问题导致系统无法支撑真实的 SaaS/软件授权运营闭环。

## What Changes

- **引入完整的 License 生命周期状态机**：在现有 `issued/revoked` 基础上，增加 `pending`（待交付）、`active`（生效中）、`expired`（已过期）、`suspended`（已暂停）状态，并支持 Renewal（续期）和 Upgrade（升级）操作。
- **重塑 Registration Code 工作流**：支持两种模式——"客户端预注册"（客户先提交机器码，操作员从列表中选择）和"系统自动生成"（基于规则生成 GUID 或硬件指纹）。废除必填自由文本字段。
- **增强 Key Rotation 的安全提示**：执行密钥轮转前，系统须展示受影响的历史 License 数量，并提供一键批量重签（re-issue）能力。
- **精简 Licensee 模型**：将 `business_info`（地址、税号、银行信息等）从核心模型中剥离，改为可选扩展字段或完全移除，使 Licensee 回归"权利归属主体"的轻量定位。
- **优化前端信息架构与交互**：
  - License 列表增加 Registration Code 列，Status Badge 使用更精准的颜色语义（active=绿色、expired=黄色、suspended=灰色、revoked=红色）。
  - Issue License Sheet 在选择预设 Plan 时，以只读摘要卡片展示授权配置，避免 disabled 状态的编辑器造成的认知困惑。
  - ConstraintEditor 折叠高级设置（min/max/default/key），降低视觉 nesting 深度。
- **修复表单状态管理反模式**：将 Sheet 内的 render 期 side effect 重构为受控的 `useEffect` 或 Wizard 模式，防止外部数据更新时意外重置表单。

## Capabilities

### New Capabilities
- `license-lifecycle`: License 生命周期管理（pending/active/expired/suspended/revoked 状态机）、自动过期检测、Renewal 续期、Upgrade 升级。
- `license-registration`: 客户端预注册（Machine ID / Hardware Fingerprint 收集）、Registration Code 自动生成策略、注册码与 Licensee 的绑定关系管理。
- `license-key-compat`: 密钥轮转影响评估（统计受影响 License 数量）、历史密钥兼容验证、批量重签 API。

### Modified Capabilities
- `license-issuance`: 增加 Renewal 和 Upgrade API；Registration Code 来源改为"预注册列表"或"自动生成"；导出 .lic 时兼容多版本公钥验证。
- `license-issuance-ui`: Issue License Sheet 增加 Registration Code 选择/生成交互；License 列表增加状态筛选（active/expired/suspended）和 Registration Code 列；详情页增加 Renewal/Upgrade 操作入口。
- `license-licensee`: **BREAKING** — 从核心数据模型中移除 `business_info`、`contact_phone`、`contact_email` 等 CRM 字段，仅保留 `name`、`code`、`notes`、`status`。
- `license-product-ui`: Key Management Tab 增加密钥轮转影响评估弹窗，展示受影响 License 数量和批量重签按钮。

## Impact

- **后端代码**：`internal/app/license/` 下的 model、service、handler、repository 需要新增生命周期状态字段、Renewal/Upgrade 业务逻辑、Registration 管理接口。
- **前端代码**：`web/src/apps/license/` 下的 pages、components 需要重构 IssueLicenseSheet、ConstraintEditor、License 列表/详情页。
- **数据库**：`license_licenses` 表可能新增 `original_license_id`（用于 upgrade 链路跟踪）、`suspended_at` 等字段；`license_licensees` 表移除或忽略 CRM 相关字段。
- **API 兼容性**：`license-issuance` 的签发接口参数中 `registrationCode` 的行为发生变化（不再接受任意字符串），属于部分行为变更。

## MODIFIED Requirements

### Requirement: Licensee data model
系统 SHALL 提供 Licensee（授权主体）实体，存储在 `license_licensees` 表中，包含以下字段：
- `id` (uint, PK) — 继承 BaseModel
- `name` (varchar 128, NOT NULL, UNIQUE) — 主体名称
- `code` (varchar 64, NOT NULL, UNIQUE) — 自动生成的唯一代码，格式 `LS-{12位随机字母数字}`
- `notes` (TEXT) — 备注
- `status` (varchar 16, NOT NULL, default "active") — 状态：`active` 或 `archived`
- `created_at`, `updated_at`, `deleted_at` — 继承 BaseModel

**说明**：`contact_name`、`contact_phone`、`contact_email`、`business_info` 字段从核心模型中移除，不再在 API 请求/响应中使用。数据库表物理列可保留以兼容历史数据，但逻辑上视为废弃。

#### Scenario: Licensee 表自动迁移
- **WHEN** 应用启动时 LicenseApp.Models() 返回 Licensee 模型
- **THEN** GORM AutoMigrate SHALL 创建或更新 `license_licensees` 表

#### Scenario: 创建时不接受 CRM 字段
- **WHEN** 用户创建 Licensee 时提交 `contactName` 或 `businessInfo`
- **THEN** 系统 MUST 忽略这些字段，仅保存 `name`、`code`、`notes`、`status`

### Requirement: Licensee CRUD API
系统 SHALL 提供以下 RESTful API 端点，均在 JWT + Casbin 中间件保护下：

| Method | Path | 说明 |
|--------|------|------|
| POST | `/api/v1/license/licensees` | 创建授权主体 |
| GET | `/api/v1/license/licensees` | 列表查询（分页+搜索+状态筛选） |
| GET | `/api/v1/license/licensees/:id` | 获取单个详情 |
| PUT | `/api/v1/license/licensees/:id` | 更新授权主体 |
| PATCH | `/api/v1/license/licensees/:id/status` | 变更状态 |

#### Scenario: 创建授权主体
- **WHEN** POST `/api/v1/license/licensees` 携带 `{name, notes?}`
- **THEN** 系统 SHALL 创建记录，自动生成 code，返回 `{code:0, data: LicenseeResponse}`，响应中不含 CRM 字段

#### Scenario: 获取单个详情
- **WHEN** GET `/api/v1/license/licensees/:id`
- **THEN** 系统 SHALL 返回精简的 LicenseeResponse，仅包含 id, name, code, notes, status, createdAt, updatedAt

#### Scenario: 更新授权主体
- **WHEN** PUT `/api/v1/license/licensees/:id` 携带可更新字段
- **THEN** 系统 SHALL 更新 name、notes 字段，code 和 status 不可通过此接口修改，CRM 字段被忽略

### Requirement: Licensee Response 类型
系统 SHALL 提供 `LicenseeResponse` 结构用于 API 响应，包含字段：`id`, `name`, `code`, `notes`, `status`, `createdAt`, `updatedAt`。

#### Scenario: ToResponse 转换
- **WHEN** Licensee 模型需要返回给前端
- **THEN** SHALL 调用 `ToResponse()` 方法转换为精简的 `LicenseeResponse`，不含 CRM 字段

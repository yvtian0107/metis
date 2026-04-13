## Why

用户需要将自己的服务（应用、容器、主机）接入可观测数据采集链路，但目前缺少统一的接入引导入口和鉴权机制。本次变更引入类 DataDog Integrations 风格的接入模块，提供 Token 管理、接入引导（监控/日志/APM 三类）和 Traefik ForwardAuth 验证端点，让用户能够以标准 OTLP/HTTP 协议安全地将数据推送到平台采集链路。

## What Changes

- 新增独立 App `internal/app/observe`，实现 App 接口（Models/Seed/Providers/Routes/Tasks）
- 新增 `IntegrationToken` 数据模型：每用户多 Token，bcrypt hash 存储，预留 `org_id`/`scope` 字段支持未来数据权限
- 新增 ForwardAuth 验证端点 `GET /api/v1/observe/auth/verify`，绕过 JWT+Casbin，供 Traefik 调用鉴权，并通过 response header 注入 `X-Metis-User-Id`、`X-Metis-Token-Id`、`X-Metis-Scope`
- 新增 Token CRUD API（创建/列表/撤销），每用户上限 10 个
- 新增 SystemConfig key `observe.otel_endpoint`，管理员配置 OTel 接入域名
- 新增前端 App `web/src/apps/observe`，包含两个页面：Integration Catalog（卡片网格）和 API Tokens 管理
- Integration Catalog 前端硬编码 11 个集成模板（APM×4 + Metrics×4 + Logs×3），每个模板提供 Docker Compose 和 Binary 两种引导文案，文案中动态填充用户所选 Token 和 endpoint
- `edition_full.go` 新增 `_ "metis/internal/app/observe"` 导入

## Capabilities

### New Capabilities

- `observe-token`: Integration Token 的生命周期管理——创建（含一次性明文展示）、列表（仅显示前缀）、撤销；每用户上限 10 个；bcrypt hash 存储；预留 `org_id`/`scope` 字段
- `observe-auth`: Traefik ForwardAuth 验证端点，使用 Bearer Token 鉴权，验证通过后在 response header 中注入数据归属信息（user_id、token_id、scope），供下游 OTel Collector 打标签写入 ClickHouse
- `observe-integration-ui`: Integration Catalog 页面——可搜索的分类卡片网格（APM/Metrics/Logs 三类，11 个集成），点击进入详情引导页，支持选择 Token、切换 Docker/Binary Tab，自动填充配置片段
- `observe-token-ui`: API Tokens 管理页面——展示 Token 列表（前缀+名称+创建/最近使用时间）、新建 Token（一次性展示完整值）、撤销 Token（二次确认）

### Modified Capabilities

（无现有 spec 需要变更）

## Impact

- **新增文件**：`internal/app/observe/`（app.go、model.go、token.go、middleware.go、repo.go、service.go、handler.go、seed.go）
- **新增文件**：`web/src/apps/observe/`（module.ts、pages/integrations/、pages/tokens/、locales/）
- **修改文件**：`cmd/server/edition_full.go`（新增 import）、`web/src/apps/_bootstrap.ts`（新增 import）
- **数据库**：新增 `integration_tokens` 表（AutoMigrate via App.Models()）
- **SystemConfig**：新增 key `observe.otel_endpoint`（seed 时写入默认空值）
- **无 breaking change**，现有功能不受影响

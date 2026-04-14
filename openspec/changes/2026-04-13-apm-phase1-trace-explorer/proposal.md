## Why

外部服务的 OTel trace 数据已通过 Collector 写入 ClickHouse（官方 `clickhouse` exporter 默认表 `otel_traces`），但平台没有任何查询和展示能力。运维人员需要登录 ClickHouse 客户端手写 SQL 才能排查问题，体验极差。

本次变更引入独立的 APM App（`internal/app/apm/`），包含 ClickHouse 数据源连接和 Trace Explorer 功能，实现 APM 模块的最小可用版本：用户配好 ClickHouse 地址后，即可在前端浏览和搜索 trace 数据、查看瀑布图和 span 详情。

APM 作为独立 App 而非 Observe App 的子功能，因为：
- APM 有自己独立的外部数据源（ClickHouse），与 Observe 的 Token/Auth 逻辑解耦
- APM 可通过 edition/build tag 独立裁剪（`APPS=system,apm`）
- 职责边界清晰：Observe 负责数据接入鉴权，APM 负责数据查询展示

## What Changes

- `internal/config/config.go` 新增 `ClickHouseConfig` struct（DSN），与 `FalkorDBConfig` 同级
- `internal/handler/install.go` 的 `ExecuteRequest` 新增 ClickHouse 字段，安装向导高级设置支持配置 ClickHouse
- 新增独立 App `internal/app/apm/`，实现 App 接口（Models/Seed/Providers/Routes/Tasks）
- 新增 `internal/app/apm/clickhouse.go`：ClickHouse 客户端封装（`clickhouse-go/v2`），nil 降级模式（同 FalkorDB 模式）
- 新增 `internal/app/apm/repository.go`：Trace 查询封装（列表 + 详情），直接查 `otel_traces` 表
- 新增 `internal/app/apm/service.go`：业务逻辑薄层
- 新增 `internal/app/apm/handler.go`：`GET /api/v1/apm/traces` + `GET /api/v1/apm/traces/:traceId`
- `cmd/server/edition_full.go` 新增 `_ "metis/internal/app/apm"` import
- 前端新增 `web/src/apps/apm/` App 模块
- 前端新增 Trace Explorer 页面（表格 + 多维过滤器）
- 前端新增 Trace Detail 页面（瀑布图 + Span 属性 Sheet）
- 前端新增时间范围选择器组件（全局复用）
- 安装向导前端 Step 2 高级设置新增 ClickHouse 区块
- `support-files/dev/docker-compose.yml` 补充 ClickHouse 服务定义
- i18n 新增 ClickHouse 安装和 APM 相关翻译

## Capabilities

### New Capabilities

- `apm-clickhouse-datasource`：ClickHouse 数据源连接管理——安装向导配置 DSN、config.yml 持久化、客户端连接池、nil 降级（未配置时 APM 功能不可用但系统正常运行）
- `apm-trace-explorer`：Trace 列表查询——支持按 service、operation、status、duration 范围、时间范围过滤，分页，按时间降序排列
- `apm-trace-detail`：Trace 详情查询——返回单个 trace 的全部 span，前端渲染为瀑布图，点击 span 展示 attributes/events/resource 详情

### Modified Capabilities

- *(none — requirements unchanged)*

## Impact

- **新增依赖**：`github.com/ClickHouse/clickhouse-go/v2`
- **新增文件**：`internal/app/apm/`（app.go、clickhouse.go、repository.go、service.go、handler.go、seed.go）
- **新增文件**：`web/src/apps/apm/`（module.ts、pages/traces/、components/）
- **修改文件**：`internal/config/config.go`、`internal/handler/install.go`、`cmd/server/edition_full.go`、`web/src/apps/registry.ts`
- **前端修改**：`web/src/pages/install/index.tsx`（新增 ClickHouse 配置区块）
- **docker-compose**：新增 ClickHouse 服务
- **数据库**：无新 GORM 表（查询 ClickHouse 而非 GORM DB）
- **无 breaking change**

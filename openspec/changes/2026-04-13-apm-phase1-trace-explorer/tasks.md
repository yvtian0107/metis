## 1. ClickHouse 配置基础设施

- [x] 1.1 `internal/config/config.go`：新增 `ClickHouseConfig` struct（`DSN string`），在 `MetisConfig` 中加 `ClickHouse *ClickHouseConfig \`yaml:"clickhouse,omitempty"\``
- [x] 1.2 `internal/handler/install.go`：`ExecuteRequest` 新增 `ClickHouseDSN string` 字段，`execute()` 中非空时写入 `cfg.ClickHouse`
- [x] 1.3 `web/src/pages/install/index.tsx`：Step 2 高级设置新增 ClickHouse 区块（toggle + DSN 输入框，placeholder `clickhouse://default:@localhost:9000/otel`）
- [x] 1.4 `web/src/pages/install/index.tsx`：CompleteStep 确认摘要中展示 ClickHouse 配置（如已启用）
- [x] 1.5 `web/src/i18n/locales/en/install.json` + `zh-CN/install.json`：新增 ClickHouse 相关翻译 key

## 2. APM App 骨架

- [x] 2.1 新建 `internal/app/apm/` 目录
- [x] 2.2 新建 `internal/app/apm/app.go`：实现 App 接口，`Name()` 返回 `"apm"`，`Models()` 返回 nil，`Tasks()` 返回 nil
- [x] 2.3 `cmd/server/edition_full.go`：新增 `_ "metis/internal/app/apm"` import

## 3. ClickHouse 客户端

- [x] 3.1 运行 `go get github.com/ClickHouse/clickhouse-go/v2` 引入依赖
- [x] 3.2 新建 `internal/app/apm/clickhouse.go`：实现 `NewClickHouseClient(i do.Injector) (*ClickHouseClient, error)`，cfg.ClickHouse 为 nil 时返回 `nil, nil`（降级模式）
- [x] 3.3 连接验证：连接成功后执行 `SELECT 1` 确认可达
- [x] 3.4 `ClickHouseClient` 实现 `do.Shutdowner` 接口，关闭连接池

## 4. APM Repository 层

- [x] 4.1 新建 `internal/app/apm/repository.go`：定义 `Repository` struct，持有 `*ClickHouseClient`
- [x] 4.2 实现 `ListTraces(ctx, filters) ([]TraceSummary, int64, error)`：查 root span 列表 + 子查询统计 span count / has_error，支持 service/operation/status/duration_min/duration_max/start/end 过滤，分页
- [x] 4.3 实现 `GetTrace(ctx, traceId) ([]Span, error)`：查单个 trace 全部 span，按 Timestamp 升序
- [x] 4.4 定义 Go struct：`TraceSummary`、`Span`、`SpanEvent`，字段映射 ClickHouse `otel_traces` 表列

## 5. APM Service 层

- [x] 5.1 新建 `internal/app/apm/service.go`：定义 `Service`，封装 repository 调用
- [x] 5.2 `Service` 在 ClickHouseClient 为 nil 时返回明确的 "not configured" 错误

## 6. APM Handler 层

- [x] 6.1 新建 `internal/app/apm/handler.go`：定义 `Handler`
- [x] 6.2 实现 `ListTraces`：`GET /api/v1/apm/traces`，解析 query params 构建过滤条件，返回分页结果
- [x] 6.3 实现 `GetTrace`：`GET /api/v1/apm/traces/:traceId`，返回全部 span
- [x] 6.4 ClickHouse 未配置时统一返回 HTTP 503 + `{"code":-1,"message":"ClickHouse not configured"}`

## 7. IOC 注册 + 路由 + Seed

- [x] 7.1 `internal/app/apm/app.go`：`Providers()` 中注册 `NewClickHouseClient`、`NewRepository`、`NewService`、`NewHandler`
- [x] 7.2 `internal/app/apm/app.go`：`Routes()` 中注册 `/apm/traces` 和 `/apm/traces/:traceId`
- [x] 7.3 新建 `internal/app/apm/seed.go`：注册 APM 菜单目录 + Traces 菜单项 + Casbin 策略

## 8. Docker Compose

- [x] 8.1 `support-files/dev/docker-compose.yml`：ClickHouse 服务已存在（`clickhouse/clickhouse-server:latest`，端口 8123+9000，volume 持久化）

## 9. 前端 App 骨架

- [x] 9.1 新建 `web/src/apps/apm/` 目录结构（module.ts、pages/、components/、hooks/）
- [x] 9.2 新建 `web/src/apps/apm/module.ts`：调用 `registerApp()` 注册 APM 路由
- [x] 9.3 `web/src/apps/_bootstrap.ts`：新增 `import './apm/module'`

## 10. 前端通用组件

- [x] 10.1 新建 `web/src/apps/apm/components/time-range-picker.tsx`：时间范围选择器（快捷选项 Last 15m/1h/6h/24h/7d + Custom 自定义范围），输出 `{ start: string, end: string }`
- [x] 10.2 新建 `web/src/apps/apm/hooks/use-time-range.ts`：时间范围状态 hook

## 11. Trace Explorer 页面

- [x] 11.1 新建 `web/src/apps/apm/pages/traces/index.tsx`：Trace 列表页面
- [x] 11.2 实现过滤器栏：Service 下拉、Operation 输入、Status 下拉（All/OK/Error）、Duration 范围输入、TimeRangePicker
- [x] 11.3 实现 Trace 列表表格：TraceId（截断+可复制）、Root Operation、Service、Duration（ms，带颜色条）、Span Count、Status 图标、Timestamp
- [x] 11.4 实现分页
- [x] 11.5 点击 trace 行跳转到 `/apm/traces/:traceId`

## 12. Trace Detail 页面（瀑布图）

- [x] 12.1 新建 `web/src/apps/apm/pages/traces/[traceId]/index.tsx`：Trace 详情页面
- [x] 12.2 新建 `web/src/apps/apm/components/waterfall-chart.tsx`：瀑布图组件
- [x] 12.3 瀑布图实现：从 flat span 列表按 parentSpanId 构建树 → 递归渲染 → 每个 span 为水平 bar（长度正比 duration，颜色按 service 区分，error 红色高亮）
- [x] 12.4 瀑布图交互：点击 span 打开 Span Detail Sheet
- [x] 12.5 新建 `web/src/apps/apm/components/span-detail-sheet.tsx`：Span 属性面板（Sheet），展示 Attributes、Events、Resource Attributes

## 13. 前端 API 集成

- [x] 13.1 新增 API 调用函数：`fetchTraces(filters)` + `fetchTrace(traceId)`
- [x] 13.2 i18n：`web/src/apps/apm/locales/en.json` + `zh-CN.json`，APM 页面翻译

## 14. 构建验证

- [x] 14.1 运行 `go build -tags dev ./cmd/server/` 确认后端编译无误
- [x] 14.2 运行 `cd web && bun run lint` 确认前端 ESLint 无报错（APM 文件无新增错误）

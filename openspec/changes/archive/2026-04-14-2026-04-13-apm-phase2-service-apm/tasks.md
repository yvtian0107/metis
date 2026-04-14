## 1. 后端 Service 聚合查询

- [x] 1.1 `internal/app/apm/repository.go`：新增 `ListServices(ctx, start, end) ([]ServiceOverview, error)`
- [x] 1.2 `internal/app/apm/repository.go`：新增 `GetServiceDetail(ctx, serviceName, start, end) (*ServiceDetail, error)`
- [x] 1.3 `internal/app/apm/repository.go`：新增 `ListOperations(ctx, serviceName, start, end) ([]OperationStats, error)`
- [x] 1.4 定义 Go struct：`ServiceOverview`、`ServiceDetail`、`OperationStats`

## 2. 后端时序聚合查询

- [x] 2.1 `internal/app/apm/repository.go`：新增 `GetTimeseries(ctx, params) ([]TimeseriesPoint, error)`
- [x] 2.2 `internal/app/apm/repository.go`：新增 `GetServiceSparklines(ctx, start, end, interval) (map[string][]SparklinePoint, error)`
- [x] 2.3 定义 Go struct：`TimeseriesPoint`、`SparklinePoint`

## 3. APM Service + Handler 扩展

- [x] 3.1 `internal/app/apm/service.go`：新增 `ListServices`、`GetServiceDetail`、`ListOperations`、`GetTimeseries`、`GetServiceSparklines` 方法
- [x] 3.2 `internal/app/apm/handler.go`：新增 `ListServices`——`GET /api/v1/apm/services`
- [x] 3.3 `internal/app/apm/handler.go`：新增 `GetServiceDetail`——`GET /api/v1/apm/services/:name`
- [x] 3.4 `internal/app/apm/handler.go`：新增 `GetTimeseries`——`GET /api/v1/apm/timeseries`

## 4. 路由 + 菜单

- [x] 4.1 `internal/app/apm/app.go`：`Routes()` 注册 `/apm/services`、`/apm/services/:name`、`/apm/timeseries`
- [x] 4.2 `internal/app/apm/seed.go`：新增 Services 菜单项，更新 Casbin 策略

## 5. 前端依赖

- [x] 5.1 `cd web && bun add recharts`

## 6. Service Catalog 页面

- [x] 6.1 新建 `web/src/apps/apm/pages/services/index.tsx`
- [x] 6.2 实现服务列表表格：ServiceName、Req/s、Avg Duration、P95、Error Rate、sparkline
- [x] 6.3 每行内嵌 sparkline，数据来自 `GetServiceSparklines` API（一次查询）
- [x] 6.4 新建 `web/src/apps/apm/components/sparkline.tsx`
- [x] 6.5 点击服务行跳转 `/apm/services/:name`

## 7. Service Detail 页面

- [x] 7.1 新建 `web/src/apps/apm/pages/services/[name]/index.tsx`
- [x] 7.2 实现指标卡片：Throughput、Avg Latency、P95 Latency、Error Rate + sparkline
- [x] 7.3 实现时序趋势图：recharts LineChart
- [x] 7.4 实现 Operations 表格
- [x] 7.5 点击 Operation 行跳转 `/apm/traces?service=xxx&operation=yyy&start=...&end=...`

## 8. Trace Explorer 增强

- [x] 8.1 `web/src/apps/apm/pages/traces/index.tsx`：支持从 URL searchParams 初始化过滤器

## 9. 前端路由 + API

- [x] 9.1 `web/src/apps/apm/module.ts`：注册 `/apm/services` 和 `/apm/services/:name`
- [x] 9.2 新增 API 函数：`fetchServices`、`fetchServiceDetail`、`fetchTimeseries`、`fetchServiceSparklines`

## 10. 构建验证

- [x] 10.1 运行 `go build -tags dev ./cmd/server/` 确认编译无误
- [x] 10.2 运行 `cd web && bun run lint` 确认 ESLint 无报错

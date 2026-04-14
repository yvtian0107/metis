## Why

Phase 1 完成后用户可以查看单条 trace，但缺乏全局视角——不知道哪些服务在运行、每个服务的健康状况如何、哪些 endpoint 最慢或错误率最高。运维人员需要像 Datadog APM 或 Sematext 那样的 Service 视图：一眼看到所有服务的吞吐量、延迟（P50/P95/P99）、错误率，以及时间趋势。

本次变更在 APM App 已有的 ClickHouse 查询基础上，新增 Service 聚合查询和时序数据 API，前端新增 Service Catalog（落地页）和 Service Detail 页面，形成完整的 APM 用户动线：Service Catalog → Service Detail → Trace Explorer。

## What Changes

- `internal/app/apm/repository.go`：新增 Service 聚合查询方法（`ListServices`、`GetServiceDetail`、`ListOperations`、`GetTimeseries`、`GetServiceSparklines`）
- `internal/app/apm/handler.go`：新增 Service 相关 API（`GET /api/v1/apm/services`、`GET /api/v1/apm/services/:name`、`GET /api/v1/apm/timeseries`）
- 前端新增 Service Catalog 页面（服务列表 + 概览指标 + sparkline）
- 前端新增 Service Detail 页面（指标卡片 + 时序趋势图 + operation 列表）
- 前端新增 sparkline 组件（迷你趋势线）
- 调整菜单：Services 作为 APM 落地页

## Capabilities

### New Capabilities

- `apm-service-catalog`：Service 目录页——从 `otel_traces` 聚合所有服务的 request_count、avg_duration、P95_duration、error_rate、last_seen，支持时间范围过滤，表格内嵌 sparkline 趋势
- `apm-service-detail`：Service 详情页——单服务的时序趋势图（throughput、latency P50/P95/P99、error rate）+ operation 列表（每个 endpoint 的指标统计），点击 operation 跳转到已过滤的 Trace Explorer
- `apm-timeseries`：时序聚合 API——按时间桶（1m/5m/1h）聚合 throughput、latency percentiles、error rate，支持按 service 和 operation 过滤

### Modified Capabilities

- `apm-trace-explorer`：Trace Explorer 表格新增「从 Service Detail 跳转时自动填充 service+operation 过滤条件」的能力

## Impact

- **修改文件**：`internal/app/apm/repository.go`、`service.go`、`handler.go`、`app.go`、`seed.go`
- **前端新增**：`web/src/apps/apm/pages/services/index.tsx`、`web/src/apps/apm/pages/services/[name]/index.tsx`、`web/src/apps/apm/components/sparkline.tsx`
- **前端修改**：`web/src/apps/apm/module.ts`（新路由）、`web/src/apps/apm/pages/traces/index.tsx`（URL 参数预填充）
- **前端新增依赖**：`recharts`
- **菜单 seed 更新**：新增 Services 菜单项
- **无 breaking change**，Phase 1 API 不变

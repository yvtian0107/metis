## Why

Phase 2 完成后用户可以看到单个服务的指标，但缺乏服务间调用关系的全局拓扑视图——不知道哪些服务依赖哪些下游、调用链上的瓶颈在哪。同时排查问题时需要在 trace 详情和日志之间来回切换，效率低下。

本次变更在 APM App 中新增 Service Map（拓扑图）和 Trace 关联日志功能：拓扑图从 span 的 parent-child 关系推导服务间调用依赖，日志关联通过 TraceId 从 `otel_logs` 表查询关联日志，嵌入 Trace Detail 页面。

## What Changes

- `internal/app/apm/repository.go`：新增拓扑查询方法（`GetTopology`，JOIN 查询 caller→callee 关系 + 调用指标）
- `internal/app/apm/repository.go`：新增日志查询方法（`GetLogsByTraceId`，查 `otel_logs` 表）
- `internal/app/apm/handler.go`：新增 API（`GET /api/v1/apm/topology`、`GET /api/v1/apm/traces/:traceId/logs`）
- 前端新增 Service Map 页面（拓扑图，使用 dagre 布局）
- 前端修改 Trace Detail 页面，新增 Logs tab
- 调整菜单：新增 Topology 入口

## Capabilities

### New Capabilities

- `apm-service-map`：Service Map（拓扑图）——从 `otel_traces` 中 client/server span 的 parent-child 关系推导服务间调用依赖，展示为有向图
- `apm-trace-logs`：Trace 关联日志——通过 TraceId 从 `otel_logs` 表查询关联日志记录

### Modified Capabilities

- `apm-trace-detail`：Trace 详情页新增 Logs tab

## Impact

- **修改文件**：`internal/app/apm/repository.go`、`service.go`、`handler.go`、`app.go`、`seed.go`
- **前端新增**：`web/src/apps/apm/pages/topology/index.tsx`、`web/src/apps/apm/components/service-map.tsx`
- **前端修改**：`web/src/apps/apm/pages/traces/[traceId]/index.tsx`（Logs tab）
- **前端新增依赖**：`@dagrejs/dagre`
- **菜单 seed 更新**：新增 Topology 菜单项
- **无 breaking change**

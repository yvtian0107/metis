## 1. 后端拓扑查询

- [x] 1.1 `internal/app/apm/repository.go`：新增 `GetTopology(ctx, start, end) (*TopologyGraph, error)`
- [x] 1.2 定义 Go struct：`TopologyGraph`、`TopologyNode`、`TopologyEdge`
- [x] 1.3 Nodes 从 Edges 的 Caller/Callee 去重生成，补充每个节点自身指标

## 2. 后端日志关联查询

- [x] 2.1 `internal/app/apm/repository.go`：新增 `GetLogsByTraceId(ctx, traceId) ([]TraceLog, bool, error)`——bool 标记 otel_logs 表是否存在
- [x] 2.2 otel_logs 表不存在时降级：返回空列表 + `logsAvailable: false`
- [x] 2.3 定义 Go struct：`TraceLog`

## 3. APM Service + Handler

- [x] 3.1 `internal/app/apm/service.go`：新增 `GetTopology`、`GetLogsByTraceId`
- [x] 3.2 `internal/app/apm/handler.go`：`GET /api/v1/apm/topology`
- [x] 3.3 `internal/app/apm/handler.go`：`GET /api/v1/apm/traces/:traceId/logs`

## 4. 路由 + 菜单

- [x] 4.1 `internal/app/apm/app.go`：注册新路由
- [x] 4.2 `internal/app/apm/seed.go`：新增 Topology 菜单项 + Casbin 策略

## 5. 前端依赖

- [x] 5.1 `cd web && bun add @dagrejs/dagre`

## 6. Service Map 页面

- [x] 6.1 新建 `web/src/apps/apm/pages/topology/index.tsx`
- [x] 6.2 新建 `web/src/apps/apm/components/service-map.tsx`：dagre 布局 + SVG 渲染
- [x] 6.3 节点：圆角矩形，ServiceName + RequestCount + ErrorRate，error > 5% 红色
- [x] 6.4 边：带箭头曲线，线宽正比 CallCount，error > 5% 红色
- [x] 6.5 交互：点击节点跳转 `/apm/services/:name`，hover tooltip
- [x] 6.6 顶部复用 TimeRangePicker

## 7. Trace Detail 日志 Tab

- [x] 7.1 修改 `web/src/apps/apm/pages/traces/[traceId]/index.tsx`：加 Tabs（Spans / Logs）
- [x] 7.2 Logs tab：表格展示 Timestamp、Severity 徽标、Service、Body
- [x] 7.3 支持 severity 下拉过滤
- [x] 7.4 `logsAvailable: false` 时显示引导提示

## 8. 前端路由 + API

- [x] 8.1 `web/src/apps/apm/module.ts`：注册 `/apm/topology`
- [x] 8.2 新增 API 函数：`fetchTopology`、`fetchTraceLogs`

## 9. 构建验证

- [x] 9.1 运行 `go build -tags dev ./cmd/server/` 确认编译无误
- [x] 9.2 运行 `cd web && bun run lint` 确认 ESLint 无报错

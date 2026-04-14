## 1. 后端分析 API

- [x] 1.1 `repository.go`：新增 `SpanSearchResult` struct + `SearchSpans(ctx, params)` 方法，支持 eq/contains/exists 三种操作符查询 SpanAttributes
- [x] 1.2 `repository.go`：新增 `AnalyticsGroup` struct + `GetAnalytics(ctx, params)` 方法，支持按 service/operation/statusCode 分组聚合
- [x] 1.3 `repository.go`：新增 `LatencyBucket` struct + `GetLatencyDistribution(ctx, params)` 方法，返回直方图 bucket 数据
- [x] 1.4 `repository.go`：新增 `ErrorGroup` struct + `GetErrors(ctx, params)` 方法，按 StatusMessage + exception.type 分组
- [x] 1.5 `service.go`：新增 `SearchSpans`、`GetAnalytics`、`GetLatencyDistribution`、`GetErrors` 方法（nil-check 模式）
- [x] 1.6 `handler.go`：新增 `SearchSpans` handler（GET /api/v1/apm/spans/search）
- [x] 1.7 `handler.go`：新增 `GetAnalytics` handler（GET /api/v1/apm/analytics）
- [x] 1.8 `handler.go`：新增 `GetLatencyDistribution` handler（GET /api/v1/apm/latency-distribution）
- [x] 1.9 `handler.go`：新增 `GetErrors` handler（GET /api/v1/apm/errors）
- [x] 1.10 `app.go`：注册 4 个新路由
- [x] 1.11 `seed.go`：为新 API 添加 admin Casbin 策略

## 2. 前端依赖 + 基础设施

- [x] 2.1 `cd web && bun add react-json-view-lite`
- [x] 2.2 新增 `api.ts`：添加 `fetchSpanSearch`、`fetchAnalytics`、`fetchLatencyDistribution`、`fetchErrors`、`fetchTopology` 的 TypeScript interface 和函数
- [x] 2.3 Go build 验证：`go build -tags dev ./cmd/server/`

## 3. 时间选择器升级（apm-time-range-pro）

- [x] 3.1 重构 `hooks/use-time-range.ts`：支持自定义日期范围（start/end ISO string）、支持从 URL searchParams 初始化
- [x] 3.2 新增自动刷新 state：interval 选项（off/10s/30s/1m/5m）+ setInterval 逻辑 + 组件卸载清理
- [x] 3.3 重构 `components/time-range-picker.tsx`：预设按钮 + Custom Popover（两个 datetime-local input + Apply 按钮）+ 自动刷新下拉

## 4. URL 状态联动（apm-url-state）

- [x] 4.1 创建 `hooks/use-url-time-range.ts`：封装 useSearchParams + useTimeRange 联动，start/end/label 双向同步到 URL
- [x] 4.2 改造 Trace Explorer（traces/index.tsx）：所有过滤参数（service/operation/status/duration_min/duration_max/page）同步到 URL
- [x] 4.3 改造 Service Catalog（services/index.tsx）：start/end 同步到 URL
- [x] 4.4 改造 Service Detail（services/[name]/index.tsx）：start/end 同步到 URL
- [x] 4.5 改造 Topology 页面（topology/index.tsx）：start/end 同步到 URL

## 5. Service Map 重构（apm-service-map）

- [x] 5.1 新建 `components/topology/service-node.tsx`：ReactFlow 自定义节点组件——服务名 + 健康指示灯 + requestCount + errorRate
- [x] 5.2 新建 `components/topology/service-edge.tsx`：ReactFlow 自定义边组件——线宽按 callCount 归一化 + errorRate > 5% 红色 + 标签
- [x] 5.3 新建 `components/topology/edge-tooltip.tsx`：边 hover tooltip——caller→callee、callCount、avgDuration、p95Duration、errorRate
- [x] 5.4 重写 `components/service-map.tsx`：用 @xyflow/react 替换手写 SVG。dagre 自动布局 → ReactFlow nodes/edges。集成 MiniMap + Controls + Background
- [x] 5.5 更新 `pages/topology/index.tsx`：使用新 ServiceMap 组件，节点点击跳转 `/apm/services/:name?start=...&end=...`
- [x] 5.6 删除旧的 dagre SVG 相关代码

## 6. 瀑布图增强（apm-waterfall-pro）

- [x] 6.1 `components/waterfall-chart.tsx`：添加时间刻度尺——顶部显示 5 等分刻度 + 垂直虚线
- [x] 6.2 添加 Span 折叠/展开功能——collapsedSet state、toggle 按钮、子 span 隐藏 + "+N spans" 提示
- [x] 6.3 添加 Service 颜色图例栏——在刻度尺下方显示所有 service + 颜色圆点
- [x] 6.4 添加 Span 搜索过滤——搜索框 + 匹配高亮/不匹配降低透明度
- [x] 6.5 添加关键路径计算和高亮——找到最长执行链路，对应 span bar 加 2px 左边框
- [x] 6.6 重构 Trace Detail 为左右分栏布局——左侧 viewPanel（瀑布图/火焰图/Span 列表 tab 切换）、右侧 detailPanel（span 详情或 trace 概要）

## 7. 火焰图（apm-trace-flamegraph）

- [x] 7.1 新建 `components/flame-chart.tsx`：Canvas 2D 火焰图组件——X 轴=时间范围、Y 轴=调用栈深度、每块=一个 span
- [x] 7.2 实现 span 坐标映射——startTime→X、depth→Y、duration→width、service→颜色
- [x] 7.3 实现关键路径高亮——最长链路 span 块加 2px 白色边框
- [x] 7.4 实现交互——hover 显示 tooltip（serviceName、spanName、duration）、点击选中 span 更新右侧详情面板
- [x] 7.5 在 Trace Detail 左侧 tab 中集成 FlameChart 组件（Waterfall / Flame Graph / Span List 三个 tab）

## 8. Service Detail 仪表盘（apm-service-dashboard）

- [x] 8.1 重构 `pages/services/[name]/index.tsx`：三行 recharts 图表——Request Rate（AreaChart）、Error Rate（AreaChart 红色）、Latency P50/P95/P99（LineChart）
- [x] 8.2 Operations 表格添加列头排序——requestCount、avgDuration、p95、errorRate 可排序（前端排序）
- [x] 8.3 新增延迟分布直方图——recharts BarChart，数据从 fetchLatencyDistribution API 获取
- [x] 8.4 新增 Errors tab——错误聚合表格（errorType、message、count、lastSeen），数据从 fetchErrors API 获取

## 9. Span 详情增强

- [x] 9.1 重构 `components/span-detail-sheet.tsx` 为 `components/span-detail-panel.tsx`——从 Sheet 弹窗改为内联面板组件（用于左右分栏）
- [x] 9.2 Attributes tab 添加 JSON tree 视图切换——默认 KV 表格，按钮切换到 react-json-view-lite 渲染
- [x] 9.3 添加属性搜索框——前端过滤 key/value 包含关键词的条目
- [x] 9.4 每个属性值旁添加复制按钮

## 10. i18n + 构建验证

- [ ] 10.1 更新 `locales/en.json` 和 `zh-CN.json`：新增 topology pro、flamegraph、waterfall pro、dashboard、analytics 相关翻译 key
- [x] 10.2 运行 `go build -tags dev ./cmd/server/` 确认编译无误
- [x] 10.3 运行 `cd web && bun run lint` 确认 ESLint 无报错

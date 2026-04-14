## Why

Metis APM 当前完成了三个阶段的基础功能（Trace Explorer、Service Catalog、Topology + Logs），但 UI 和功能水平与 Datadog、Sematext 等成熟产品差距巨大——手写 SVG 拓扑图无法交互、瀑布图缺少火焰图视图、表格无排序、时间选择器无自定义范围、Span 分析能力为零。需要一次性将 APM 模块升级到生产级水准，引入 ReactFlow 等成熟第三方组件替换自制粗糙实现，同时补齐后端分析 API。

## What Changes

### 前端体验升级
- **Service Map 重构**：用 `@xyflow/react`（项目已安装）替换手写 dagre+SVG，自定义节点嵌入 RED 指标和健康状态色，自定义边显示流量和延迟，内置 pan/zoom/minimap/controls
- **Trace Detail 大升级**：新增火焰图视图（Canvas 渲染）、关键路径高亮、Span 搜索/过滤、左右分栏布局（主视图 + Span 详情面板）
- **Waterfall 增强**：时间刻度尺、Span 折叠/展开、Service 泳道模式、颜色图例
- **Service Catalog 增强**：卡片模式（嵌入 RED 迷你图表）、列表/卡片视图切换、表格排序
- **Service Detail 增强**：多图表仪表盘（Request Rate、Error Rate、Latency 分布）、操作级 RED 指标
- **时间选择器增强**：自定义日期范围选择（DatePicker）、相对时间输入、自动刷新间隔
- **URL 联动**：所有页面的过滤状态同步到 URL searchParams，支持分享链接
- **Span 详情增强**：JSON tree 查看器替换 KV 表格、属性搜索、一键复制

### 后端 API 增强
- **Span 属性搜索 API**：支持按 SpanAttributes/ResourceAttributes 的键值查询
- **聚合分析 API**：按 service×operation 的 group by 聚合，支持自定义时间桶
- **延迟分布 API**：histogram bucket 分布数据，供热力图/分位数图使用
- **错误聚合 API**：按 error message/status code 分组统计

### 新增依赖
- `react-json-view-lite`：Span 属性嵌套查看
- `date-fns`：日期处理（如未安装）

### 保留不变
- ClickHouse 数据源、otel_traces/otel_logs 表结构
- 现有 API 端点保持兼容
- Go 后端架构（repository → service → handler）

## Capabilities

### New Capabilities
- `apm-service-map`: ReactFlow 服务拓扑图——自定义节点/边、dagre 自动布局、交互式 pan/zoom/minimap
- `apm-trace-flamegraph`: 火焰图视图——Canvas 渲染的 Span 聚合可视化，关键路径高亮
- `apm-waterfall-pro`: 增强瀑布图——时间刻度、折叠展开、泳道模式、Span 搜索过滤
- `apm-service-dashboard`: Service Detail 仪表盘——多维 RED 图表、操作级指标、延迟分布
- `apm-analytics-api`: 后端分析 API——Span 属性搜索、聚合分析、延迟分布、错误聚合
- `apm-time-range-pro`: 增强时间选择器——自定义日期范围、相对时间、自动刷新
- `apm-url-state`: URL 状态联动——全页面过滤参数同步到 URL

### Modified Capabilities
（无已有 spec 需要修改）

## Impact

- **前端**：`web/src/apps/apm/` 下几乎所有页面和组件重写或大幅修改
- **后端**：`internal/app/apm/repository.go` 新增 4-5 个查询方法，`handler.go` 新增对应端点，`seed.go` 新增 API 策略
- **依赖**：新增 `react-json-view-lite`，充分利用已安装的 `@xyflow/react`、`recharts`、`@dagrejs/dagre`
- **API**：新增 `/api/v1/apm/spans/search`、`/api/v1/apm/analytics`、`/api/v1/apm/latency-distribution`、`/api/v1/apm/errors` 端点
- **Casbin 策略**：为新增 API 端点添加 admin 角色策略

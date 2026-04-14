## Context

Metis APM 已完成三个阶段的基础实现（Trace Explorer、Service Catalog、Topology + Logs），后端从 ClickHouse `otel_traces`/`otel_logs` 表查询数据，前端用 Vite + React 19 + shadcn/ui 展示。当前 UI 粗糙（手写 SVG 拓扑、无火焰图、表格不可排序、时间选择器只有预设），距离 Datadog/Sematext 的生产水平差距明显。

项目已安装 `@xyflow/react@12.10.2`（AI 知识图谱使用了 `react-force-graph-2d`，但未用于 APM）、`recharts@3.8.1`、`@dagrejs/dagre@3.0.0`。shadcn/ui 组件齐全（Tabs、Sheet、Table、Badge、Select、Card 等）。前端使用 React Query 管理服务端状态，Zustand 管理客户端状态。

## Goals / Non-Goals

**Goals:**
- 将 Service Map 升级为 ReactFlow 交互式拓扑（pan/zoom/minimap/自定义节点边）
- 新增火焰图视图（Canvas 渲染）作为 Trace 可视化的第二种模式
- 增强瀑布图（时间刻度尺、折叠展开、Span 搜索、关键路径高亮）
- 增强 Service Detail 为多图表仪表盘（Request Rate、Error Rate、Latency 三图 + 操作级 RED）
- 增强时间选择器（自定义日期范围 + 自动刷新）
- 所有页面过滤状态同步到 URL searchParams
- 后端新增分析 API（Span 属性搜索、聚合分析、延迟分布、错误聚合）
- Span 详情用 JSON tree 替换平铺 KV 表格

**Non-Goals:**
- 告警系统（需要独立的告警引擎，不在本次范围）
- 自定义仪表盘/保存视图（后续迭代）
- RUM / Synthetic Monitoring（需要客户端 SDK，不在范围）
- 实时 WebSocket 推送（保持轮询 + 手动刷新模式）
- Profiling（CPU/Memory，需要独立数据管道）

## Decisions

### D1: Service Map 用 @xyflow/react 替换手写 dagre+SVG

**选择**: @xyflow/react（ReactFlow v12）
**替代方案**: 继续手写 SVG、react-force-graph-2d、vis-network
**理由**:
- 项目已安装且 AI 模块有使用 xyflow 的 canvas/node/edge 组件经验
- 内置 pan/zoom/minimap/controls，开箱即用
- 自定义节点/边通过 React 组件实现，可嵌入 recharts 迷你图
- dagre 布局通过 `@dagrejs/dagre`（已安装）做自动布局后传入 ReactFlow
- 力导向图不适合 APM 拓扑——服务间关系有方向性，dagre 分层布局更清晰

**节点设计**: 自定义 `ServiceNode` 组件，包含：
- 服务名 + 健康状态指示灯（绿/黄/红）
- Request Rate + Error Rate 数字
- 迷你 sparkline（最近趋势）
- 点击展开右侧详情面板

**边设计**: 自定义 `ServiceEdge` 组件：
- 线宽正比调用量
- 颜色：正常灰色，error > 5% 红色
- 标签显示 callCount + avgLatency
- animated 属性模拟流量方向

### D2: 火焰图用 Canvas 自绘

**选择**: 基于 Canvas 2D API 自实现
**替代方案**: speedscope、@nicola/flamechart、d3-flame-graph
**理由**:
- 第三方火焰图库主要面向 CPU profiler（stack frame），OTel span 的火焰图结构更简单——本质是 span tree 的横向时间对齐
- Canvas 渲染性能好，支持数千 span 不卡顿
- 自定义程度高：可以加关键路径高亮、service 着色、hover tooltip
- 数据结构已有（span tree + duration），只需做坐标映射

**实现**: `FlameChart` 组件
- X 轴 = 时间范围（与瀑布图共享）
- Y 轴 = 调用栈深度
- 每个矩形块 = 一个 span，宽度 = duration 占比
- 颜色 = service 着色（与瀑布图一致）
- 关键路径 = 从 root span 到最深叶子 span 的最长链路，高亮显示
- 点击块 → 打开 Span 详情面板

### D3: 瀑布图增强为 Pro 版

**在现有 waterfall-chart.tsx 基础上增量改造**:
- 顶部时间刻度尺（0ms ... totalDuration），5 等分刻度线
- Span 折叠/展开：service 级别或 parent 级别的子树折叠
- Service 颜色图例栏（显示所有 service → color 映射）
- Span 搜索：输入框过滤 spanName/serviceName，命中的高亮
- 关键路径高亮：最长执行链路用粗边框标记
- 左右分栏布局：左侧瀑布/火焰图，右侧固定 Span 详情（替换 Sheet 弹窗）

### D4: Service Detail 多图表仪表盘

**在现有页面基础上扩展**:
- 三行图表：Request Rate（area chart）、Error Rate（area chart, 红色）、Latency P50/P95/P99（line chart，已有）
- 每个图表使用 recharts，共享时间轴和 brush 联动
- 操作表增加排序（按 requestCount、errorRate、p95 排序）
- 添加延迟分布图（直方图，数据来自新 API）

### D5: 时间选择器升级

**增强现有 TimeRangePicker**:
- 保留预设按钮（15m/1h/6h/24h/7d）
- 新增「自定义」按钮，点击弹出 Popover：
  - 两个日期输入框（start/end），用 shadcn Popover + 简单 input type="datetime-local"
  - 避免引入重依赖，不用 react-day-picker / date-fns
- 新增自动刷新下拉：Off / 10s / 30s / 1m / 5m
- 刷新状态在 URL searchParams 中不持久化（仅 start/end 持久化）

### D6: URL 状态联动

**所有 APM 页面的过滤参数同步到 URL searchParams**:
- Trace Explorer: start, end, service, operation, status, duration_min, duration_max, page
- Service Catalog: start, end
- Service Detail: start, end
- Topology: start, end
- 使用 React Router 的 `useSearchParams` 实现
- 页面加载时从 URL 初始化状态，状态变化时更新 URL
- 支持分享链接、浏览器前进后退

### D7: Span 详情 JSON Tree

**选择**: react-json-view-lite
**替代方案**: react-json-view（重依赖，不再维护）、手写递归组件
**理由**: 轻量（< 5KB gzipped），支持折叠/展开、复制，样式可通过 CSS 自定义。
- SpanAttributes 和 ResourceAttributes 用 JSON tree 展示
- 保留 KV 表格作为默认视图，JSON tree 作为切换选项
- 增加属性搜索框（前端过滤）
- 每个值旁加复制按钮

### D8: 后端分析 API

**新增 4 个 ClickHouse 查询端点**:

1. **Span 属性搜索** `GET /api/v1/apm/spans/search`
   - 参数: start, end, key, value, op(eq/contains/exists)
   - ClickHouse: `WHERE SpanAttributes[key] = value` 或 `LIKE %value%`
   - 返回匹配的 span 列表（复用 Span struct）

2. **聚合分析** `GET /api/v1/apm/analytics`
   - 参数: start, end, groupBy(service/operation/statusCode), service?, operation?
   - ClickHouse: 按 groupBy 字段分组聚合 count/avg/p95/errorRate
   - 返回 `[]AnalyticsGroup{key, requestCount, avgDurationMs, p95Ms, errorRate}`

3. **延迟分布** `GET /api/v1/apm/latency-distribution`
   - 参数: start, end, service?, operation?, buckets(默认 20)
   - ClickHouse: `histogram(buckets)(Duration/1e6)` 或手动 CASE WHEN 分桶
   - 返回 `[]LatencyBucket{rangeStart, rangeEnd, count}`

4. **错误聚合** `GET /api/v1/apm/errors`
   - 参数: start, end, service?
   - ClickHouse: GROUP BY StatusMessage 或 SpanAttributes['exception.type']
   - 返回 `[]ErrorGroup{errorType, message, count, lastSeen, services[]}`

## Risks / Trade-offs

**[Canvas 火焰图的可访问性]** → 提供瀑布图作为无障碍替代，火焰图为增强视图
**[ReactFlow bundle 体积]** → @xyflow/react 已安装且 tree-shaking 良好，Topology 页面 lazy load 不影响其他页面
**[ClickHouse Span 属性搜索性能]** → Map 类型列的 key 查询在大数据量时可能慢 → 后期可考虑物化列或二级索引，当前通过时间范围限制查询量
**[URL 参数过多]** → 只持久化关键过滤参数，页码/刷新间隔等不入 URL
**[datetime-local 兼容性]** → 所有现代浏览器支持良好，避免引入 react-day-picker 减少依赖

# apm-service-map Specification

## Purpose
TBD - created by archiving change 2026-04-13-apm-phase3-topology-logs. Update Purpose after archive.
## Requirements
### Requirement: Service Map 拓扑图
系统 SHALL 提供 Service Map API 和前端拓扑图，展示服务间调用关系。

#### Scenario: 拓扑查询
- **WHEN** 用户请求 `GET /api/v1/apm/topology` 并传入时间范围
- **THEN** 返回服务间调用关系图（nodes + edges），从 otel_traces 表的 parent-child span 关系推导跨服务调用，每条 edge 包含 Caller、Callee、CallCount、AvgDuration、P95Duration、ErrorRate

#### Scenario: 拓扑图展示
- **WHEN** 用户访问 `/apm/topology`
- **THEN** 展示有向拓扑图，节点为服务（显示名称+指标），边为调用关系（线宽正比调用量），error_rate > 5% 的节点/边红色高亮

#### Scenario: 拓扑图交互
- **WHEN** 用户点击拓扑图中的服务节点
- **THEN** 跳转到 `/apm/services/:name`

### Requirement: Trace 关联日志
系统 SHALL 提供 Trace 关联日志查询，在 Trace 详情页展示关联日志。

#### Scenario: 日志查询
- **WHEN** 用户请求 `GET /api/v1/apm/traces/:traceId/logs`
- **THEN** 返回该 trace 关联的所有日志记录（从 `otel_logs` 表按 TraceId 查询），按 Timestamp 升序

#### Scenario: otel_logs 表不存在
- **WHEN** ClickHouse 中 `otel_logs` 表不存在（用户未配置 logs pipeline）
- **THEN** 返回空列表 + `logsAvailable: false` 标记，前端展示引导提示

#### Scenario: 日志过滤
- **WHEN** 用户在 Trace Detail 的 Logs tab 中选择 severity 过滤
- **THEN** 仅展示对应级别（ERROR/WARN/INFO/DEBUG）的日志

#### Scenario: Trace Detail 日志 Tab
- **WHEN** 用户在 Trace Detail 页面切换到 "Logs" tab
- **THEN** 展示该 trace 关联的日志列表（Timestamp、Severity 徽标、Service、Body），点击行可展开完整内容

### Requirement: ReactFlow 交互式服务拓扑
系统 SHALL 使用 @xyflow/react 渲染服务拓扑图，替换现有手写 dagre+SVG 实现。拓扑图 SHALL 支持 pan（拖拽画布）、zoom（滚轮缩放）、minimap（右下角缩略导航）和 controls（缩放控制按钮）。

#### Scenario: 拓扑图基础交互
- **WHEN** 用户打开 /apm/topology 页面且有拓扑数据
- **THEN** 系统显示 ReactFlow 画布，节点按 dagre 分层布局排列，用户可以拖拽画布平移、滚轮缩放、通过 minimap 快速导航

### Requirement: 自定义服务节点
每个服务节点 SHALL 渲染为自定义 React 组件，包含：服务名、健康状态指示（绿/黄/红圆点，基于 errorRate 阈值 0%/5%/10%）、Request Count 数字、Error Rate 百分比。

#### Scenario: 正常服务节点
- **WHEN** 服务 errorRate < 5%
- **THEN** 节点显示绿色健康指示灯，背景为默认卡片色

#### Scenario: 警告服务节点
- **WHEN** 服务 errorRate >= 5% 且 < 10%
- **THEN** 节点显示黄色健康指示灯，边框为黄色

#### Scenario: 错误服务节点
- **WHEN** 服务 errorRate >= 10%
- **THEN** 节点显示红色健康指示灯，边框为红色

### Requirement: 自定义调用边
边 SHALL 显示调用量和平均延迟标签。线宽 SHALL 按 callCount 归一化（最小 1.5px，最大 5px）。error > 5% 的边 SHALL 渲染为红色。边 SHALL 使用 animated 属性表示流量方向。

#### Scenario: 高错误率边
- **WHEN** 两个服务间调用的 errorRate > 5%
- **THEN** 边渲染为红色，标签显示 callCount、avgLatency 和 errorRate

### Requirement: 节点点击钻取
点击拓扑节点 SHALL 导航到该服务的详情页 `/apm/services/:name`。

#### Scenario: 点击节点跳转
- **WHEN** 用户点击拓扑图中的服务节点
- **THEN** 页面跳转到 `/apm/services/{serviceName}?start=...&end=...`，携带当前时间范围

### Requirement: 边 hover 详情
hover 拓扑边 SHALL 显示 tooltip，包含：caller→callee、callCount、avgDuration、p95Duration、errorRate。

#### Scenario: hover 边显示详情
- **WHEN** 用户 hover 一条拓扑边
- **THEN** 显示 tooltip 展示该调用关系的完整指标


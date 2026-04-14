## ADDED Requirements

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

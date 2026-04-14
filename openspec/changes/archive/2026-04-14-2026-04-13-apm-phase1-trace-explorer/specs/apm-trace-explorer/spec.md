## ADDED Requirements

### Requirement: Trace 列表查询
系统 SHALL 提供 Trace 列表查询 API，从 ClickHouse `otel_traces` 表查询 root span 并返回 trace 摘要。

#### Scenario: 基础查询
- **WHEN** 用户请求 `GET /api/v1/apm/traces` 并传入 `start` + `end` 时间范围
- **THEN** 返回该时间范围内的 trace 列表（分页），每条包含 TraceId、RootOperation、ServiceName、DurationMs、StatusCode、SpanCount、HasError、Timestamp

#### Scenario: 多维过滤
- **WHEN** 用户附加 `service`、`operation`、`status`（ok/error）、`duration_min`、`duration_max` 过滤参数
- **THEN** 返回结果仅包含满足所有条件的 trace

#### Scenario: 分页
- **WHEN** 用户传入 `page` + `page_size`
- **THEN** 返回对应页码的数据及 `total` 总数

### Requirement: Trace 详情查询
系统 SHALL 提供 Trace 详情查询 API，返回单个 trace 的全部 span。

#### Scenario: 查询详情
- **WHEN** 用户请求 `GET /api/v1/apm/traces/:traceId`
- **THEN** 返回该 trace 的所有 span（按 Timestamp 升序），每个 span 包含 SpanId、ParentSpanId、ServiceName、SpanName、SpanKind、StartTime、Duration、StatusCode、StatusMessage、Attributes、ResourceAttributes、Events

#### Scenario: Trace 不存在
- **WHEN** 指定的 traceId 在 ClickHouse 中无数据
- **THEN** 返回空 span 列表（不报错）

### Requirement: Trace Explorer 前端
系统 SHALL 提供 Trace Explorer 页面，支持列表浏览、过滤和跳转详情。

#### Scenario: 列表展示
- **WHEN** 用户访问 `/apm/traces`
- **THEN** 展示 trace 列表表格，包含 TraceId（截断）、Root Operation、Service、Duration（带颜色指示）、Span Count、Status 图标、时间

#### Scenario: 瀑布图
- **WHEN** 用户点击 trace 行进入详情页
- **THEN** 展示该 trace 的瀑布图（span 按层级缩进，bar 长度正比 duration，颜色按 service 区分，error 红色高亮）

#### Scenario: Span 详情
- **WHEN** 用户在瀑布图中点击某个 span
- **THEN** 打开 Sheet 面板，展示该 span 的 Attributes、Events、Resource Attributes

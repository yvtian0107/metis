## ADDED Requirements

### Requirement: Service 目录查询
系统 SHALL 提供 Service 目录查询 API，从 ClickHouse 聚合所有服务的概览指标。

#### Scenario: 服务列表
- **WHEN** 用户请求 `GET /api/v1/apm/services` 并传入时间范围
- **THEN** 返回所有服务的聚合指标：ServiceName、RequestCount、AvgDuration、P50/P95/P99、ErrorRate、FirstSeen、LastSeen，仅统计 SPAN_KIND_SERVER 的 span

### Requirement: Service 详情查询
系统 SHALL 提供单服务的详情查询 API，包含时序趋势和 operation 级别指标。

#### Scenario: 服务详情
- **WHEN** 用户请求 `GET /api/v1/apm/services/:name` 并传入时间范围
- **THEN** 返回该服务的概览指标 + 时序趋势数据 + operation 列表指标

#### Scenario: Operation 列表
- **WHEN** 查询服务详情
- **THEN** 返回该服务下每个 SpanName（operation）的 RequestCount、AvgDuration、P50/P95/P99、ErrorRate

### Requirement: 时序聚合查询
系统 SHALL 提供通用时序聚合 API，支持按可配置时间桶聚合趋势数据。

#### Scenario: 时序查询
- **WHEN** 用户请求 `GET /api/v1/apm/timeseries` 并传入 service、interval、时间范围
- **THEN** 返回按时间桶聚合的数据点：Timestamp、Requests、AvgMs、P50Ms、P95Ms、P99Ms、ErrorRate

### Requirement: Service Catalog 前端
系统 SHALL 提供 Service Catalog 页面作为 APM 落地页。

#### Scenario: 服务表格
- **WHEN** 用户访问 `/apm/services`
- **THEN** 展示服务列表表格（ServiceName、Req/s、Avg Duration、P95、Error Rate、sparkline 趋势）

#### Scenario: 跳转 Service Detail
- **WHEN** 用户点击服务行
- **THEN** 跳转到 `/apm/services/:name` 展示该服务详情

#### Scenario: 跳转 Trace Explorer
- **WHEN** 用户在 Service Detail 中点击某个 operation 行
- **THEN** 跳转到 `/apm/traces?service=xxx&operation=yyy` 并自动填充过滤条件

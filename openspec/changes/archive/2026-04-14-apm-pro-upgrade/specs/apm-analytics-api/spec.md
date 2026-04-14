## ADDED Requirements

### Requirement: Span 属性搜索 API
系统 SHALL 提供 `GET /api/v1/apm/spans/search` 端点，支持按 SpanAttributes 或 ResourceAttributes 的键值查询。参数包含 start、end（必填）、key、value、op（eq/contains/exists，默认 eq）。返回匹配的 span 列表。

#### Scenario: 精确匹配查询
- **WHEN** 请求 `/api/v1/apm/spans/search?start=...&end=...&key=http.method&value=POST&op=eq`
- **THEN** 返回指定时间范围内 SpanAttributes['http.method'] = 'POST' 的所有 span

#### Scenario: 模糊匹配查询
- **WHEN** 请求 `/api/v1/apm/spans/search?start=...&end=...&key=db.statement&value=SELECT&op=contains`
- **THEN** 返回指定时间范围内 SpanAttributes['db.statement'] LIKE '%SELECT%' 的所有 span

#### Scenario: 键存在查询
- **WHEN** 请求 `/api/v1/apm/spans/search?start=...&end=...&key=http.status_code&op=exists`
- **THEN** 返回指定时间范围内存在 SpanAttributes['http.status_code'] 键的所有 span

### Requirement: 聚合分析 API
系统 SHALL 提供 `GET /api/v1/apm/analytics` 端点，支持按 service、operation 或 statusCode 分组聚合。返回每组的 requestCount、avgDurationMs、p95Ms、errorRate。

#### Scenario: 按 operation 聚合
- **WHEN** 请求 `/api/v1/apm/analytics?start=...&end=...&groupBy=operation&service=api-gateway`
- **THEN** 返回 api-gateway 服务下每个 operation 的聚合指标数组

### Requirement: 延迟分布 API
系统 SHALL 提供 `GET /api/v1/apm/latency-distribution` 端点，返回指定时间范围和服务的延迟直方图数据。参数包含 start、end（必填）、service、operation（可选）、buckets（默认 20）。

#### Scenario: 获取延迟分布
- **WHEN** 请求 `/api/v1/apm/latency-distribution?start=...&end=...&service=user-service&buckets=20`
- **THEN** 返回 20 个延迟区间的 `{rangeStartMs, rangeEndMs, count}` 数组

### Requirement: 错误聚合 API
系统 SHALL 提供 `GET /api/v1/apm/errors` 端点，按错误类型和消息分组聚合。返回每组的 count、lastSeen、涉及的 services 列表。

#### Scenario: 获取错误聚合
- **WHEN** 请求 `/api/v1/apm/errors?start=...&end=...&service=api-gateway`
- **THEN** 返回按 (StatusMessage, SpanAttributes['exception.type']) 分组的错误聚合列表，按 count 降序

### Requirement: 新 API Casbin 策略
所有新增 API 端点 SHALL 在 seed.go 中注册 admin 角色的 Casbin 策略。

#### Scenario: 策略注册
- **WHEN** 系统启动执行 seed
- **THEN** admin 角色拥有 GET 权限访问 /api/v1/apm/spans/search、/api/v1/apm/analytics、/api/v1/apm/latency-distribution、/api/v1/apm/errors

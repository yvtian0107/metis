### Requirement: OTel 总开关
系统 SHALL 通过 DB `SystemConfig` 表中的 `otel.enabled` 键控制 OpenTelemetry 功能。该值默认为 `false`。OTel 初始化 SHALL 在数据库连接建立之后执行。未安装状态下，OTel SHALL 始终禁用。

#### Scenario: OTel 禁用（默认）
- **WHEN** `otel.enabled` 在 SystemConfig 中不存在或值为 `false`
- **THEN** 系统 SHALL 使用 OTel 默认的 noop TracerProvider，不创建 exporter，不发送任何 trace 数据，应用正常运行

#### Scenario: OTel 启用
- **WHEN** `otel.enabled` 在 SystemConfig 中为 `true`
- **THEN** 系统 SHALL 初始化 TracerProvider、OTLP HTTP exporter 和 W3C TraceContext propagator

#### Scenario: 安装模式下 OTel 禁用
- **WHEN** 系统处于安装模式（未安装）
- **THEN** OTel SHALL 始终使用 noop provider，不尝试从 DB 读取配置

### Requirement: OTLP HTTP Trace 导出
当 OTel 启用时，系统 SHALL 通过 OTLP HTTP 协议将 trace 数据导出到指定端点。端点地址从 SystemConfig 的 `otel.exporter_endpoint` 键读取。

#### Scenario: 配置导出端点
- **WHEN** `otel.enabled` 为 `true` 且 `otel.exporter_endpoint` 在 SystemConfig 中已设置
- **THEN** 系统 SHALL 使用该端点作为 OTLP HTTP trace exporter 的目标地址

#### Scenario: 默认导出端点
- **WHEN** `otel.enabled` 为 `true` 且 `otel.exporter_endpoint` 未设置
- **THEN** 系统 SHALL 使用 `http://localhost:4318` 作为默认端点

#### Scenario: 导出端点不可用
- **WHEN** OTLP HTTP 端点不可达
- **THEN** BatchSpanProcessor SHALL 异步重试，不阻塞 HTTP 请求处理

### Requirement: 服务资源标识
系统 SHALL 在 trace 数据中包含 service.name 资源属性，从 SystemConfig 的 `otel.service_name` 键读取。

#### Scenario: 自定义服务名
- **WHEN** `otel.service_name` 在 SystemConfig 中设置为 `my-metis`
- **THEN** 所有导出的 span SHALL 包含 `service.name=my-metis` 资源属性

#### Scenario: 默认服务名
- **WHEN** `otel.service_name` 未设置
- **THEN** 系统 SHALL 使用 `metis` 作为默认 service.name

### Requirement: 采样率配置
系统 SHALL 支持通过 SystemConfig 的 `otel.sample_rate` 键配置 trace 采样率。

#### Scenario: 全量采样（默认）
- **WHEN** `otel.sample_rate` 未设置
- **THEN** 系统 SHALL 使用 1.0（100%）采样率

#### Scenario: 部分采样
- **WHEN** `otel.sample_rate` 设置为 `0.1`
- **THEN** 系统 SHALL 使用 ParentBased(TraceIDRatioBased(0.1)) sampler，约 10% 的根 trace 被采样

### Requirement: HTTP 请求自动追踪
系统 SHALL 通过 otelgin middleware 自动为所有 HTTP 请求创建 span。

#### Scenario: 正常请求追踪
- **WHEN** 一个 HTTP 请求到达 Gin router
- **THEN** otelgin middleware SHALL 自动创建一个 span，包含 http.method、http.route、http.status_code 等属性

### Requirement: DB 查询自动追踪
系统 SHALL 通过 otelgorm plugin 自动为所有 GORM 数据库操作创建 span，且 SQL 参数 SHALL 被脱敏。

#### Scenario: 查询追踪
- **WHEN** 业务代码通过 GORM 执行数据库查询
- **THEN** otelgorm plugin SHALL 创建一个 child span，包含 db.system 和 db.statement（参数替换为 `?`）

#### Scenario: 参数脱敏
- **WHEN** SQL 语句包含用户密码等敏感参数
- **THEN** trace 中的 db.statement SHALL 将所有参数值替换为 `?`，不泄露实际值

### Requirement: slog 日志关联 trace
当 OTel 启用时，系统 SHALL 通过 otelslog bridge 在日志输出中自动注入 trace_id 和 span_id 字段。

#### Scenario: 请求日志包含 trace 信息
- **WHEN** HTTP 请求被处理且 OTel 已启用
- **THEN** logger middleware 的日志输出 SHALL 包含当前请求的 trace_id 和 span_id 字段

#### Scenario: OTel 禁用时日志不变
- **WHEN** `otel.enabled` 未启用
- **THEN** slog 日志输出 SHALL 与当前行为完全一致，无额外字段

### Requirement: Graceful shutdown 刷新 spans
系统 SHALL 在关闭时正确刷新所有 pending span 数据。

#### Scenario: 收到终止信号
- **WHEN** 系统收到 SIGTERM/SIGINT
- **THEN** TracerProvider.Shutdown() SHALL 被调用，将 BatchSpanProcessor 中的 pending span 刷新到 exporter

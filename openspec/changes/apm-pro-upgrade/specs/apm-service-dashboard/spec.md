## ADDED Requirements

### Requirement: 多维 RED 图表
Service Detail 页面 SHALL 显示三个时序图表：Request Rate（area chart）、Error Rate（area chart，红色系）、Latency P50/P95/P99（line chart）。三个图表 SHALL 共享同一时间轴。

#### Scenario: 三图表展示
- **WHEN** 用户打开 Service Detail 页面
- **THEN** 页面显示三个并排的时序图表，数据从 `/api/v1/apm/timeseries?service=xxx` 获取

### Requirement: 操作级表格排序
Operations 表格 SHALL 支持按 requestCount、avgDurationMs、p95Ms、errorRate 列排序（升序/降序切换）。默认按 requestCount 降序。

#### Scenario: 点击列头排序
- **WHEN** 用户点击 Operations 表格的 "P95" 列头
- **THEN** 表格按 P95 延迟降序排列，再次点击切换为升序

### Requirement: 延迟分布图
Service Detail 页面 SHALL 显示延迟分布直方图，数据从 `/api/v1/apm/latency-distribution` API 获取。X 轴为延迟区间，Y 轴为请求数量。

#### Scenario: 延迟分布展示
- **WHEN** 用户查看 Service Detail 页面
- **THEN** 延迟分布图显示 20 个 bucket 的直方图，可直观看到延迟分布形态

### Requirement: 错误列表
Service Detail 页面 SHALL 显示一个「Errors」tab，列出该服务的错误聚合数据（来自 `/api/v1/apm/errors?service=xxx`），包含错误类型、消息、次数、最后出现时间。

#### Scenario: 错误聚合展示
- **WHEN** 用户切换到 Errors tab
- **THEN** 显示错误聚合列表，按出现次数降序，每行包含 errorType、message（截断）、count、lastSeen

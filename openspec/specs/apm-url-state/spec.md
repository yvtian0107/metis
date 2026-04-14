# apm-url-state Specification

## Purpose
TBD - created by archiving change apm-pro-upgrade. Update Purpose after archive.
## Requirements
### Requirement: URL 参数同步
所有 APM 页面的过滤状态 SHALL 同步到 URL searchParams。页面加载时 SHALL 从 URL 初始化状态。状态变化时 SHALL 更新 URL（使用 `replace` 避免过多历史记录）。

#### Scenario: Trace Explorer URL 联动
- **WHEN** 用户在 Trace Explorer 设置 service=api-gateway、status=error
- **THEN** URL 更新为 `/apm/traces?start=...&end=...&service=api-gateway&status=error`

#### Scenario: 从 URL 恢复状态
- **WHEN** 用户打开包含过滤参数的 URL `/apm/traces?service=user-service&status=error&start=...&end=...`
- **THEN** 页面加载时过滤器自动设置为 service=user-service, status=error，并使用 URL 中的时间范围

### Requirement: 各页面同步参数
URL 同步的参数范围：
- **Trace Explorer**: start, end, service, operation, status, duration_min, duration_max, page
- **Service Catalog**: start, end
- **Service Detail**: start, end
- **Topology**: start, end

#### Scenario: Service Catalog 时间范围同步
- **WHEN** 用户在 Service Catalog 选择 Last 7d 时间范围
- **THEN** URL 更新包含 start 和 end 参数

### Requirement: 分享链接
用户 SHALL 能够复制当前页面 URL 分享给他人，接收方打开链接后看到相同的过滤状态和数据。

#### Scenario: 分享 Trace Explorer 链接
- **WHEN** 用户 A 在 Trace Explorer 设置了 service + status 过滤，复制 URL 发给用户 B
- **THEN** 用户 B 打开链接后看到相同的 service 和 status 过滤已设置

### Requirement: Span 详情 JSON Tree 视图
Span 详情面板中 SpanAttributes 和 ResourceAttributes SHALL 支持 JSON tree 视图（使用 react-json-view-lite）。默认显示 KV 表格，可切换到 JSON tree。JSON tree 支持折叠/展开嵌套层级。

#### Scenario: 切换到 JSON tree
- **WHEN** 用户在 Span 详情的 Attributes tab 点击 JSON 切换按钮
- **THEN** 属性显示从 KV 表格切换为可交互的 JSON tree

### Requirement: 属性搜索
Span 详情面板 SHALL 提供属性搜索框，用户输入关键词后前端过滤 key 或 value 包含该关键词的属性条目。

#### Scenario: 搜索属性
- **WHEN** 用户输入 "http" 到属性搜索框
- **THEN** 只显示 key 或 value 中包含 "http" 的属性行


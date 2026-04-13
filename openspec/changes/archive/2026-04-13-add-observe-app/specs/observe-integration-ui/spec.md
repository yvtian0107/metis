## ADDED Requirements

### Requirement: Integration Catalog 页面
系统 SHALL 提供 Integration Catalog 页面（路径 `/observe/integrations`），以卡片网格形式展示所有可用集成，按 APM、Metrics、Logs 三类分组，每个分组有清晰的分组标题。

#### Scenario: 页面加载
- **WHEN** 用户导航至 `/observe/integrations`
- **THEN** 系统 SHALL 展示 11 个集成卡片，按 APM（4个）、Metrics（4个）、Logs（3个）三组排列

#### Scenario: 搜索过滤
- **WHEN** 用户在搜索框中输入关键词（如 "docker"）
- **THEN** 系统 SHALL 实时过滤卡片，仅显示名称或标签匹配的集成，不匹配的分组标题亦隐藏

#### Scenario: 分类 Tab 过滤
- **WHEN** 用户点击 "APM"、"Metrics" 或 "Logs" Tab
- **THEN** 系统 SHALL 仅展示对应分类的集成卡片

### Requirement: Integration 卡片
每个集成卡片 SHALL 展示：集成图标（品牌 SVG 或 Lucide 图标）、集成名称、数据类型标签（Traces/Metrics/Logs）。卡片样式 SHALL 采用 DataDog 风格：白色背景、细边框、hover 时边框加深并有轻微阴影提升。

#### Scenario: 卡片点击进入详情
- **WHEN** 用户点击某个集成卡片
- **THEN** 系统 SHALL 导航至该集成的详情引导页 `/observe/integrations/:slug`

### Requirement: Integration 详情引导页
集成详情页 SHALL 以步骤引导形式展示接入配置，分为三个步骤区块：选择 Token、安装配置、验证连接。

#### Scenario: Token 选择区块
- **WHEN** 用户进入详情页
- **THEN** 系统 SHALL 显示 Token 下拉选择器（列出用户所有未撤销 Token）和 OTel Endpoint 地址（从 SystemConfig `observe.otel_endpoint` 读取）；若用户无 Token，SHALL 显示 "新建 Token" 快捷入口

#### Scenario: 配置片段自动填充
- **WHEN** 用户选择某个 Token
- **THEN** 配置代码片段 SHALL 自动将 `{{TOKEN}}` 和 `{{ENDPOINT}}` 替换为实际值，无需手动编辑

#### Scenario: Docker/Binary Tab 切换
- **WHEN** 用户切换 "Docker Compose" 和 "Binary / systemd" Tab
- **THEN** 系统 SHALL 展示对应安装方式的配置片段，Tab 状态保持至页面离开

#### Scenario: 一键复制配置片段
- **WHEN** 用户点击配置代码块右上角的复制图标
- **THEN** 系统 SHALL 将完整配置文本（含填充后的 Token 和 Endpoint）复制到剪贴板，并短暂显示 "已复制" 反馈

### Requirement: 第一批集成模板
系统 SHALL 包含以下 11 个硬编码集成模板，维护于前端 `data/integrations.ts`：

APM 类：Go（otel-go SDK）、Node.js（otel-node SDK）、Python（otel-python SDK）、Java（otel-java Agent）

Metrics 类：Host（otelcol hostmetrics receiver）、Docker（otelcol docker_stats receiver）、MySQL（otelcol mysql receiver）、PostgreSQL（otelcol postgresql receiver）

Logs 类：File Logs（otelcol filelog receiver）、Docker Logs（otelcol docker log driver）、Nginx（otelcol nginx + filelog receiver）

#### Scenario: 每个模板包含完整引导
- **WHEN** 用户访问任意一个集成的详情页
- **THEN** 该集成 SHALL 至少提供 Docker Compose 配置片段，APM 类集成 SHALL 额外提供对应语言的 SDK 初始化代码片段

### Requirement: OTel Endpoint 展示
详情页 SHALL 从后端 API 读取 SystemConfig `observe.otel_endpoint` 的值并展示，若未配置则显示占位提示"请联系管理员配置 OTel 接入地址"。

#### Scenario: Endpoint 已配置
- **WHEN** `observe.otel_endpoint` 在 SystemConfig 中有非空值
- **THEN** 详情页 SHALL 展示完整的 Endpoint URL，并在配置片段中自动填充

#### Scenario: Endpoint 未配置
- **WHEN** `observe.otel_endpoint` 为空
- **THEN** 详情页 SHALL 在 Endpoint 位置显示提示文案，配置片段中的 Endpoint 占位符保持可见

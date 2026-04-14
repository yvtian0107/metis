# apm-trace-flamegraph Specification

## Purpose
TBD - created by archiving change apm-pro-upgrade. Update Purpose after archive.
## Requirements
### Requirement: 火焰图视图
系统 SHALL 提供火焰图（Flame Chart）作为 Trace Detail 的第二种可视化模式，与瀑布图并列可切换。火焰图 SHALL 使用 Canvas 2D API 渲染。

#### Scenario: 切换到火焰图
- **WHEN** 用户在 Trace Detail 页面点击「Flame Graph」视图切换按钮
- **THEN** 主视图区域切换为火焰图渲染，X 轴为时间范围，Y 轴为调用栈深度

### Requirement: 火焰图渲染规则
火焰图中每个矩形块 SHALL 对应一个 span。块的 X 坐标 SHALL 对应 span 的 startTime 在 trace 时间范围中的位置，宽度 SHALL 对应 duration 占总时间的比例。Y 坐标 SHALL 对应调用栈深度（root span 在顶部）。颜色 SHALL 按 service 着色（与瀑布图一致的配色方案）。

#### Scenario: 正确渲染 span 块
- **WHEN** trace 包含嵌套 span（parent→child 关系）
- **THEN** child span 渲染在 parent span 下方一层，X 坐标在 parent 范围内

### Requirement: 关键路径高亮
系统 SHALL 识别并高亮 trace 的关键路径——从 root span 到耗时最长叶子 span 的执行链路。关键路径上的 span 块 SHALL 有明显视觉区分（加粗边框或高亮背景）。

#### Scenario: 关键路径标识
- **WHEN** 火焰图渲染完成
- **THEN** 关键路径上的 span 块有 2px 白色边框标识，其余 span 块无边框

### Requirement: 火焰图交互
用户 SHALL 可以 hover 火焰图中的 span 块显示 tooltip（serviceName、spanName、duration）。点击 span 块 SHALL 在右侧详情面板中显示该 span 的完整信息。

#### Scenario: hover span 块
- **WHEN** 用户 hover 火焰图中的一个 span 块
- **THEN** 显示 tooltip 包含 serviceName、spanName、durationMs

#### Scenario: 点击 span 块
- **WHEN** 用户点击火焰图中的一个 span 块
- **THEN** 右侧 Span 详情面板更新为该 span 的属性、事件、资源信息


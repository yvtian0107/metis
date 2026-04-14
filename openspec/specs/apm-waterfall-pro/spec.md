# apm-waterfall-pro Specification

## Purpose
TBD - created by archiving change apm-pro-upgrade. Update Purpose after archive.
## Requirements
### Requirement: 时间刻度尺
瀑布图顶部 SHALL 显示时间刻度尺，标注从 0ms 到 totalDuration 的 5 等分刻度，每个刻度显示对应毫秒数并绘制垂直虚线贯穿整个图表。

#### Scenario: 刻度尺显示
- **WHEN** trace 的总时长为 250ms
- **THEN** 刻度尺显示 0ms、50ms、100ms、150ms、200ms、250ms，每个位置有垂直虚线

### Requirement: Span 折叠展开
瀑布图 SHALL 支持按 parent span 折叠/展开子树。折叠时 SHALL 显示一个展开图标和隐藏的子 span 数量。默认展开所有 span。

#### Scenario: 折叠子树
- **WHEN** 用户点击一个有子 span 的 span 左侧折叠按钮
- **THEN** 该 span 的所有后代 span 隐藏，显示 "▶ +N spans" 提示

#### Scenario: 展开子树
- **WHEN** 用户点击已折叠 span 的展开按钮
- **THEN** 所有后代 span 恢复显示

### Requirement: Service 颜色图例
瀑布图 SHALL 在顶部（刻度尺下方）显示 service 颜色图例栏，列出所有参与服务及其对应颜色标记。

#### Scenario: 颜色图例显示
- **WHEN** trace 涉及 3 个服务（api-gateway、user-service、postgres）
- **THEN** 图例栏显示 3 个带颜色圆点的服务名标签

### Requirement: Span 搜索过滤
瀑布图 SHALL 提供搜索框，用户可输入关键词过滤 spanName 或 serviceName。匹配的 span SHALL 高亮显示，不匹配的 SHALL 降低透明度（不隐藏）。

#### Scenario: 搜索匹配
- **WHEN** 用户输入 "postgres" 到搜索框
- **THEN** 包含 "postgres" 的 serviceName 或 spanName 的 span 行高亮，其余 span 行透明度降低

### Requirement: 关键路径高亮
瀑布图中关键路径上的 span bar SHALL 有 2px 实线左边框标记，区别于普通 span。

#### Scenario: 关键路径标记
- **WHEN** trace 的关键路径为 root → serviceA → dbQuery
- **THEN** 这三个 span 的时间条有额外的左边框高亮标记

### Requirement: 左右分栏布局
Trace Detail 页面 SHALL 采用左右分栏布局。左侧显示瀑布图/火焰图/Span 列表，右侧显示选中 span 的详情面板（替换现有 Sheet 弹窗模式）。未选中 span 时右侧面板显示 trace 概要信息。

#### Scenario: 选中 span 显示详情
- **WHEN** 用户在瀑布图中点击一个 span
- **THEN** 右侧面板更新为该 span 的详情（属性、事件、资源），无需弹窗

#### Scenario: 未选中 span
- **WHEN** 页面加载后未点击任何 span
- **THEN** 右侧面板显示 trace 概要（总时长、服务数、span 数、root operation）


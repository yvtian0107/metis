## ADDED Requirements

### Requirement: 自定义日期范围
TimeRangePicker SHALL 在预设按钮（15m/1h/6h/24h/7d）旁新增「Custom」按钮。点击后弹出 Popover 包含两个 `input[type="datetime-local"]`（start 和 end），用户设置后点击 Apply 应用自定义范围。

#### Scenario: 选择自定义范围
- **WHEN** 用户点击 Custom 按钮，设置 start=2024-01-01T00:00 和 end=2024-01-02T00:00，点击 Apply
- **THEN** 时间范围更新为用户指定的区间，预设按钮不再有 active 高亮

#### Scenario: 取消自定义
- **WHEN** 用户打开 Custom Popover 后点击外部关闭
- **THEN** 时间范围保持不变

### Requirement: 自动刷新
TimeRangePicker SHALL 提供自动刷新间隔选择（Off / 10s / 30s / 1m / 5m）。启用后 SHALL 按间隔自动重新计算时间窗口并触发数据刷新。

#### Scenario: 启用自动刷新
- **WHEN** 用户选择 30s 自动刷新
- **THEN** 每 30 秒自动刷新时间范围和数据查询，直到用户切换为 Off 或离开页面

#### Scenario: 页面卸载清理
- **WHEN** 用户从 APM 页面导航到其他页面
- **THEN** 自动刷新定时器被清除

### Requirement: 时间范围全局共享
`useTimeRange` hook SHALL 将当前时间范围存储在 URL searchParams（start、end 参数）中，使所有使用该 hook 的页面共享同一时间上下文。

#### Scenario: 时间范围通过 URL 传递
- **WHEN** 用户在 Trace Explorer 设置时间范围后点击 Service 进入 Service Detail
- **THEN** Service Detail 页面从 URL 中读取 start/end 参数，使用相同的时间范围

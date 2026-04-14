## ADDED Requirements

### Requirement: ReactFlow 交互式服务拓扑
系统 SHALL 使用 @xyflow/react 渲染服务拓扑图，替换现有手写 dagre+SVG 实现。拓扑图 SHALL 支持 pan（拖拽画布）、zoom（滚轮缩放）、minimap（右下角缩略导航）和 controls（缩放控制按钮）。

#### Scenario: 拓扑图基础交互
- **WHEN** 用户打开 /apm/topology 页面且有拓扑数据
- **THEN** 系统显示 ReactFlow 画布，节点按 dagre 分层布局排列，用户可以拖拽画布平移、滚轮缩放、通过 minimap 快速导航

### Requirement: 自定义服务节点
每个服务节点 SHALL 渲染为自定义 React 组件，包含：服务名、健康状态指示（绿/黄/红圆点，基于 errorRate 阈值 0%/5%/10%）、Request Count 数字、Error Rate 百分比。

#### Scenario: 正常服务节点
- **WHEN** 服务 errorRate < 5%
- **THEN** 节点显示绿色健康指示灯，背景为默认卡片色

#### Scenario: 警告服务节点
- **WHEN** 服务 errorRate >= 5% 且 < 10%
- **THEN** 节点显示黄色健康指示灯，边框为黄色

#### Scenario: 错误服务节点
- **WHEN** 服务 errorRate >= 10%
- **THEN** 节点显示红色健康指示灯，边框为红色

### Requirement: 自定义调用边
边 SHALL 显示调用量和平均延迟标签。线宽 SHALL 按 callCount 归一化（最小 1.5px，最大 5px）。error > 5% 的边 SHALL 渲染为红色。边 SHALL 使用 animated 属性表示流量方向。

#### Scenario: 高错误率边
- **WHEN** 两个服务间调用的 errorRate > 5%
- **THEN** 边渲染为红色，标签显示 callCount、avgLatency 和 errorRate

### Requirement: 节点点击钻取
点击拓扑节点 SHALL 导航到该服务的详情页 `/apm/services/:name`。

#### Scenario: 点击节点跳转
- **WHEN** 用户点击拓扑图中的服务节点
- **THEN** 页面跳转到 `/apm/services/{serviceName}?start=...&end=...`，携带当前时间范围

### Requirement: 边 hover 详情
hover 拓扑边 SHALL 显示 tooltip，包含：caller→callee、callCount、avgDuration、p95Duration、errorRate。

#### Scenario: hover 边显示详情
- **WHEN** 用户 hover 一条拓扑边
- **THEN** 显示 tooltip 展示该调用关系的完整指标

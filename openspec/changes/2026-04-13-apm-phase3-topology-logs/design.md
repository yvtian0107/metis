## Context

Phase 1 提供了 Trace Explorer，Phase 2 提供了 Service Catalog + Detail。APM App 已具备完整的 trace 查询和 service 指标能力。本次补全两个关键能力：

1. **服务间关系**——从 span 的 parent-child 关系推导 caller→callee 跨服务调用
2. **日志关联**——通过 TraceId 从 `otel_logs` 表查询同一 trace 的日志

## Goals / Non-Goals

**Goals:**
- Topology API：从 span parent-child 关系推导服务间调用图 + 调用指标
- Trace Logs API：通过 TraceId 查询 `otel_logs` 表关联日志
- 前端 Service Map 页面（有向拓扑图 + 节点/边指标）
- 前端 Trace Detail 页面新增 Logs tab

**Non-Goals:**
- 不做实时拓扑（按需查询，非 WebSocket 推送）
- 不做日志全文搜索（只做 trace 关联查询）
- 不做独立的日志查询页面（Log Explorer 是未来独立模块）
- 不做延迟分布直方图 / 热力图

## Decisions

### D1：拓扑推导查询

**决策**：self-JOIN 推导跨服务调用：

```sql
SELECT
    parent.ServiceName  AS Caller,
    child.ServiceName   AS Callee,
    count()             AS CallCount,
    avg(child.Duration) / 1e6 AS AvgDurationMs,
    quantile(0.95)(child.Duration) / 1e6 AS P95DurationMs,
    countIf(child.StatusCode = 'STATUS_CODE_ERROR') * 100.0 / count() AS ErrorRate
FROM otel_traces AS child
INNER JOIN otel_traces AS parent
    ON child.ParentSpanId = parent.SpanId
    AND child.TraceId = parent.TraceId
WHERE child.ServiceName != parent.ServiceName
  AND child.Timestamp >= {start:DateTime64}
  AND child.Timestamp <= {end:DateTime64}
GROUP BY Caller, Callee
```

---

### D2：拓扑图技术选择

**决策**：`@dagrejs/dagre` 做自动布局 + 自定义 SVG 渲染。

**理由**：dagre 纯布局计算无 UI 依赖，配合自定义 SVG 完全控制样式。ReactFlow 对只读拓扑图过重。

---

### D3：日志关联查询

**决策**：通过 TraceId 查 `otel_logs`，`otel_logs` 表不存在时返回空列表 + `logsAvailable: false`。

---

### D4：日志表降级

**决策**：查 `otel_logs` 时捕获表不存在错误，返回空列表 + warning flag。前端根据 flag 显示引导提示。

## Risks / Trade-offs

**[Self-JOIN 性能]** → 大数据量下可能秒级。
**缓解**：强制时间范围参数、前端 loading 状态。

**[拓扑图布局]** → 服务数 >50 时可能杂乱。
**缓解**：目标场景 <30 服务。

## Migration Plan

1. 后端新增 Topology + Logs 查询方法和 API
2. `cd web && bun add @dagrejs/dagre`
3. 实现 Service Map + Trace Detail Logs tab
4. 更新菜单 seed
5. 验证

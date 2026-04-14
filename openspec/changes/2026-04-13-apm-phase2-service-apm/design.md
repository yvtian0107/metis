## Context

Phase 1 完成后，APM App 具备了 Trace Explorer 能力（列表 + 瀑布图），用户可以搜索和查看单条 trace。但运维日常最需要的是「全局服务健康概览」——一眼看到所有服务的吞吐量、延迟、错误率，定位异常后再钻取到 trace 级别。

ClickHouse 的 `otel_traces` 表 ORDER BY `(ServiceName, SpanName, toUnixTimestamp(Timestamp))` 对按服务+操作+时间的聚合查询非常高效，可以直接实时 GROUP BY，不需要预聚合。

## Goals / Non-Goals

**Goals:**
- Service Catalog API：聚合所有服务的 request_count、avg_duration、P95、error_rate
- Service Detail API：单服务的时序趋势 + operation 列表指标
- Timeseries API：按可配置时间桶聚合趋势数据
- 前端 Service Catalog 页面（服务表格 + sparkline）
- 前端 Service Detail 页面（指标卡片 + 趋势图 + operation 表格）
- 从 Service Detail 跳转 Trace Explorer 时自动填充过滤条件

**Non-Goals:**
- 不做预聚合 / 物化视图
- 不做自定义仪表盘——固定布局
- 不做告警功能
- 不做 Service Map 拓扑图（Phase 3）

## Decisions

### D1：Service 聚合查询策略

**决策**：对 `otel_traces` 表做 `GROUP BY ServiceName`，只统计 `SpanKind = 'SPAN_KIND_SERVER'` 的 span。

```sql
SELECT
    ServiceName,
    count()                                           AS RequestCount,
    avg(Duration) / 1e6                               AS AvgDurationMs,
    quantile(0.50)(Duration) / 1e6                    AS P50Ms,
    quantile(0.95)(Duration) / 1e6                    AS P95Ms,
    quantile(0.99)(Duration) / 1e6                    AS P99Ms,
    countIf(StatusCode = 'STATUS_CODE_ERROR') * 100.0 / count() AS ErrorRate,
    min(Timestamp)                                    AS FirstSeen,
    max(Timestamp)                                    AS LastSeen
FROM otel_traces
WHERE Timestamp >= {start:DateTime64}
  AND Timestamp <= {end:DateTime64}
  AND SpanKind = 'SPAN_KIND_SERVER'
GROUP BY ServiceName
ORDER BY RequestCount DESC
```

**理由**：只统计 SERVER span 才是真实的「请求量」，避免 client/internal span 重复计数。

---

### D2：时序聚合策略

**决策**：`toStartOfInterval(Timestamp, INTERVAL N SECOND)` 做时间桶。前端根据时间范围自动选 interval：

| 时间范围 | interval | 数据点 |
|---------|----------|-------|
| 15m | 15s | ~60 |
| 1h | 1m | 60 |
| 6h | 5m | 72 |
| 24h | 15m | 96 |
| 7d | 1h | 168 |

---

### D3：Sparkline 数据策略

**决策**：Service Catalog 表格的 sparkline 数据通过一次查询返回所有服务的 mini 时序（`GROUP BY ServiceName, time_bucket`），避免 N+1。

---

### D4：图表库选择

**决策**：使用 `recharts` 绘制时序图和 sparkline。

**理由**：React 生态最成熟的图表库，声明式 API，tree-shakeable。

---

### D5：Service → Trace 跳转

**决策**：点击 operation 行跳转 `/apm/traces?service=xxx&operation=yyy&start=...&end=...`，Trace Explorer 从 URL searchParams 初始化过滤器。

## Risks / Trade-offs

**[实时聚合性能]** → 数据量极大时聚合可能变慢。
**缓解**：ClickHouse 列式存储百万级 span 聚合通常 <500ms。

**[recharts bundle size]** → 完整包 ~200KB gzipped。
**缓解**：只导入使用的组件，tree-shaking 后 ~80KB。

## Migration Plan

1. 后端新增 Service 聚合查询方法和 API endpoint
2. `cd web && bun add recharts`
3. 实现 Service Catalog + Service Detail 页面
4. 更新 APM App 菜单 seed
5. 修改 Trace Explorer 支持 URL 参数预填充
6. 验证：`go build -tags dev ./cmd/server/` + `cd web && bun run lint`

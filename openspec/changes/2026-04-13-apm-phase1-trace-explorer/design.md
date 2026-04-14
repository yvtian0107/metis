## Context

Observe App 已完成数据接入层：IntegrationToken 鉴权 + Traefik ForwardAuth + OTel Collector → ClickHouse 写入链路。ClickHouse 使用官方 `clickhouse` exporter 默认表结构（`otel_traces`），ORDER BY `(ServiceName, SpanName, toUnixTimestamp(Timestamp))`，对按服务、操作、时间查询天然友好。

本次引入独立的 APM App，负责 ClickHouse 只读查询 + 前端展示。与 Observe App 的职责划分：

```
Observe App — 数据接入鉴权（Token 管理 + ForwardAuth）
APM App    — 数据查询展示（ClickHouse 查询 + 前端 UI）
```

## Goals / Non-Goals

**Goals:**
- 新建独立 App `internal/app/apm/`，实现 App 接口
- 在 `config.yml` 中支持 ClickHouse 连接配置，安装向导高级设置可配置
- 实现 ClickHouse 客户端封装，遵循 FalkorDB 的 nil 降级模式
- 提供 Trace 列表查询 API，支持 service/operation/status/duration/time_range 过滤
- 提供 Trace 详情查询 API，返回单个 trace 的全部 span
- 前端 `web/src/apps/apm/` 独立 App 模块
- 前端 Trace Explorer 页面（列表 + 过滤器）
- 前端 Trace Detail 页面（瀑布图 + Span 属性 Sheet）
- 前端时间范围选择器（全局复用组件）

**Non-Goals:**
- 不做 Service 聚合视图（Phase 2）
- 不做拓扑图（Phase 3）
- 不做日志关联（Phase 3）
- 不做预聚合表或物化视图——直接实时查询 ClickHouse
- 不做 ClickHouse 写入——数据由 OTel Collector 写入，Metis 只读
- 不做 ClickHouse 健康检查定时任务——连接时验证即可

## Decisions

### D1：独立 App 而非 Observe App 子功能

**决策**：APM 作为独立 App `internal/app/apm/` 实现，与 Observe App 平级。

**理由**：
- APM 有独立的外部数据源（ClickHouse），而 Observe 的 IntegrationToken 依赖 GORM DB，数据源不同
- APM 可通过 build tag / edition 独立裁剪（`APPS=system,apm` 或 `APPS=system,observe,apm`）
- 前端 `web/src/apps/apm/` 独立模块，路由前缀 `/apm/*`
- API 前缀 `/api/v1/apm/*`，不与 observe 的 `/api/v1/observe/*` 混杂
- 职责边界清晰：Observe 管接入鉴权，APM 管查询展示

**备选**：放在 Observe App 内 — 否决，两个模块的数据源、职责、裁剪需求完全不同，耦合在一起会导致 Observe 膨胀。

---

### D2：ClickHouse 配置存储位置

**决策**：存入 `config.yml`（`ClickHouseConfig` struct），与 FalkorDB 同级，不存 SystemConfig。

**理由**：ClickHouse 是基础设施级连接（如同数据库 DSN），修改后需要重启应用重新建立连接池。与 FalkorDB 完全一致的处理方式，保持架构一致性。

**备选**：存 SystemConfig（运行时可改）— 否决，ClickHouse 连接池需要重建，热更新复杂度高且收益低。

---

### D3：ClickHouse 连接库选择

**决策**：使用 `github.com/ClickHouse/clickhouse-go/v2`，通过 `database/sql` 接口连接。

**理由**：ClickHouse 官方 Go driver，支持 Native protocol（端口 9000）和 HTTP protocol（端口 8123）。DSN 格式 `clickhouse://user:pass@host:9000/database`。

---

### D4：nil 降级模式

**决策**：`NewClickHouseClient(i do.Injector)` 在 `cfg.ClickHouse == nil` 时返回 `nil, nil`（不报错），Handler 层检查 client 是否为 nil，返回 HTTP 503。

**理由**：完全复用 FalkorDB 的降级模式。APM App 的 `Models()` 返回空（无 GORM 表），`Routes()` 正常注册（前端总是能访问页面，只是数据请求返回 503）。

---

### D5：查询 `otel_traces` 表结构映射

**决策**：直接查询 ClickHouse 官方 exporter 创建的 `otel_traces` 表，不创建额外视图。Go 侧定义查询结果 struct，不使用 ORM。

官方 `otel_traces` 表关键字段：

| 字段 | 类型 | 用途 |
|------|------|------|
| Timestamp | DateTime64(9) | span 开始时间 |
| TraceId | String | trace 标识 |
| SpanId | String | span 标识 |
| ParentSpanId | String | 父 span（空=root） |
| SpanName | LowCardinality(String) | operation name |
| SpanKind | LowCardinality(String) | SPAN_KIND_SERVER 等 |
| ServiceName | LowCardinality(String) | 服务名 |
| Duration | Int64 | 纳秒 |
| StatusCode | LowCardinality(String) | STATUS_CODE_OK/ERROR/UNSET |
| StatusMessage | String | 错误信息 |
| SpanAttributes | Map(LowCardinality(String), String) | span 属性 |
| ResourceAttributes | Map(LowCardinality(String), String) | resource 属性 |
| Events.Timestamp | Array(DateTime64(9)) | event 时间 |
| Events.Name | Array(LowCardinality(String)) | event 名 |
| Events.Attributes | Array(Map(...)) | event 属性 |

---

### D6：Trace 列表查询策略

**决策**：Trace 列表只查 root span（`ParentSpanId = ''`），用子查询统计每个 trace 的 span 数量和是否含 error。

```sql
SELECT
    t.TraceId,
    t.ServiceName,
    t.SpanName AS RootOperation,
    t.Duration / 1e6 AS DurationMs,
    t.StatusCode,
    t.Timestamp,
    counts.SpanCount,
    counts.HasError
FROM otel_traces t
INNER JOIN (
    SELECT
        TraceId,
        count() AS SpanCount,
        max(StatusCode = 'STATUS_CODE_ERROR') AS HasError
    FROM otel_traces
    WHERE Timestamp >= {start:DateTime64}
      AND Timestamp <= {end:DateTime64}
    GROUP BY TraceId
) counts ON t.TraceId = counts.TraceId
WHERE t.ParentSpanId = ''
  AND t.Timestamp >= {start:DateTime64}
  AND t.Timestamp <= {end:DateTime64}
ORDER BY t.Timestamp DESC
LIMIT {limit:UInt32} OFFSET {offset:UInt32}
```

---

### D7：Trace 详情查询

**决策**：`SELECT * FROM otel_traces WHERE TraceId = ? ORDER BY Timestamp ASC`，返回全部字段。

**理由**：一个 trace 通常几十个 span，全量返回无压力。前端用数据渲染瀑布图 + span 详情。

---

### D8：时间范围处理

**决策**：所有查询 API 必须传 `start` + `end` 参数（ISO8601），后端不设默认值。前端时间范围选择器提供快捷选项（Last 15m / 1h / 6h / 24h / 7d / Custom）并转换为绝对时间。

---

### D9：安装向导 ClickHouse 配置

**决策**：Step 2 高级设置新增 ClickHouse 区块，字段：Toggle + DSN 输入框。后端 `ExecuteRequest` 新增 `ClickHouseDSN string`，非空时写入 config.yml。

---

### D10：瀑布图前端实现

**决策**：自定义实现，用 div + CSS 定位渲染 span bar。支持：
- 按 parent-child 关系递归构建 span 树
- 每个 span 为水平 bar，长度正比 duration，颜色按 service 区分
- Error span 红色高亮
- 点击 span 打开 Sheet 展示 attributes/events/resource

**理由**：核心交互组件需完全控制样式，代码量 ~200 行，与 shadcn/ui + Tailwind 风格统一。

---

### D11：APM App 的 App 接口实现

**决策**：

```go
// internal/app/apm/app.go
type APMApp struct { injector do.Injector }

func (a *APMApp) Name() string { return "apm" }
func (a *APMApp) Models() []any { return nil }  // 无 GORM 表
func (a *APMApp) Seed(db, enforcer) error { return seedAPM(db, enforcer) }
func (a *APMApp) Providers(i) { /* CH client + repo + service + handler */ }
func (a *APMApp) Routes(api) { /* /apm/traces, /apm/traces/:traceId */ }
func (a *APMApp) Tasks() []scheduler.TaskDef { return nil }
```

**理由**：`Models()` 返回 nil（不操作 GORM DB），`Tasks()` 返回 nil（无定时任务）。Seed 只注册菜单和 Casbin 策略。

## Risks / Trade-offs

**[ClickHouse Map 类型序列化]** → `SpanAttributes` 和 `ResourceAttributes` 是 `Map(LowCardinality(String), String)` 类型。
**缓解**：clickhouse-go/v2 原生支持 Map 类型反序列化为 `map[string]string`。

**[Nested Array 类型]** → Events 是三个独立 Array 列，需手动 zip。
**缓解**：repository 层用 struct 接收三个 array，Go 侧组装为 `[]SpanEvent`。

**[大 Trace]** → 极端情况一个 trace 可能有数千 span。
**缓解**：前端瀑布图做虚拟滚动，后端暂不限制。

**[APM App 对 ClickHouse 的独占依赖]** → ClickHouse 配置放在全局 config.yml，但只有 APM App 使用。
**缓解**：与 FalkorDB 模式一致（FalkorDB 也是全局配置但只有 AI App 使用），架构上没问题。如果未来有其他 App 也需要 ClickHouse，可共享同一连接。

## Migration Plan

1. `go get github.com/ClickHouse/clickhouse-go/v2` 引入依赖
2. 修改 `config.go` 加入 ClickHouseConfig，修改 `install.go` 支持安装时配置
3. 创建 `internal/app/apm/` 目录，实现 App 接口全套文件
4. `edition_full.go` 加 import
5. 前端创建 `web/src/apps/apm/` 模块，实现 Trace Explorer + Trace Detail
6. 安装向导加 ClickHouse 配置区块
7. 更新 docker-compose
8. 验证：`go build -tags dev ./cmd/server/` + `cd web && bun run lint`

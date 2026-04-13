## Context

Metis 目前的 `internal/telemetry/` 模块负责 Metis 自身作为 OTel 客户端向外发送 trace 数据。本次变更方向相反：引入 `observe` App，使外部服务能够将 OTel 数据安全地推送进来。

数据链路已就绪：ClickHouse 已在 docker-compose 中运行，Traefik 作为 gateway，OTel Collector（otelcol-contrib）作为数据接收层。缺少的是**鉴权层**和**接入引导层**。

项目采用可插拔 App 架构（`internal/app/app.go` 定义的 App 接口），Node App 已提供完整的独立 Token 认证先例（`mtk_` 前缀 + bcrypt hash + Traefik 独立路由组），本次设计直接复用该模式。

## Goals / Non-Goals

**Goals:**
- 实现 Integration Token 的完整生命周期（创建/列表/撤销），每用户最多 10 个
- 提供 Traefik ForwardAuth 验证端点，验证通过后注入数据归属 header
- 前端提供 DataDog 风格的 Integration Catalog 和 Token 管理页面
- 11 个硬编码集成模板，覆盖 APM/Metrics/Logs 三类常见场景
- 数据模型预留 `org_id`/`scope` 字段，支持未来组织级 Token 和数据权限

**Non-Goals:**
- 不实现可观测数据的查询、展示、告警功能（后续模块）
- 不部署或配置 OTel Collector、ClickHouse、Traefik（假设已就绪）
- 不实现组织级 Token（预留字段，逻辑留到 org 模块集成时）
- 不实现 Token 使用量统计或速率限制

## Decisions

### D1：独立 App 而非内核功能

**决策**：作为独立 App `internal/app/observe` 实现。

**理由**：有自己的数据模型（`IntegrationToken` 表）、独立路由组、独立 Seed 菜单，符合 App 架构语义。未来可通过 `APPS=system,observe` 按需裁剪。放入内核会污染内核职责边界。

**备选**：放入内核 `internal/handler/` — 否决，内核不应承载业务模块数据模型。

---

### D2：Token 格式与存储

**决策**：`itk_<32byte-hex>` 格式，只存 bcrypt hash + 8字符前缀（`itk_` + 前4位hex = 8字符），创建时一次性返回完整明文，之后不可再取。

**理由**：与 Node Token（`mtk_`）完全同构，利用相同的 `GenerateToken` / `ValidateToken` 函数模式。bcrypt hash 防止数据库泄露导致 Token 被复用。prefix 用于数据库按前缀查找，避免全表扫描。

**备选**：AES 加密存储（可逆）— 否决，不需要 "找回" Token，不可逆更安全。

---

### D3：ForwardAuth 端点的路由注册方式

**决策**：在 `Routes()` 方法中通过 `do.MustInvoke[*gin.Engine](a.injector)` 获取原始 Engine，注册独立路由组 `/api/v1/observe/auth`，绑定自定义 `IntegrationTokenMiddleware`，完全绕过 JWT+CasbinAuth 中间件链。

**理由**：与 Node Sidecar（`/api/v1/nodes/sidecar`）完全相同的注册模式，已被验证可行。OTLP 请求来自外部服务，无法携带用户 JWT，必须走独立鉴权路径。不需要修改 Casbin 白名单。

**备选**：将 verify 路径加入 CasbinAuth 白名单 — 否决，白名单跳过的是 Casbin 权限检查，JWT 验证仍然会执行，而外部服务没有 JWT。

---

### D4：数据归属注入方式

**决策**：ForwardAuth verify 成功后，在 HTTP response header 中返回：
```
X-Metis-User-Id: <user_id>
X-Metis-Token-Id: <token_id>
X-Metis-Scope: personal
X-Metis-Org-Id: (空，未来填充)
```
Traefik 的 `forwardAuth.authResponseHeaders` 配置将这些 header 透传给 OTel Collector，Collector 通过 `transform` processor 将其写入 resource attributes（`metis.user_id`、`metis.token_id` 等），最终随数据写入 ClickHouse。

**理由**：数据归属信息在 gateway 层注入，比在 Metis 服务内代理请求更轻量，OTel Collector 原生支持 header → attribute 转换。未来 ClickHouse 查询层可直接按 `metis.user_id` 过滤实现数据权限。

**备选**：Metis 作为 OTLP 代理接收数据再转发 — 否决，引入不必要的性能开销，且偏离 Metis 的职责定位。

---

### D5：Integration Catalog 数据来源

**决策**：前端 TypeScript 硬编码，维护在 `web/src/apps/observe/data/integrations.ts` 中。

**理由**：Integration 模板是静态内容（icon、名称、引导文案、配置片段），无需数据库存储。前端维护成本低，模板更新不需要后端发布。未来如需动态化（第三方插件）再迁移。

**备选**：后端 DB 存储 — 否决，当前场景无动态需求，过度设计。

---

### D6：OTel Endpoint 配置

**决策**：新增 SystemConfig key `observe.otel_endpoint`，在 Seed 时写入默认空字符串，管理员通过系统设置页面配置实际 Traefik 转发地址。前端引导页通过 API 读取并填充到配置片段中。

**理由**：与现有 SystemConfig 模式一致（如 `otel.exporter_endpoint`），管理员配置和用户使用分离，不依赖 `site.url`。

## Risks / Trade-offs

**[bcrypt 验证性能]** → ForwardAuth 端点每次 OTLP 请求都需要 bcrypt 校验（~100ms CPU），高频数据推送下可能成为瓶颈。**缓解**：引入短期内存缓存（token hash → user info，TTL 60s），验证通过后缓存结果，减少重复 bcrypt 计算。

**[Token 数量上限]** → 上限 10 个是硬编码常量，未来可能需要调整。**缓解**：常量定义在 service 层，修改不涉及数据库 schema。

**[prefix 碰撞]** → 8字符 hex prefix 理论上有碰撞风险（1/2^32），同一前缀找到多个 Token 时需逐一 bcrypt 验证。**缓解**：碰撞概率极低（同 Node Token 现有设计），多 Token 时仅多一次 bcrypt，不影响正确性。

**[引导文案维护成本]** → 11 个集成的 Docker/Binary 两套配置片段，OTel Collector 版本升级时需手动更新。**缓解**：配置片段集中在单一 TS 文件，易于统一更新；片段中的 Collector 镜像版本作为常量维护。

## Migration Plan

1. 运行时启动：App.Models() 触发 `integration_tokens` 表 AutoMigrate，无需手动 DDL
2. Seed 写入菜单和 Casbin 策略（幂等），写入默认 SystemConfig key `observe.otel_endpoint`
3. 无数据迁移，无 breaking change，可随主版本正常发布

## Open Questions

- Traefik 的 `forwardAuth.authResponseHeaders` 配置示例是否需要纳入引导文案（针对需要自建 Traefik 路由的用户）？建议：纳入一个独立的 "Advanced / 自托管部署" 折叠区块，不影响主流程。

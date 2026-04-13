## Context

`observe` App 已在代码层面实现了 Integration Token 生命周期管理（创建/列表/撤销）和 ForwardAuth 验证端点（`AuthHandler.Verify`），但数据接入链路的基础设施配置存在缺口，导致安全与数据归属功能未真正生效：

- `docker-compose.yml` 中的 Traefik 仅做了简单路由转发，未调用 `/api/v1/observe/auth/verify` 进行 Token 校验。
- `otel-collector-config.yaml` 未配置 header → attribute 转换，验证成功后透传的 `X-Metis-*` headers 不会被写入 trace/metric/log 的 resource attributes。
- 前端 Go SDK snippet 使用了 `otlptracehttp.WithEndpoint`，该选项只接受 `host:port`，不接受含 scheme 的完整 URL，用户直接复制配置会导致初始化失败。

## Goals / Non-Goals

**Goals:**
- 补全 Traefik ForwardAuth 中间件配置，使外部 OTLP HTTP 请求必须经过 Metis Token 验证
- 补全 OTel Collector 的 `transform` processor，将 `X-Metis-User-Id` / `X-Metis-Token-Id` / `X-Metis-Scope` 写入 resource attributes
- 删除不再使用的 `IntegrationTokenMiddleware` 死代码
- 修复前端 Go SDK snippet 和验证命令的准确性

**Non-Goals:**
- 不修改 Integration Token 的生成/校验算法
- 不新增 ClickHouse 查询或数据展示功能
- 不引入组织级 Token 逻辑（`org_id` 字段继续保留但为空）

## Decisions

### D1：Traefik ForwardAuth 配置方式

**决策**：在 `docker-compose.yml` 的 `traefik` service 中新增 labels，为 `otlp-http` entrypoint 挂载 `forwardAuth` middleware：

```yaml
- "traefik.http.middlewares.otlp-auth.forwardauth.address=http://host.docker.internal:8080/api/v1/observe/auth/verify"
- "traefik.http.middlewares.otlp-auth.forwardauth.authResponseHeaders=X-Metis-User-Id,X-Metis-Token-Id,X-Metis-Scope,X-Metis-Org-Id"
- "traefik.http.routers.otlp.middlewares=otlp-auth"
```

**理由**：`observe` App 已在 `app.go` 中将 `auth/verify` 注册到原始 `gin.Engine`，该端点绕过 JWT+Casbin，可直接被 Traefik 调用。使用 labels 是 Docker Compose 下 Traefik 的标准做法，无需额外配置文件。

**备选**：在 `traefik.yml` 静态配置中定义 middleware — 否决，当前开发环境完全依赖 labels，引入静态文件会增加维护成本。

---

### D2：Collector transform processor 实现方式

**决策**：在 `otel-collector-config.yaml` 的 `processors` 段增加 `transform` processor，并在 traces/metrics/logs 三个 pipeline 中插入：

```yaml
processors:
  transform/metis:
    error_mode: ignore
    trace_statements:
      - context: resource
        statements:
          - set(attributes["metis.user_id"], request.headers["X-Metis-User-Id"][0])
          - set(attributes["metis.token_id"], request.headers["X-Metis-Token-Id"][0])
          - set(attributes["metis.scope"], request.headers["X-Metis-Scope"][0])
    metric_statements:
      - context: resource
        statements: ...
    log_statements:
      - context: resource
        statements: ...
```

**理由**：OTel Collector Contrib 的 `transform` processor 原生支持从 `request.headers` 读取值并写入 resource attributes，`error_mode: ignore` 可防止 header 缺失时导致数据丢弃。

**备选**：在 Metis 内部实现一个 OTLP 代理路由，由 Metis 接收数据、注入 attributes 后再转发给 Collector — 否决，这会给 Metis 带来不必要的性能与内存开销，且偏离其职责定位。

---

### D3：死代码清理策略

**决策**：直接删除 `internal/app/observe/middleware.go`。

**理由**：该文件中的 `IntegrationTokenMiddleware` 在 `app.go` 的 `Routes()` 中完全没有被引用。设计阶段原本考虑用 middleware 直接保护某些 Metis 内部路由，但实际实现已切换为 Traefik ForwardAuth 模式（由网关层做鉴权）。保留死代码会误导后续维护者。

---

### D4：Go SDK snippet 修复

**决策**：将 Go snippet 中的 `otlptracehttp.WithEndpoint("{{ENDPOINT}}")` 替换为 `otlptracehttp.WithEndpointURL("{{ENDPOINT}}/v1/traces")`。

**理由**：`WithEndpoint` 仅接收主机地址（如 `otel.example.com:4318`），而 `observe.otel_endpoint` 存储的是完整 URL（含 scheme）。`WithEndpointURL` 支持完整 URL，与其他语言 snippet 保持一致。

---

### D5：Verify 端点响应体验

**决策**：在 `AuthHandler.Verify` 验证成功时返回一个轻量 JSON `{"ok":true}`，而不是空 body。

**理由**：不影响 Traefik ForwardAuth（它只关心 status code 和 headers），但可改善开发调试体验，使用 curl 测试时能立即确认端点正常。

## Risks / Trade-offs

**[开发环境网络连通性]** → Traefik 在 Docker 容器内，Metis 服务在宿主机（`:8080`），使用 `host.docker.internal:8080` 在部分 Linux 环境下可能不可用。
**缓解**：在 design 文档中标注该限制，生产环境应使用内网 DNS 或容器网络名；开发环境若遇问题，可临时改用 `172.17.0.1` 或加入同一 Docker 网络。

**[transform processor 语法兼容性]** → `request.headers[...][0]` 语法要求 otelcol-contrib ≥ 0.85.0，当前 `docker-compose.yml` 使用 `latest` 标签，通常满足。
**缓解**：配置中设置 `error_mode: ignore`，即使语法因版本差异失效，也不会阻断数据收集，只会缺失归属 attributes。

**[header 大小写敏感性]** → OTel Collector 的 `request.headers` 键名匹配是否区分大小写取决于版本实现。
**缓解**：Traefik 透传时保持原样；在 transform 中统一使用大写首字母的键名 `X-Metis-User-Id`（HTTP 标准写法）。

## Migration Plan

1. 停止当前 `docker-compose.yml` 运行的 Traefik 和 OTel Collector 容器。
2. 应用新的 `docker-compose.yml` 和 `otel-collector-config.yaml`。
3. 重新启动容器：`docker compose up -d traefik otel-collector`。
4. 验证：使用 curl 携带有效/无效 Token 分别请求 `http://localhost:4318/v1/traces`，确认无效 Token 返回 401，有效 Token 返回 200 且 Collector 输出包含 `metis.user_id` attribute。

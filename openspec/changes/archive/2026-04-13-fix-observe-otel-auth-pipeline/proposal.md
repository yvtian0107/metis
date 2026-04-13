## Why

`observe` App 的数据接入链路存在关键缺陷：Traefik 未配置 ForwardAuth 中间件，OTel Collector 未将鉴权 header 转换为 resource attributes，导致 Integration Token 的创建/撤销功能形同虚设，外部服务可无鉴权推送数据，且数据无法按用户归属隔离。此外，前端 Go SDK 引导代码使用错误的 OTLP endpoint API，会导致用户接入失败。

## What Changes

- 补全 `support-files/dev/docker-compose.yml` 中 Traefik 的 `forwardAuth` middleware 配置，将 OTLP HTTP 流量路由到 Metis `/api/v1/observe/auth/verify` 进行 Token 校验
- 补全 `support-files/dev/otel-collector-config.yaml` 中的 `transform` processor，将 `X-Metis-*` response headers 映射为 resource attributes
- 删除未使用的 `internal/app/observe/middleware.go`（`IntegrationTokenMiddleware` 已成为死代码）
- 修复 `web/src/apps/observe/data/integrations.ts` 中 Go SDK snippet 的 endpoint 初始化方式（`WithEndpoint` → `WithEndpointURL`）
- 优化前端验证命令和 `AuthHandler.Verify` 的响应体验

## Capabilities

### New Capabilities
- *(none — this is a bugfix and completion of existing `observe` capability)*

### Modified Capabilities
- *(none — requirements unchanged; this closes the gap between design intent and implementation)*

## Impact

- `support-files/dev/docker-compose.yml`
- `support-files/dev/otel-collector-config.yaml`
- `internal/app/observe/middleware.go` (removal)
- `internal/app/observe/auth_handler.go`
- `web/src/apps/observe/data/integrations.ts`
- `web/src/apps/observe/pages/integrations/[slug].tsx`
- 开发环境启动文档（需说明 Traefik ForwardAuth 依赖 Metis 服务可达）

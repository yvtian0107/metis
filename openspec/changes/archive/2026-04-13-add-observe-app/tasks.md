## 1. 后端 App 骨架

- [x] 1.1 创建 `internal/app/observe/` 目录，新建 `app.go`
- [x] 1.2 在 `cmd/server/edition_full.go` 新增 `_ "metis/internal/app/observe"` import

## 2. Token 数据模型与 Token 工具函数

- [x] 2.1 新建 `internal/app/observe/model.go`，定义 `IntegrationToken` struct（含 user_id、org_id *uint、scope、name、token_hash、token_prefix、last_used_at、revoked_at、created_at）
- [x] 2.2 新建 `internal/app/observe/token.go`，实现 `GenerateIntegrationToken()`（格式 `itk_<32hex>`）和 `ValidateIntegrationToken()` 函数

## 3. Repository 层

- [x] 3.1 新建 `internal/app/observe/repo.go`，实现 `IntegrationTokenRepo`：`Create`、`ListByUserID`（仅未撤销）、`FindByPrefix`（用于验证）、`Revoke`（写 revoked_at）、`UpdateLastUsed`

## 4. Service 层

- [x] 4.1 新建 `internal/app/observe/service.go`，实现 `IntegrationTokenService`：`Create`（含数量上限检查）、`List`、`Revoke`（含主动清除缓存）
- [x] 4.2 在 Service 中实现验证缓存逻辑：内存 map + TTL 60s，`Verify(raw string) (*VerifyResult, error)` 方法，先查缓存再 bcrypt，撤销时清除缓存条目

## 5. Handler 层

- [x] 5.1 新建 `internal/app/observe/handler.go`，实现 `IntegrationTokenHandler`：`Create`、`List`、`Revoke` 三个 HTTP handler，使用标准 `handler.OK` / `handler.Fail` 响应格式
- [x] 5.2 新建 `internal/app/observe/auth_handler.go`，实现 `AuthHandler.Verify`：从 Authorization header 提取 Token，调用 Service.Verify，成功则写入 `X-Metis-User-Id`、`X-Metis-Token-Id`、`X-Metis-Scope` response header，返回 200；失败返回 401

## 6. 中间件与路由注册

- [x] 6.1 新建 `internal/app/observe/middleware.go`，实现 `IntegrationTokenMiddleware`（模式同 `node/middleware.go`）
- [x] 6.2 在 `app.go` 的 `Routes()` 方法中注册路由

## 7. Seed 数据

- [x] 7.1 新建 `internal/app/observe/seed.go`
- [x] 7.2 在 seed 中添加 admin 角色的 Casbin 策略
- [x] 7.3 在 seed 中写入 SystemConfig key `observe.otel_endpoint`

## 8. IOC 注册

- [x] 8.1 在 `app.go` 的 `Providers()` 方法中注册所有依赖

## 9. 前端 App 骨架

- [x] 9.1 创建 `web/src/apps/observe/` 目录结构
- [x] 9.2 在 `web/src/apps/_bootstrap.ts` 中新增 `import "./observe/module"`

## 10. 集成数据定义

- [x] 10.1 在 `data/integrations.ts` 中定义 11 个集成配置

## 11. Integration Catalog 页面

- [x] 11.1 新建 `pages/integrations/index.tsx`
- [x] 11.2 新建 Integration 卡片组件
- [x] 11.3 新建 `pages/integrations/[slug].tsx`

## 12. API Tokens 管理页面

- [x] 12.1 新建 `pages/tokens/index.tsx`
- [x] 12.2 实现新建 Token Sheet
- [x] 12.3 实现撤销 Token 二次确认 Dialog

## 13. 前端 API 集成

- [x] 13.1 新增 API 调用函数
- [x] 13.2 实现配置片段动态替换逻辑

## 14. 路由注册

- [x] 14.1 在 `module.ts` 中注册路由

## 15. 构建验证

- [x] 15.1 运行 `go build -tags dev ./cmd/server/` 确认后端编译无误
- [x] 15.2 运行 `cd web && bun run lint` 确认前端 ESLint 无报错

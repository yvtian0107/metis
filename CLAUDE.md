# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.
@AGENTS.md
@openspec
## Project Overview

Metis is a Go 1.25 web application with an embedded React frontend. It compiles to a single binary that serves both API and static assets. The backend uses Gin + GORM + samber/do (IOC), the frontend uses Vite 8 + React 19 + TypeScript 6 + React Compiler.

## Build & Run Commands

```bash
make dev              # Run Go server (port 8080) with -tags dev (no frontend embed needed)
make web-dev          # Run Vite dev server (port 3000, proxies /api → :8080)
make build            # Build frontend + compile single binary (./metis)
make release          # Cross-compile for linux/darwin/windows (amd64+arm64) → dist/
make run              # build + run
cd web && bun run lint  # ESLint the frontend (includes React Compiler rules)
```

For development, run `make dev` and `make web-dev` in separate terminals. The Vite dev server at :3000 proxies `/api/*` to the Go server at :8080. On first run, open `http://localhost:3000` — the install wizard will guide you through database selection, site setup, and admin account creation.

**Go build verification**: Use `go build -tags dev ./cmd/server/` to check compilation without building the frontend (the `dev` tag provides an empty embed FS).

**Modular build** — 按需裁剪前后端模块：

```bash
make build                                      # 全功能（默认）
make build EDITION=edition_lite APPS=system      # 仅内核
make build APPS=system,ai                        # 内核 + AI（前端裁剪，后端需对应 edition）
```

| 参数 | 作用 | 默认值 |
|------|------|--------|
| `EDITION` | Go build tag，控制后端编译哪些 App | 空（全部编译） |
| `APPS` | 前端模块列表，`scripts/gen-registry.sh` 据此生成 `web/src/apps/registry.ts` | 空（全量 registry） |

**CLI subcommands**: None — all configuration is handled via the browser-based install wizard and `metis.yaml`.

**Package manager**: Frontend uses **bun** (`bun run dev`, `bun run build`). No Go tests exist yet.

## Architecture

### Kernel（内核）

**Layered structure with dependency injection (samber/do v2):**

```
cmd/server/main.go → IOC container setup → Gin engine + middleware
    ↓
internal/handler/     → HTTP handlers, route registration, unified response (R{code,message,data})
internal/service/     → Business logic, custom error types (ErrUserNotFound, etc.)
internal/repository/  → GORM data access, ListParams/ListResult for pagination
internal/model/       → Domain structs (BaseModel for common fields, SystemConfig for K/V table)
internal/config/      → metis.yaml config (MetisConfig struct, Load/Save)
internal/database/    → GORM init, SQLite + PostgreSQL support
internal/middleware/   → slog request logger, panic recovery, JWT auth, Casbin RBAC, audit logging
internal/scheduler/   → Task engine: cron scheduling + async queue, GORM-backed persistence
internal/seed/        → Install() for first-time setup, Sync() for incremental updates on restart
internal/pkg/token/   → JWT generation/validation, password hashing (bcrypt), token blacklist
internal/pkg/oauth/   → OAuth 2.0 providers (Google, GitHub), state manager for CSRF
internal/casbin/      → Casbin enforcer init with RBAC model (keyMatch2 matcher), GORM adapter
internal/channel/     → Message channels with driver pattern (email etc.)
internal/telemetry/   → OpenTelemetry tracing with OTLP HTTP exporter, config from DB SystemConfig
```

All dependencies are registered as `do.Provide()` providers in main.go and resolved lazily. The database wrapper (`database.DB`) implements `do.Shutdowner` for graceful cleanup.

### 可插拔 App 架构

```
┌──────────────────────────────────┐
│            Kernel（内核）          │  ← 用户/角色/菜单/认证/设置/任务/审计
│         始终存在，不可拔除          │     代码在 internal/ 原位不动
└──────────────┬───────────────────┘
               │
     ┌─────────┼─────────┐
     ▼         ▼         ▼
  App: AI   App: License  ...     ← 可选模块，build tag 控制
```

每个 App 实现 `app.App` 接口（`internal/app/app.go`）：

```go
type App interface {
    Name() string                                        // 唯一标识
    Models() []any                                       // GORM AutoMigrate
    Seed(db *gorm.DB, enforcer *casbin.Enforcer) error   // 菜单 + 策略
    Providers(i do.Injector)                             // IOC 注册
    Routes(api *gin.RouterGroup)                         // 路由（已带 JWT+Casbin 中间件）
    Tasks() []scheduler.TaskDef                          // 定时任务，无则返回 nil
}
```

启动时 main.go 对每个注册的 App 依次调用：`Models → Providers → Seed → Routes → Tasks`。

**新建 App 步骤**：
1. 后端：`internal/app/<name>/app.go` — 实现 App 接口 + `func init() { app.Register(&XxxApp{}) }`
2. Edition 文件：`cmd/server/edition_full.go` 加 `import _ "metis/internal/app/<name>"`
3. 前端：`web/src/apps/<name>/module.ts` — 调用 `registerApp()` 注册路由
4. 在 `web/src/apps/registry.ts` 底部加 `import './<name>/module'`

App 可通过 IOC 容器引用内核 service：`do.MustInvoke[*service.UserService](i)`

**自定义 edition**：创建 `cmd/server/edition_xxx.go`（带 `//go:build edition_xxx`），仅 import 需要的 App。

### Middleware Chain

认证路由的中间件链（顺序固定，在 `handler.Register()` 中配置）：

```
JWTAuth → PasswordExpiry → CasbinAuth → Audit → Handler
```

- **CasbinAuth 白名单**: `middleware/casbin.go` 中 `casbinWhitelist`（精确匹配）和 `casbinWhitelistPrefixes`（前缀匹配）定义了跳过权限检查的公开路由。新增需要公开访问的 API 需要加到白名单。
- **Audit**: 仅 2xx 响应会记录审计日志。Handler 通过 `c.Set()` 设置审计字段：`audit_action`, `audit_resource`, `audit_resource_id`, `audit_summary`。

### Auth & RBAC

JWTAuth middleware → extracts UserID + Role from token → CasbinAuth middleware → `enforce(roleCode, path, method)` with `keyMatch2` for wildcard paths.

**JWT tokens**: Access token 30 min, refresh token 7 days. TokenClaims 含 `purpose` 字段用于区分 2FA token。

**OAuth 2.0**: Google & GitHub social login built-in（`internal/pkg/oauth/`）。UserConnection model 支持多账号关联。

**Sessions**: 支持查看活跃会话列表、管理员踢人（通过 token blacklist）。

### Scheduler Engine

`Engine.Register(taskDef)` before `Start()`. Tasks are either "scheduled" (cron expressions) or "async" (queue-polled every 3s). Handler signature: `func(ctx context.Context, payload json.RawMessage) error`. Default timeout: 30s, retries: 3.

**内置任务**: `scheduler-history-cleanup` (30天)、`blacklist-cleanup` (每5分钟)、`expired-token-cleanup` (每日3AM)、`audit-log-cleanup` (每日3AM)。

### OpenTelemetry

通过 DB SystemConfig 配置（默认关闭）：`otel.enabled`, `otel.exporter_endpoint`, `otel.service_name`, `otel.sample_rate`。在系统设置页面修改。Trace ID/Span ID 自动注入 slog 日志。

### Frontend Stack (`web/src/`)

```
apps/             → Pluggable app modules (registry.ts + per-app module.ts)
pages/            → Kernel feature pages (users, roles, menus, tasks, settings, config, home)
stores/           → Zustand stores (auth, menu, ui) — hydrated from localStorage
components/       → shadcn/ui components + DashboardLayout
lib/api.ts        → Centralized HTTP client with auto-token refresh + 401 request queueing
hooks/            → useListPage (pagination + react-query), usePermission
```

- **State**: Zustand (auth tokens, user, menus, UI) + React Query (server state, staleTime: 30s, refetchOnWindowFocus: false)
- **Routing**: React Router 7, lazy-loaded routes, AuthGuard + PermissionGuard wrappers
- **UI**: Tailwind CSS 4, shadcn/ui, Lucide icons, DnD Kit for menu reordering
- **Forms**: React Hook Form + Zod validation
- **表单容器**: 新建/编辑表单统一使用 Sheet（抽屉），不要用 Dialog（弹窗）
- **HTTP client**: 401 时自动刷新 token，并发请求排队等待刷新完成后统一重试

## React Compiler 约束

项目启用了 React Compiler（通过 `babel-plugin-react-compiler`）和 `eslint-plugin-react-hooks` 严格规则。以下模式会导致编译错误或运行时 "Rendered fewer hooks than expected" 崩溃：

1. **禁止在 hooks 之前 early return** — 所有 `useState`, `useEffect`, `useCallback`, `useMemo` 必须在任何条件 return 之前调用
2. **禁止在 effect 中直接 setState** — `useEffect` 内不能同步调用 `setState`，需要用事件回调或拆分 effect
3. **禁止在 render 中读写 ref** — `ref.current` 不能在组件函数体中读写，只能在 event handler 或 effect 中使用
4. **禁止 IIFE** — 立即执行函数表达式会被 React Compiler 错误编译，改用普通变量赋值

**正确模式示例**：
```tsx
// ✅ 所有 hooks 在最前面，early return 在最后面
function MyComponent({ data }) {
  const [state, setState] = useState(false)
  useEffect(() => { ... }, [])
  if (!data) return null  // early return 在所有 hooks 之后
  return <div>...</div>
}
```

## Key Conventions

- **API prefix**: `/api/v1/*`
- **Response format**: `handler.OK(c, data)` / `handler.Fail(c, status, msg)` — returns `{"code":0,"message":"ok","data":...}`
- **Database**: SQLite (default, pure Go, CGO_ENABLED=0) or PostgreSQL. Default SQLite DSN 使用 `_pragma=journal_mode(WAL)` 开启 WAL 模式。Database driver is selected during the install wizard and stored in `metis.yaml`.
- **Configuration**: `metis.yaml` stores infrastructure config (db_driver, db_dsn, jwt_secret, license_key_secret). All other settings (server_port, OTel, site.name, etc.) are in DB `SystemConfig` table. No `.env` file is used.
- **Install wizard**: On first run (no `metis.yaml`), the server enters install mode and serves only `/api/v1/install/*` + SPA. The frontend at `/install` guides database selection → site info → admin account creation. After install, the process hot-switches to normal mode.
- **Seed pattern**: `seed.Install()` runs during first-time installation (full seed). `seed.Sync()` runs on every subsequent startup (incremental — adds missing roles/menus/policies only, never overwrites existing SystemConfig values).
- **New kernel models**: Add struct in `internal/model/`, register in `database.go` AutoMigrate call, create repo → service → handler. Wire into IOC in `main.go` via `do.Provide()`.
- **New app models**: 在 App 的 `Models()` 方法中返回，main.go 自动 AutoMigrate。
- **BaseModel**: Embed `model.BaseModel` for auto ID + timestamps + soft delete. SystemConfig uses Key as PK instead.
- **ToResponse pattern**: Models expose `.ToResponse()` methods to strip sensitive fields (e.g., User hides password hash).
- **Pagination**: Backend `ListParams{Keyword, IsActive, Page, PageSize}` → `ListResult{Items, Total}`. Frontend `useListPage` hook wraps this with React Query.
- **Error handling**: Services 定义包级 sentinel error（如 `ErrProductNotFound`），Handler 用 `errors.Is()` 匹配并转换为 HTTP status code。
- **Static embedding**: `embed.go` at project root embeds `web/dist/` via `//go:embed all:web/dist`. `embed_dev.go`（`//go:build dev`）提供空 FS 用于开发模式（`make dev` 不需要先构建前端）。SPA fallback serves `index.html` for non-API, non-file routes.
- **Frontend path alias**: `@/` maps to `web/src/` in both Vite and TypeScript configs.
- **Route registration**: Handler 的 `Register()` 返回已带 JWT+Casbin+Audit 中间件的 `*gin.RouterGroup`，App routes 挂载在此 group 下。
- **Seed pattern**: 种子数据用 `db.Where("permission = ?", x).First(&existing)` 做幂等检查，只在记录不存在时创建。Casbin 策略用 `enforcer.HasPolicy()` 检查。Install() 为首次安装全量种子，Sync() 为后续启动增量同步。

## Do Not Modify

- `refer/` and `support-files/refer/` — user's reference code, never modify
- `next-app/` — separate Next.js experiment, not part of the main app
- `openspec/` — spec-driven development artifacts, managed via `/opsx:*` commands

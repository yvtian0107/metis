# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.
@AGENTS.md
@openspec
## Project Overview

Metis is a Go 1.26 web application with an embedded React frontend. It compiles to a single binary that serves both API and static assets. The backend uses Gin + GORM + samber/do (IOC), the frontend uses Vite 8 + React 19 + TypeScript 6 + React Compiler.

## Build & Run Commands

```bash
make dev              # Run Go server (port 8080) with -tags dev (no frontend embed needed)
make web-dev          # Run Vite dev server (port 3000, proxies /api → :8080)
make build            # Build frontend + compile single binary (./server)
make run              # build + run
make release          # Cross-compile for linux/darwin/windows (amd64+arm64) → dist/

# Sidecar (separate binary for remote agent execution)
make build-sidecar    # Build sidecar binary (./sidecar)
make release-sidecar  # Cross-compile sidecar → dist/

# License edition
make build-license    # Build license edition binary (./license)
make release-license  # Cross-compile license edition → dist/

# Frontend
make web-build        # Build frontend for production
cd web && bun run lint     # ESLint (includes React Compiler rules)
cd web && bun run preview  # Preview production build locally
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

**CLI subcommands**: None — all configuration is handled via the browser-based install wizard and `config.yml`.

**Package manager**: Frontend uses **bun** (`bun run dev`, `bun run build`).

**Tests**: Run Go tests with `go test ./...`. There is currently one test file (`internal/app/ai/data_stream_test.go`); no frontend tests exist yet.

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
internal/config/      → config.yml config (MetisConfig struct, Load/Save)
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
  ┌────┬────┬──┴──┬────┬────┐
  ▼    ▼    ▼     ▼    ▼    ▼
 AI  Node  Org   APM  Obs  License   ← 可选模块，build tag 控制
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
4. 在 `web/src/apps/_bootstrap.ts` 加 `import './<name>/module'`（`gen-registry.sh` 在过滤构建时自动管理此文件）

App 可通过 IOC 容器引用内核 service：`do.MustInvoke[*service.UserService](i)`

**自定义 edition**：创建 `cmd/server/edition_xxx.go`（带 `//go:build edition_xxx`），仅 import 需要的 App。

**现有 edition**：

| 文件 | Build tag | 内容 |
|------|-----------|------|
| `edition_full.go` | 无（默认） | 全部 App（build constraint: `!(edition_lite \|\| edition_license)`） |
| `edition_lite.go` | `edition_lite` | 仅内核，无 App |
| `edition_license.go` | `edition_license` | 内核 + license App |

### Version Injection

Makefile 通过 `-ldflags` 注入 `internal/version` 包的三个变量：`Version`（git tag 或 `nightly-YYYYMMDD-<hash>`）、`GitCommit`、`BuildTime`。`make dev` 同样注入 ldflags。

### Middleware Chain

认证路由的中间件链（顺序固定，在 `handler.Register()` 中配置）：

```
JWTAuth → PasswordExpiry → CasbinAuth → DataScope → Audit → Handler
```

- **CasbinAuth 白名单**: `middleware/casbin.go` 中 `casbinWhitelist`（精确匹配）和 `casbinWhitelistPrefixes`（前缀匹配）定义了跳过权限检查的公开路由。新增需要公开访问的 API 需要加到白名单。
- **Audit**: 仅 2xx 响应会记录审计日志。Handler 通过 `c.Set()` 设置审计字段：`audit_action`, `audit_resource`, `audit_resource_id`, `audit_summary`。
- **DataScope**: 基于角色的数据可见性过滤（`middleware/data_scope.go`）。通过 `c.Get("deptScope")` 获取 `*[]uint`：nil=全部可见，`&[]uint{}`=仅自己，`&[]uint{1,2,3}`=指定部门。依赖 Org App 的 `OrgScopeResolver`，未安装 Org 时不过滤。

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

### AI App: Knowledge Module

Knowledge 模块在 `internal/app/ai/` 中，是 AI App 的子系统，架构分两阶段：

**Pipeline**: 添加来源 → 异步提取文本 → 异步 LLM 编译 → 知识图谱

```
KnowledgeBase         → 知识库（compile status: idle/compiling/completed/error）
  └─ KnowledgeSource  → 来源（文件/URL/文本，extract status: pending/completed/error）
  └─ KnowledgeNode    → 知识图谱节点（concept/index 两种类型）
  └─ KnowledgeEdge    → 节点间关系（related/contradicts/extends/part_of）
  └─ KnowledgeLog     → 编译日志
```

**Scheduler 任务**（注册在 `AIApp.Tasks()`）：
- `ai-source-extract`（async）— 提取文件/URL 文本内容，md/txt 在上传时直接提取，PDF/DOCX/XLSX/PPTX 目前为 TODO（会返回 error）
- `ai-knowledge-crawl`（cron `*/5 * * * *`）— 遍历开启爬取的 URL 来源，按 `CrawlSchedule` 决定是否重新爬取，内容变化时触发重新编译
- `ai-knowledge-compile`（async）— 调用 LLM 将全部 `completed` 来源编译为知识图谱，编译后自动生成 `index` 节点并执行 lint

**LLM 编译**（`KnowledgeCompileService`）：使用知识库配置的模型（未配置则取默认 LLM），构造 prompt → 调用 `internal/llm.Client` → 解析 JSON 输出（nodes + updated_nodes）→ 写入 node/edge。`internal/llm/` 是共享的 LLM 客户端包，支持 OpenAI-compatible 和 Anthropic 两种协议，通过 Provider 的 protocol + BaseURL + 加密 API Key 构建客户端。

**URL 爬取**：支持 `crawlDepth`（递归抓取同域链接）和 `urlPattern`（前缀过滤子链接）。HTML 内容用简单正则转换为 Markdown，10MB 大小限制。

### AI App: Agent Runtime

Agent Runtime 是 AI App 的子系统，支持多轮对话会话和多种执行策略：

**核心模型**：
```
Agent                 → 智能体配置（类型、策略、模型、工具绑定）
  ├─ AgentTypeAssistant  → 通用助手（ReAct/Plan-and-Execute 策略）
  ├─ AgentTypeCoding     → 编码助手（本地或远程执行）
  └─ AgentTemplate       → 可复用的模板配置
AgentSession          → 会话（多轮消息流）
  └─ SessionMessage     → 消息（user/assistant/tool 角色）
AgentMemory           → 长期记忆（关键信息提取存储）
```

**执行策略**：
- **ReAct** (`AgentStrategyReact`) — 思考-行动-观察循环，适合多步推理
- **Plan-and-Execute** (`AgentStrategyPlanAndExecute`) — 先规划再执行，适合复杂任务

**编码执行模式** (`AgentTypeCoding`)：
- **Local** — 本地执行（通过 `executor_coding_local.go` 直接调用 CLI 工具）
- **Remote** — 远程执行（通过 Node/Sidecar 在远程节点上执行）

**工具系统**：
- **Tool Registry** — 内置工具注册表
- **MCP Server** — Model Context Protocol 服务器支持
- **Skill** — Git 仓库形式的技能包，可导入并绑定到 Agent

**会话 API**：
- `POST /api/v1/ai/sessions/:sid/messages` — 发送消息
- `GET /api/v1/ai/sessions/:sid/stream` — SSE 流式响应
- `POST /api/v1/ai/sessions/:sid/cancel` — 取消正在执行的会话

### Node App: Sidecar Architecture

Node App 提供远程节点管理，支持 Agent 的远程代码执行：

```
Node                  → 工作节点（注册 token、标签、资源限制）
  ├─ NodeProcess      → 绑定的进程实例
  └─ ProcessDef       → 进程定义（镜像、配置模板）
NodeCommand           → 下发给节点的命令（启动/停止/重启/重载）
NodeProcessLog        → 进程日志收集
```

**通信协议**：
- Sidecar 在远程节点运行（`cmd/sidecar` 编译的独立二进制）
- 长连接 SSE `/api/v1/nodes/sidecar/stream` — 服务器→节点命令推送
- 轮询 `GET /api/v1/nodes/sidecar/commands` — 节点获取待处理命令
- REST API — 心跳、日志上传、配置下载

**认证**：Node Token（通过 `X-Node-Token` header），不同于用户 JWT。

**Scheduler 任务**：
- `node-offline-detection` — 检测心跳超时的离线节点
- `node-command-cleanup` — 清理过期未确认命令
- `node-log-cleanup` — 清理历史进程日志

### Agent & Knowledge 的 Node Token API

以下 API 使用 Node Token 鉴权（非 JWT+Casbin），供 Sidecar 调用：

```
/api/v1/ai/knowledge/*       → 知识查询（搜索、节点、图谱）
/api/v1/ai/internal/skills/* → Skill 包下载
```

### Org App: Organization Structure

组织架构管理，为数据权限过滤提供基础：

```
Department        → 部门树（parent_id 自关联，支持树形查询）
Position          → 职位定义
UserPosition      → 用户-职位分配（多对多，支持主岗/兼岗）
```

- 提供 `OrgScopeResolver` 接口实现 — 供 `DataScopeMiddleware` 查询用户可见部门范围
- API 前缀：`/api/v1/org/*`（部门 CRUD + 树、职位 CRUD、用户职位分配）

### APM App: Application Performance Monitoring

基于 ClickHouse 的 APM 应用，查询 OpenTelemetry trace/span 数据：

- 无自有 Model（`Models()` 返回 nil），数据来自外部 ClickHouse
- `NewClickHouseClient` 从 SystemConfig 读取 ClickHouse 连接配置
- API 前缀：`/api/v1/apm/*`（traces、services、topology、timeseries、analytics、errors）

### Observe App: Integration Token Authentication

为外部可观测性工具（如 Grafana）提供 ForwardAuth 验证：

- `IntegrationToken` — 集成令牌（name + hashed token + scopes）
- `/api/v1/observe/auth/verify` — ForwardAuth 端点，绕过 JWT+Casbin，直接注册在 Gin Engine 上
- API 前缀：`/api/v1/observe/*`（令牌 CRUD + 设置）

### License App

许可证管理模块（独立 edition `edition_license` 可单独编译）。

### i18n 国际化

**后端**：`internal/locales/` 使用 go-i18n，内核内嵌 `zh-CN.json` + `en.json`。App 可实现 `LocaleProvider` 接口提供额外翻译文件，main.go 在启动时自动加载。通过 `localeSvc.T(locale, messageID)` 获取翻译。

**前端**：`web/src/i18n/` 使用 i18next + react-i18next。内核翻译按 namespace 拆分在 `web/src/i18n/locales/{zh-CN,en}/` 下。App 翻译在 `web/src/apps/<name>/locales/`，通过 `registerTranslations(ns, resources)` 在 `module.ts` 中注册。支持 zh-CN 和 en，fallback 为 zh-CN。

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
- **Database**: SQLite (default, pure Go, CGO_ENABLED=0) or PostgreSQL. Default SQLite DSN 使用 `_pragma=journal_mode(WAL)` 开启 WAL 模式。Database driver is selected during the install wizard and stored in `config.yml`.
- **Configuration**: `config.yml` stores infrastructure config (db_driver, db_dsn, secret_key, jwt_secret, license_key_secret). All other settings (server_port, OTel, site.name, etc.) are in DB `SystemConfig` table. No `.env` file is used.
- **Install wizard**: On first run (no `config.yml`), the server enters install mode and serves only `/api/v1/install/*` + SPA. The frontend at `/install` guides database selection → site info → admin account creation. After install, the process hot-switches to normal mode.
- **Seed pattern**: `seed.Install()` runs during first-time installation (full seed). `seed.Sync()` runs on every subsequent startup (incremental — adds missing roles/menus/policies only, never overwrites existing SystemConfig values). 种子数据用 `db.Where("permission = ?", x).First(&existing)` 做幂等检查，只在记录不存在时创建。Casbin 策略用 `enforcer.HasPolicy()` 检查。
- **New kernel models**: Add struct in `internal/model/`, register in `database.go` AutoMigrate call, create repo → service → handler. Wire into IOC in `main.go` via `do.Provide()`.
- **New app models**: 在 App 的 `Models()` 方法中返回，main.go 自动 AutoMigrate。
- **BaseModel**: Embed `model.BaseModel` for auto ID + timestamps + soft delete. SystemConfig uses Key as PK instead.
- **ToResponse pattern**: Models expose `.ToResponse()` methods to strip sensitive fields (e.g., User hides password hash).
- **Pagination**: Backend `ListParams{Keyword, IsActive, Page, PageSize}` → `ListResult{Items, Total}`. Frontend `useListPage` hook wraps this with React Query.
- **Error handling**: Services 定义包级 sentinel error（如 `ErrProductNotFound`），Handler 用 `errors.Is()` 匹配并转换为 HTTP status code。
- **Static embedding**: `embed.go` at project root embeds `web/dist/` via `//go:embed all:web/dist`. `embed_dev.go`（`//go:build dev`）提供空 FS 用于开发模式（`make dev` 不需要先构建前端）。SPA fallback serves `index.html` for non-API, non-file routes.
- **Frontend path alias**: `@/` maps to `web/src/` in both Vite and TypeScript configs.
- **Route registration**: Handler 的 `Register()` 返回已带 JWT+Casbin+Audit 中间件的 `*gin.RouterGroup`，App routes 挂载在此 group 下。

## Do Not Modify

- `refer/` and `support-files/refer/` — user's reference code, never modify
- `next-app/` — separate Next.js experiment, not part of the main app
- `openspec/` — spec-driven development artifacts, managed via `/opsx:*` commands

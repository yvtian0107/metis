# Repository Instructions

## Working Style

- 默认用中文与用户交流。
- `AGENTS.md` 是指向 `CLAUDE.md` 的软链接；更新说明文件时改 `CLAUDE.md`，不要删除或重建这个链接。
- `DESIGN.md` 是前端设计沉淀文档，记录已经落地并验证过的页面结构、视觉语言、交互规则和不再使用的旧模式。做 UI 相关改动时，先参考它；当设计方向或已实现模式发生稳定变化时，同步更新它。
- `openspec/` 是规格工件；`support-files/refer/` 是参考代码。除非用户明确要求，否则不要修改这两处。

## Commands

- 前端只用 Bun，目录在 `web/`；安装依赖用 `make web-install`，不要改用 `npm`/`pnpm`/`yarn`。
- 本地开发要开两个进程：`make dev` 启动 Go 服务（`:8080`，`-tags dev`），`make web-dev` 启动 Vite（`:3000`，代理 `/api` 到 `:8080`）。
- 后端快速编译验证用 `go build -tags dev ./cmd/server`；`make build` 会先构建 `web/dist`，再产出嵌入前端的 `./server`。
- Sidecar 是独立二进制，入口在 `cmd/sidecar`；验证构建用 `make build-sidecar`。
- 前端验证用 `cd web && bun run lint` 和 `cd web && bun run build`；仓库当前没有前端测试。
- 常用测试：`make test`、`make test-license`、`make test-bdd`、`make test-llm`。`make test-llm` 依赖根目录 `.env.test`，模板在 `.env.test.example`。

## Architecture

- 这是单仓库双端项目：后端入口在 `cmd/server/main.go`，前端入口在 `web/src/main.tsx`；生产态由 Go 二进制嵌入 `web/dist`，开发态 `embed_dev.go` 提供空 FS，所以 `make dev` 本身不会服务前端资源。
- 可插拔 App 的真实接口以 `internal/app/app.go` 为准，不要照抄 `internal/app/README.md` 里的旧签名；当前接口是 `Seed(db, enforcer, install bool)`。
- 新增 App 时至少同步三处：`internal/app/<name>/`、`cmd/server/edition_*.go`、`web/src/apps/<name>/module.ts`。前端路由还依赖 `web/src/apps/_bootstrap.ts` 的副作用导入。
- `handler.Register()` 返回的 `*gin.RouterGroup` 已经挂好 `JWT -> PasswordExpiry -> Casbin -> DataScope -> Audit`；App 路由默认都走这条链。
- `cmd/server/edition_full.go` 的空白导入顺序会影响 `init() -> app.Register()` 顺序，也会影响启动时的 `Models -> Providers -> Seed -> Routes -> Tasks` 顺序；有跨 App 依赖时不要随意改导入顺序。

## Gotchas

- `scripts/gen-registry.sh` 会重写 `web/src/apps/_bootstrap.ts`；过滤构建时还会重写 `web/tsconfig.app.json`。新增或删除前端 App 时，要同步更新脚本里的 `ALL_APPS`。如果过滤构建被中断，用 `make web-full-registry` 恢复全量状态。
- 后端响应格式统一是 `R{code,message,data}`，不是裸 JSON；前端优先复用 `web/src/lib/api.ts`，它已经处理了 token 刷新和并发 401 排队。
- 运行时配置不走 `.env`：基础密钥和数据库在 `config.yml`，很多运行参数（如 `server_port`、OTel）在数据库 `SystemConfig`；只有 LLM 测试读取 `.env.test`。
- 需要“登录即可、无需细粒度权限”的 API 时，除了注册路由，还要更新 `internal/middleware/casbin.go` 的 `casbinWhitelist` 或 `casbinWhitelistPrefixes`；否则会被 Casbin 拦截。
- React Compiler 已开启（见 `web/vite.config.ts` 的 `reactCompilerPreset()`）；前端改动避免在所有 hooks 之前提前 `return`、避免在 effect 里同步 `setState`、避免在 render 里读写 `ref.current`。
- 前端 UI 约定见 `DESIGN.md`；它既包含通用约定，也包含像【供应商管理】这种模块的当前实现基线。最常用的一条是新建/编辑表单优先用 `Sheet`，不要用 `Dialog`。

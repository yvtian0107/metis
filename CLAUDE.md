# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

# Repository Instructions

- 默认用中文与用户交流。
- `AGENTS.md` 是指向 `CLAUDE.md` 的软链接；更新说明只改 `CLAUDE.md`，不要删除或重建链接。
- `openspec/` 是规格工件，`support-files/refer/` 是参考代码；除非用户明确要求，否则不要修改。
- 做 UI 改动先读 `DESIGN.md`；当前基线是安静工作台，CRUD 新建/编辑优先 `Sheet`，不要改成 `Dialog`。

## Commands

- 前端包管理只用 Bun。`web/` 安装依赖用 `make web-install`，不要改用 `npm`/`pnpm`/`yarn`。
- 联调优先 `make dev`：会先恢复完整前端 registry；若根目录存在 `.env.dev`，会先执行 `make seed-dev`，再由 `cmd/dev` 自动找空闲 `3000+/8000+` 端口并同时启动 Go 服务和 Vite，把 `/api` 代理到本次后端端口。
- 只跑前端用 `make web-dev`；默认代理 `http://localhost:8080`，需要别的后端时用 `VITE_API_TARGET=http://localhost:<port> make web-dev`。
- 常用验证：后端 `go build -tags dev ./cmd/server`、`go test ./...`、`go test ./path/to/pkg -run TestName`；前端 `cd web && bun run lint && bun run build`；Sidecar `make build-sidecar`。
- 前端测试走 Bun，例如 `cd web && bun test src/components/chat-workspace/message-timeline.test.ts`。`web/tsconfig.app.json` 默认排除 `src/**/*.test.*`，所以 `bun run build` 不会检查测试文件。
- 需要 edition / 模块过滤构建时只用 `make web-build` 或 `APPS=... ./scripts/gen-registry.sh` 这套流程；脚本会改写 `web/src/apps/_bootstrap.ts` 和 `web/tsconfig.app.json`，异常残留用 `make web-full-registry` 恢复。
- `make test-llm`、`make test-bdd`、`make test-bdd-vpn` 都要求根目录 `.env.test`。

## Architecture

- 单仓库：后端入口 `cmd/server/main.go`，开发编排入口 `cmd/dev/main.go`，前端入口 `web/src/main.tsx`。生产构建把 `web/dist` 嵌进 Go 二进制；`-tags dev` 下前端不嵌入，走 Vite。Sidecar 是独立进程，入口 `cmd/sidecar/main.go`。
- 后端使用 GORM（SQLite）、Gin、Casbin（RBAC）、samber/do v2（依赖注入容器）。
- 可插拔 App 的真实接口以 `internal/app/app.go` 为准，当前 `Seed` 签名是 `Seed(db, enforcer, install bool)`；`internal/app/README.md` 有旧说明时不要信。App 可以实现可选接口（`LocaleProvider`、`OrgResolver`、`AIAgentProvider`、`ToolRegistryProvider` 等），消费方需判空处理。
- 新增 App 至少同步：`internal/app/<name>/`、对应 `cmd/server/edition_*.go` 的空白导入、`web/src/apps/<name>/module.ts`，以及 `scripts/gen-registry.sh` 的 `ALL_APPS`。前端是否生效还取决于 `web/src/apps/_bootstrap.ts` 的副作用导入。
- `cmd/server/edition_full.go` 的空白导入顺序决定 `app.Register()` 顺序，分三层：Tier 0（org/node/apm/observe/license）→ Tier 1（ai）→ Tier 2（itsm）；启动时按 `Models -> Providers -> Seed -> Routes -> Tasks` 装配；有跨 App 依赖时不要随意重排。
- `handler.Register()` 返回的 `/api/v1` 鉴权链固定为 `JWT -> PasswordExpiry -> Casbin -> DataScope -> Audit`；App 路由默认挂在这条链后面。

## Gotchas

- 后端统一响应是 `R{code,message,data}`，不是裸 JSON；前端优先复用 `web/src/lib/api.ts`，里面已经处理 token 刷新、并发 401 排队和 `FormData` 上传。
- 某个 API 若只要求“已登录”不要求细粒度权限，除了注册路由，还要同步更新 `internal/middleware/casbin.go` 的白名单、前缀白名单或 `KeyMatch` 规则，否则会被 Casbin 拦住。
- 运行时配置主要来自 `config.yml` 和数据库里的 `SystemConfig`，不是通用 `.env`；`.env.dev` 只给开发 bootstrap，`.env.test` 只给测试。
- React Compiler 已在 `web/vite.config.ts` 开启；前端改动避免破坏 hooks 顺序、在 effect 里同步 `setState`、或在 render 中读写 `ref.current`。

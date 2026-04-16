# Metis Agent Notes

Use this file for quick repo-specific guidance. `CLAUDE.md` has the long-form architecture reference.

## Working Areas

- Main product is the root Go server plus `web/`.
- `doc-website/` is a separate docs site.
- Do not modify `refer/`, `support-files/refer/`, `next-app/`, or `openspec/`.

## Commands That Matter

- Frontend package manager is `bun`, not npm/pnpm.
- First-time frontend setup: `make web-install`
- Local dev uses two terminals: `make dev` and `make web-dev`
- Full build: `make build`
- Fast backend compile check without rebuilding frontend assets: `go build -tags dev ./cmd/server`
- Backend tests: `go test ./...`
- Single Go test: `go test ./internal/app/ai -run TestName -v`
- Frontend checks: `cd web && bun run lint` and `cd web && bun run build`
- There are currently no frontend tests; lint + build are the real frontend verification steps.

## Build And Edition Gotchas

- `make build` always builds the frontend first, then embeds `web/dist` into the Go binary.
- Filtered frontend builds use `APPS=...` and rewrite `web/src/apps/_bootstrap.ts` via `scripts/gen-registry.sh`.
- If you run a filtered build, restore the full frontend registry by rerunning the generator with empty `APPS` or by letting the Make target clean up after itself.
- Backend app inclusion is controlled separately by blank imports in `cmd/server/edition_*.go`.
- Current default edition is `cmd/server/edition_full.go`; it imports `ai`, `apm`, `itsm`, `license`, `node`, `observe`, and `org`.

## Entrypoints And Wiring

- Server entrypoint is `cmd/server/main.go`.
- Missing `config.yml` puts the server into install mode; only install APIs and the SPA are available until setup completes.
- Normal startup order for each optional app is `Models -> Providers -> Seed -> Routes -> Tasks`.
- Kernel IOC providers live in `cmd/server/providers.go`; keep `registerKernelProviders` and `overrideKernelProviders` in sync.
- Optional backend apps live under `internal/app/<name>` and must register themselves with `func init() { app.Register(...) }`.
- Optional frontend apps live under `web/src/apps/<name>/module.ts` and register through `registerApp()` side effects.

## Conventions Easy To Miss

- Authenticated route chain is fixed: `JWTAuth -> PasswordExpiry -> CasbinAuth -> DataScope -> Audit -> handler`.
- New public APIs usually also need a Casbin whitelist update in `internal/middleware/casbin.go`.
- API responses should go through `handler.OK` / `handler.Fail`.
- Services use sentinel errors; handlers translate them to HTTP status codes with `errors.Is`.
- Runtime infra secrets live in `config.yml`; most other settings live in DB `SystemConfig`. There is no `.env` workflow.
- New/edit forms in the frontend should use `Sheet`, not `Dialog`.

## React Frontend Constraints

- React Compiler is enabled in Vite via Babel; avoid patterns it rejects.
- Do not return early before hooks.
- Do not call `setState` synchronously inside `useEffect`.
- Do not read or write `ref.current` during render.
- Do not use IIFEs in component render logic.

## Verification Heuristics

- Backend-only changes: usually `go test ./...` plus `go build -tags dev ./cmd/server`.
- Frontend-only changes: usually `cd web && bun run lint` plus `cd web && bun run build`.
- Cross-stack changes: run both sets.

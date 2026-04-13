## 1. Infrastructure Configuration

- [x] 1.1 Update `support-files/dev/docker-compose.yml`: add Traefik `forwardAuth` middleware labels for `otlp-http` router, pointing to `http://host.docker.internal:8080/api/v1/observe/auth/verify`, with `authResponseHeaders` for `X-Metis-User-Id`, `X-Metis-Token-Id`, `X-Metis-Scope`, `X-Metis-Org-Id`
- [x] 1.2 Update `support-files/dev/otel-collector-config.yaml`: add `transform/metis` processor to map `X-Metis-*` request headers to resource attributes (`metis.user_id`, `metis.token_id`, `metis.scope`), and include it in traces/metrics/logs pipelines

## 2. Backend Cleanup & Fixes

- [x] 2.1 Delete unused `internal/app/observe/middleware.go` (`IntegrationTokenMiddleware` is dead code)
- [x] 2.2 Update `internal/app/observe/auth_handler.go`: return JSON response `{"ok":true}` on successful verification instead of empty body

## 3. Frontend Fixes

- [x] 3.1 Fix `web/src/apps/observe/data/integrations.ts` Go SDK snippet: replace `otlptracehttp.WithEndpoint("{{ENDPOINT}}")` with `otlptracehttp.WithEndpointURL("{{ENDPOINT}}/v1/traces")`
- [x] 3.2 Update `web/src/apps/observe/pages/integrations/[slug].tsx` verification command: change from `curl -I` to a more meaningful probe or add a disclaimer about what `-I` tests

## 4. Verification

- [x] 4.1 Run `go build -tags dev ./cmd/server/` and confirm no compilation errors
- [x] 4.2 Run `cd web && bun run lint` and confirm no ESLint errors (no new errors introduced in modified files)
- [x] 4.3 (Optional manual test) Start docker-compose services and verify that an invalid Token returns 401 at `localhost:4318/v1/traces`

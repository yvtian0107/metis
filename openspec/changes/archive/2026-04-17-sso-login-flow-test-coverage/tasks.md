## 1. Refactor SSOHandler for Injection

- [x] 1.1 Define internal `oidcProvider` interface in `internal/handler/sso.go`
- [x] 1.2 Add `getOIDCProvider`, `provisionExternalUser`, `generateTokenPair` fields to `SSOHandler`
- [x] 1.3 Add `resolveOIDCProvider`, `resolveProvisionExternalUser`, `resolveGenerateTokenPair` helper methods with nil-fallback to real implementations
- [x] 1.4 Update `InitiateSSO` to use `resolveOIDCProvider`
- [x] 1.5 Update `SSOCallback` to use `resolveOIDCProvider`, `resolveProvisionExternalUser`, and `resolveGenerateTokenPair`
- [x] 1.6 Run `go build -tags dev ./cmd/server/` and fix any compile errors

## 2. SSO State Manager Tests

- [x] 2.1 Create `internal/pkg/identity/sso_state_test.go`
- [x] 2.2 Add test: `Generate` returns non-empty state string
- [x] 2.3 Add test: `Validate` returns correct `SourceID` and `CodeVerifier`
- [x] 2.4 Add test: validating the same state twice returns error
- [x] 2.5 Add test: expired state returns error (inject `nowFn` or use short TTL if refactored)
- [x] 2.6 Run `go test ./internal/pkg/identity/...` and ensure all pass

## 3. CheckDomain Endpoint Tests

- [x] 3.1 Create `internal/handler/sso_test.go` with test DB helper and `newSSOHandlerForTest` constructor
- [x] 3.2 Add test: `CheckDomain` returns 200 with source info for a bound domain
- [x] 3.3 Add test: `CheckDomain` returns 400 when `email` query param is missing
- [x] 3.4 Add test: `CheckDomain` returns 400 for invalid email format
- [x] 3.5 Add test: `CheckDomain` returns 404 when domain has no identity source

## 4. InitiateSSO Endpoint Tests

- [x] 4.1 Add test: `InitiateSSO` returns 200 with `authUrl` and `state` for enabled OIDC source
- [x] 4.2 Add test: `InitiateSSO` returns 400 for invalid `:id` parameter
- [x] 4.3 Add test: `InitiateSSO` returns 404 when source does not exist
- [x] 4.4 Add test: `InitiateSSO` returns 400 when source is disabled
- [x] 4.5 Add test: `InitiateSSO` returns 400 when source type is not `oidc`

## 5. SSOCallback Endpoint Tests

- [x] 5.1 Add test: `SSOCallback` returns 200 with token pair for successful new-user JIT provision
- [x] 5.2 Add test: `SSOCallback` returns 200 for existing connection user
- [x] 5.3 Add test: `SSOCallback` returns 400 when request body lacks `code` or `state`
- [x] 5.4 Add test: `SSOCallback` returns 400 for invalid/expired state
- [x] 5.5 Add test: `SSOCallback` returns 404 when identity source not found
- [x] 5.6 Add test: `SSOCallback` returns 400 when source type is not `oidc`
- [x] 5.7 Add test: `SSOCallback` returns 502 when `getOIDCProvider` fails (discovery error)
- [x] 5.8 Add test: `SSOCallback` returns 502 when `ExchangeCode` fails
- [x] 5.9 Add test: `SSOCallback` returns 502 when `VerifyIDToken` fails
- [x] 5.10 Add test: `SSOCallback` returns 409 when `ProvisionExternalUser` returns `ErrEmailConflict`
- [x] 5.11 Add test: `SSOCallback` returns 500 for other `ProvisionExternalUser` errors

## 6. Verification & Regression Check

- [x] 6.1 Run `go test ./internal/handler/... ./internal/pkg/identity/...` and ensure all tests pass
- [x] 6.2 Run `go build -tags dev ./cmd/server/` to confirm no compilation regressions
- [x] 6.3 Run `go test ./...` to ensure no other tests are broken by the handler refactor

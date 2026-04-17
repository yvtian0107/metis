## Why

The SSO login flow (`GET /api/v1/auth/check-domain`, `GET /api/v1/auth/sso/:id/authorize`, `POST /api/v1/auth/sso/callback`) is the critical user-facing path for external identity authentication. Despite being production code, it currently has **zero automated test coverage**. The handler directly calls external network dependencies (`identity.GetOIDCProvider`, `AuthService.ProvisionExternalUser`, `AuthService.GenerateTokenPair`) which makes it impossible to test without refactoring.

Adding tests now is essential to prevent regressions in OIDC discovery, PKCE state management, JIT user provisioning, and token generation.

## What Changes

- Refactor `SSOHandler` to accept injectable function fields for `getOIDCProvider`, `provisionExternalUser`, and `generateTokenPair`, with nil-safe fallbacks to real implementations.
- Define an internal `oidcProvider` interface so `identity.OIDCProvider` can be mocked in tests.
- Add unit tests for `internal/pkg/identity/sso_state.go` (Generate, Validate, expiry, reuse).
- Add handler integration tests for `CheckDomain` (success, missing email, invalid format, not found).
- Add handler integration tests for `InitiateSSO` (success, invalid ID, disabled source, non-OIDC type).
- Add handler integration tests for `SSOCallback` (success for new/existing users, invalid state, OIDC errors, email conflict, provisioning failures).

## Capabilities

### New Capabilities
- `sso-login-flow-test-coverage`: Comprehensive unit and integration test coverage for the SSO login flow and its underlying state manager.

### Modified Capabilities
- *(none — this change only adds tests and minimal internal refactor for testability)*

## Impact

- `internal/handler/sso.go` — add injectable fields and fallback helpers
- `internal/handler/handler.go` — no behavior change (SSOHandler instantiated with nil injectables)
- `internal/pkg/identity/sso_state_test.go` (new)
- `internal/handler/sso_test.go` (new)
- No API or frontend changes

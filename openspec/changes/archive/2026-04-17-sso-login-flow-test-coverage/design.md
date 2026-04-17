## 1. Refactoring SSOHandler for Testability

### 1.1 Problem
`SSOHandler.InitiateSSO` and `SSOHandler.SSOCallback` call external dependencies directly:
- `identity.GetOIDCProvider()` — performs live HTTP discovery via `go-oidc`
- `h.authSvc.ProvisionExternalUser()` — spans multiple repositories (user, role, connection)
- `h.authSvc.GenerateTokenPair()` — requires refresh-token repo, JWT secret, blacklist

This makes handler tests heavy and non-deterministic.

### 1.2 Solution: Function Injection with Nil Fallbacks

Add three function fields to `SSOHandler`:

```go
type SSOHandler struct {
    svc                   *service.IdentitySourceService
    authSvc               *service.AuthService
    stateMgr              *identity.SSOStateManager

    getOIDCProvider       func(ctx context.Context, sourceID uint, cfg *model.OIDCConfig) (oidcProvider, error)
    provisionExternalUser func(params service.ExternalUserParams) (*model.User, error)
    generateTokenPair     func(user *model.User, ip, ua string) (*service.TokenPair, error)
}
```

Introduce an internal interface for the subset of `identity.OIDCProvider` methods used by the handler:

```go
type oidcProvider interface {
    AuthURL(state string, pkce *identity.PKCEParams) string
    ExchangeCode(ctx context.Context, code string, codeVerifier string) (*oauth2.Token, error)
    VerifyIDToken(ctx context.Context, token *oauth2.Token) (*gooidc.IDToken, error)
}
```

Add nil-safe resolver helpers:

```go
func (h *SSOHandler) resolveOIDCProvider(...) (oidcProvider, error) {
    if h.getOIDCProvider != nil { return h.getOIDCProvider(...) }
    return identity.GetOIDCProvider(...)
}
```

Production code in `handler.go` instantiates `SSOHandler` with these fields nil, preserving existing behavior.

## 2. Testing Strategy

### 2.1 SSO State Manager (`internal/pkg/identity/sso_state_test.go`)
Tests for the in-memory TTL state store:
- `Generate` returns a non-empty string
- `Validate` returns correct `SourceID` and `CodeVerifier`
- Re-validating the same state returns "invalid or expired state"
- Expired states are rejected (inject `nowFn` into `SSOStateManager` if needed to avoid real sleeps)

### 2.2 CheckDomain
Lightweight endpoint tests using a real `IdentitySourceService` + SQLite:
- Success: email maps to an enabled source → 200 with `id`, `name`, `type`, `forceSso`
- BadRequest: missing `email` query param
- BadRequest: invalid email format (`"not-an-email"`)
- NotFound: domain has no bound identity source

### 2.3 InitiateSSO
Tests the OIDC authorization URL generation path. Use a stub `getOIDCProvider` that returns a mock `oidcProvider`:
- Success: enabled OIDC source → 200 with `authUrl` and `state`, and `authUrl` contains the state/PKCE params
- BadRequest: invalid `:id` param
- NotFound: source does not exist
- BadRequest: source is disabled
- BadRequest: source type is not `oidc`

### 2.4 SSOCallback
Tests the full callback orchestration. Stub all three external dependencies:
- **Success (new user)**: valid state/code → mock provider exchanges code and verifies ID token → stub `ProvisionExternalUser` returns a user → stub `GenerateTokenPair` returns tokens → 200
- **Success (existing user)**: same path, but `ProvisionExternalUser` returns an existing user
- **BadRequest**: missing `code` or `state` in JSON body
- **BadRequest**: invalid or expired state
- **NotFound**: identity source not found
- **BadRequest**: source type is not `oidc`
- **BadGateway**: `getOIDCProvider` returns error (OIDC discovery failure)
- **BadGateway**: `ExchangeCode` returns error
- **BadGateway**: `VerifyIDToken` returns error
- **Conflict**: `ProvisionExternalUser` returns `service.ErrEmailConflict`
- **InternalServerError**: `ProvisionExternalUser` returns other error

## 3. Test Data Setup

Reuse the existing test DB helper pattern from `identity_source_test.go`:
- In-memory SQLite with `model.IdentitySource{}` and `model.SystemConfig{}` migrated
- Seed helper to insert OIDC/LDAP sources with encrypted or plain configs
- `newSSOHandlerForTest` builds `SSOHandler` with injectable stubs

## 4. Scope Boundaries

- **In scope**: `SSOHandler`, `SSOStateManager`, and their immediate dependencies.
- **Out of scope**: Real OIDC provider interactions (we stub them), real `AuthService` internals (we stub `ProvisionExternalUser` and `GenerateTokenPair`), and frontend tests.

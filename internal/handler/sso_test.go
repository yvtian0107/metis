package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"github.com/glebarez/sqlite"
	"github.com/samber/do/v2"
	"golang.org/x/oauth2"
	"gorm.io/gorm"

	"metis/internal/database"
	"metis/internal/model"
	"metis/internal/pkg/identity"
	"metis/internal/repository"
	"metis/internal/service"
)

func newTestDBForSSOHandler(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	gdb, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := gdb.AutoMigrate(&model.IdentitySource{}, &model.SystemConfig{}); err != nil {
		t.Fatalf("migrate test db: %v", err)
	}
	return gdb
}

func seedIdentitySource(t *testing.T, db *gorm.DB, name, sourceType, config, domains string, enabled bool) *model.IdentitySource {
	t.Helper()
	s := &model.IdentitySource{
		Name:    name,
		Type:    sourceType,
		Config:  config,
		Domains: domains,
		Enabled: enabled,
	}
	if err := db.Create(s).Error; err != nil {
		t.Fatalf("seed identity source: %v", err)
	}
	return s
}

func newIdentitySourceServiceForTest(t *testing.T, db *gorm.DB) *service.IdentitySourceService {
	t.Helper()
	injector := do.New()
	do.ProvideValue(injector, &database.DB{DB: db})
	do.Provide(injector, repository.NewIdentitySource)
	do.Provide(injector, service.NewIdentitySource)
	svc := do.MustInvoke[*service.IdentitySourceService](injector)
	svc.TestOIDCFn = func(ctx context.Context, issuerURL string) error { return nil }
	svc.TestLDAPFn = func(cfg *model.LDAPConfig) error { return nil }
	svc.LDAPAuthFn = func(cfg *model.LDAPConfig, username, password string) (*identity.LDAPAuthResult, error) {
		return &identity.LDAPAuthResult{DN: "cn=user", Username: "user"}, nil
	}
	return svc
}

func newSSOHandlerForTest(t *testing.T, db *gorm.DB) *SSOHandler {
	t.Helper()
	svc := newIdentitySourceServiceForTest(t, db)
	return &SSOHandler{
		svc:      svc,
		stateMgr: identity.NewSSOStateManager(),
	}
}

func setupSSORouter(h *SSOHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	v1 := r.Group("/api/v1")
	{
		v1.GET("/auth/check-domain", h.CheckDomain)
		v1.GET("/auth/sso/:id/authorize", h.InitiateSSO)
		v1.POST("/auth/sso/callback", h.SSOCallback)
	}
	return r
}

// mockOIDCProvider implements the internal oidcProvider interface.
type mockOIDCProvider struct {
	authURLFn      func(state string, pkce *identity.PKCEParams) string
	exchangeCodeFn func(ctx context.Context, code string, codeVerifier string) (*oauth2.Token, error)
	verifyIDTokenFn func(ctx context.Context, token *oauth2.Token) (*gooidc.IDToken, error)
}

func (m *mockOIDCProvider) AuthURL(state string, pkce *identity.PKCEParams) string {
	if m.authURLFn != nil {
		return m.authURLFn(state, pkce)
	}
	return ""
}

func (m *mockOIDCProvider) ExchangeCode(ctx context.Context, code string, codeVerifier string) (*oauth2.Token, error) {
	if m.exchangeCodeFn != nil {
		return m.exchangeCodeFn(ctx, code, codeVerifier)
	}
	return nil, fmt.Errorf("exchange code not implemented")
}

func (m *mockOIDCProvider) VerifyIDToken(ctx context.Context, token *oauth2.Token) (*gooidc.IDToken, error) {
	if m.verifyIDTokenFn != nil {
		return m.verifyIDTokenFn(ctx, token)
	}
	return nil, fmt.Errorf("verify id token not implemented")
}

func TestSSOHandler_CheckDomain_Success(t *testing.T) {
	db := newTestDBForSSOHandler(t)
	h := newSSOHandlerForTest(t, db)
	seedIdentitySource(t, db, "Okta", "oidc", `{}`, "acme.com", true)
	r := setupSSORouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/check-domain?email=user@acme.com", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	data := resp["data"].(map[string]any)
	if data["name"] != "Okta" {
		t.Fatalf("expected name Okta, got %v", data["name"])
	}
}

func TestSSOHandler_CheckDomain_MissingEmail(t *testing.T) {
	db := newTestDBForSSOHandler(t)
	h := newSSOHandlerForTest(t, db)
	r := setupSSORouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/check-domain", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSSOHandler_CheckDomain_InvalidEmail(t *testing.T) {
	db := newTestDBForSSOHandler(t)
	h := newSSOHandlerForTest(t, db)
	r := setupSSORouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/check-domain?email=not-an-email", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSSOHandler_CheckDomain_NotFound(t *testing.T) {
	db := newTestDBForSSOHandler(t)
	h := newSSOHandlerForTest(t, db)
	r := setupSSORouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/check-domain?email=user@unknown.com", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSSOHandler_InitiateSSO_Success(t *testing.T) {
	db := newTestDBForSSOHandler(t)
	h := newSSOHandlerForTest(t, db)
	seeded := seedIdentitySource(t, db, "Okta", "oidc", `{"issuerUrl":"https://example.com","clientId":"test"}`, "", true)
	h.getOIDCProvider = func(ctx context.Context, sourceID uint, cfg *model.OIDCConfig) (oidcProvider, error) {
		return &mockOIDCProvider{
			authURLFn: func(state string, pkce *identity.PKCEParams) string {
				return fmt.Sprintf("https://provider.com/authorize?state=%s", state)
			},
		}, nil
	}
	r := setupSSORouter(h)

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/auth/sso/%d/authorize", seeded.ID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	data := resp["data"].(map[string]any)
	authURL := data["authUrl"].(string)
	state := data["state"].(string)
	if !strings.Contains(authURL, state) {
		t.Fatalf("expected authUrl to contain state, got %s", authURL)
	}
}

func TestSSOHandler_InitiateSSO_InvalidID(t *testing.T) {
	db := newTestDBForSSOHandler(t)
	h := newSSOHandlerForTest(t, db)
	r := setupSSORouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/sso/abc/authorize", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSSOHandler_InitiateSSO_NotFound(t *testing.T) {
	db := newTestDBForSSOHandler(t)
	h := newSSOHandlerForTest(t, db)
	r := setupSSORouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/sso/9999/authorize", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSSOHandler_InitiateSSO_Disabled(t *testing.T) {
	db := newTestDBForSSOHandler(t)
	h := newSSOHandlerForTest(t, db)
	seeded := seedIdentitySource(t, db, "Okta", "oidc", `{"issuerUrl":"https://example.com"}`, "", false)
	r := setupSSORouter(h)

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/auth/sso/%d/authorize", seeded.ID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSSOHandler_InitiateSSO_NonOIDC(t *testing.T) {
	db := newTestDBForSSOHandler(t)
	h := newSSOHandlerForTest(t, db)
	seeded := seedIdentitySource(t, db, "LDAP", "ldap", `{}`, "", true)
	r := setupSSORouter(h)

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/auth/sso/%d/authorize", seeded.ID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSSOHandler_SSOCallback_Success(t *testing.T) {
	db := newTestDBForSSOHandler(t)
	h := newSSOHandlerForTest(t, db)
	seeded := seedIdentitySource(t, db, "Okta", "oidc", `{"issuerUrl":"https://example.com","clientId":"test"}`, "", true)

	state, _ := h.stateMgr.Generate(seeded.ID, "verifier")
	h.getOIDCProvider = func(ctx context.Context, sourceID uint, cfg *model.OIDCConfig) (oidcProvider, error) {
		return &mockOIDCProvider{
			exchangeCodeFn: func(ctx context.Context, code string, codeVerifier string) (*oauth2.Token, error) {
				return &oauth2.Token{AccessToken: "at"}, nil
			},
			verifyIDTokenFn: func(ctx context.Context, token *oauth2.Token) (*gooidc.IDToken, error) {
				return &gooidc.IDToken{}, nil
			},
		}, nil
	}
	h.extractClaims = func(idToken *gooidc.IDToken) (*identity.OIDCClaims, error) {
		return &identity.OIDCClaims{Sub: "sub-1", Email: "user@acme.com", Name: "User"}, nil
	}
	h.provisionExternalUser = func(params service.ExternalUserParams) (*model.User, error) {
		return &model.User{Username: "user", Email: "user@acme.com"}, nil
	}
	h.generateTokenPair = func(user *model.User, ip, ua string) (*service.TokenPair, error) {
		return &service.TokenPair{AccessToken: "access", RefreshToken: "refresh"}, nil
	}
	r := setupSSORouter(h)

	body := fmt.Sprintf(`{"code":"authcode","state":"%s"}`, state)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/sso/callback", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].(map[string]any)
	if data["accessToken"] != "access" {
		t.Fatalf("expected access token, got %v", data)
	}
}

func TestSSOHandler_SSOCallback_ExistingUser(t *testing.T) {
	db := newTestDBForSSOHandler(t)
	h := newSSOHandlerForTest(t, db)
	seeded := seedIdentitySource(t, db, "Okta", "oidc", `{"issuerUrl":"https://example.com","clientId":"test"}`, "", true)

	state, _ := h.stateMgr.Generate(seeded.ID, "verifier")
	h.getOIDCProvider = func(ctx context.Context, sourceID uint, cfg *model.OIDCConfig) (oidcProvider, error) {
		return &mockOIDCProvider{
			exchangeCodeFn: func(ctx context.Context, code string, codeVerifier string) (*oauth2.Token, error) {
				return &oauth2.Token{AccessToken: "at"}, nil
			},
			verifyIDTokenFn: func(ctx context.Context, token *oauth2.Token) (*gooidc.IDToken, error) {
				return &gooidc.IDToken{}, nil
			},
		}, nil
	}
	h.extractClaims = func(idToken *gooidc.IDToken) (*identity.OIDCClaims, error) {
		return &identity.OIDCClaims{Sub: "sub-1", Email: "user@acme.com", Name: "User"}, nil
	}
	h.provisionExternalUser = func(params service.ExternalUserParams) (*model.User, error) {
		return &model.User{Username: "existing", Email: "user@acme.com"}, nil
	}
	h.generateTokenPair = func(user *model.User, ip, ua string) (*service.TokenPair, error) {
		return &service.TokenPair{AccessToken: "access2", RefreshToken: "refresh2"}, nil
	}
	r := setupSSORouter(h)

	body := fmt.Sprintf(`{"code":"authcode","state":"%s"}`, state)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/sso/callback", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSSOHandler_SSOCallback_MissingFields(t *testing.T) {
	db := newTestDBForSSOHandler(t)
	h := newSSOHandlerForTest(t, db)
	r := setupSSORouter(h)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/sso/callback", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSSOHandler_SSOCallback_InvalidState(t *testing.T) {
	db := newTestDBForSSOHandler(t)
	h := newSSOHandlerForTest(t, db)
	r := setupSSORouter(h)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/sso/callback", strings.NewReader(`{"code":"c","state":"bad-state"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSSOHandler_SSOCallback_SourceNotFound(t *testing.T) {
	db := newTestDBForSSOHandler(t)
	h := newSSOHandlerForTest(t, db)
	state, _ := h.stateMgr.Generate(9999, "")
	r := setupSSORouter(h)

	body := fmt.Sprintf(`{"code":"c","state":"%s"}`, state)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/sso/callback", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSSOHandler_SSOCallback_NonOIDC(t *testing.T) {
	db := newTestDBForSSOHandler(t)
	h := newSSOHandlerForTest(t, db)
	seeded := seedIdentitySource(t, db, "LDAP", "ldap", `{}`, "", true)
	state, _ := h.stateMgr.Generate(seeded.ID, "")
	r := setupSSORouter(h)

	body := fmt.Sprintf(`{"code":"c","state":"%s"}`, state)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/sso/callback", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSSOHandler_SSOCallback_OIDCDiscoveryFailed(t *testing.T) {
	db := newTestDBForSSOHandler(t)
	h := newSSOHandlerForTest(t, db)
	seeded := seedIdentitySource(t, db, "Okta", "oidc", `{"issuerUrl":"https://example.com"}`, "", true)
	state, _ := h.stateMgr.Generate(seeded.ID, "")
	h.getOIDCProvider = func(ctx context.Context, sourceID uint, cfg *model.OIDCConfig) (oidcProvider, error) {
		return nil, fmt.Errorf("discovery failed")
	}
	r := setupSSORouter(h)

	body := fmt.Sprintf(`{"code":"c","state":"%s"}`, state)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/sso/callback", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSSOHandler_SSOCallback_ExchangeCodeFailed(t *testing.T) {
	db := newTestDBForSSOHandler(t)
	h := newSSOHandlerForTest(t, db)
	seeded := seedIdentitySource(t, db, "Okta", "oidc", `{"issuerUrl":"https://example.com"}`, "", true)
	state, _ := h.stateMgr.Generate(seeded.ID, "")
	h.getOIDCProvider = func(ctx context.Context, sourceID uint, cfg *model.OIDCConfig) (oidcProvider, error) {
		return &mockOIDCProvider{
			exchangeCodeFn: func(ctx context.Context, code string, codeVerifier string) (*oauth2.Token, error) {
				return nil, fmt.Errorf("exchange failed")
			},
		}, nil
	}
	r := setupSSORouter(h)

	body := fmt.Sprintf(`{"code":"c","state":"%s"}`, state)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/sso/callback", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSSOHandler_SSOCallback_VerifyIDTokenFailed(t *testing.T) {
	db := newTestDBForSSOHandler(t)
	h := newSSOHandlerForTest(t, db)
	seeded := seedIdentitySource(t, db, "Okta", "oidc", `{"issuerUrl":"https://example.com"}`, "", true)
	state, _ := h.stateMgr.Generate(seeded.ID, "")
	h.getOIDCProvider = func(ctx context.Context, sourceID uint, cfg *model.OIDCConfig) (oidcProvider, error) {
		return &mockOIDCProvider{
			exchangeCodeFn: func(ctx context.Context, code string, codeVerifier string) (*oauth2.Token, error) {
				return &oauth2.Token{AccessToken: "at"}, nil
			},
			verifyIDTokenFn: func(ctx context.Context, token *oauth2.Token) (*gooidc.IDToken, error) {
				return nil, fmt.Errorf("verify failed")
			},
		}, nil
	}
	r := setupSSORouter(h)

	body := fmt.Sprintf(`{"code":"c","state":"%s"}`, state)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/sso/callback", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSSOHandler_SSOCallback_EmailConflict(t *testing.T) {
	db := newTestDBForSSOHandler(t)
	h := newSSOHandlerForTest(t, db)
	seeded := seedIdentitySource(t, db, "Okta", "oidc", `{"issuerUrl":"https://example.com"}`, "", true)
	state, _ := h.stateMgr.Generate(seeded.ID, "")
	h.getOIDCProvider = func(ctx context.Context, sourceID uint, cfg *model.OIDCConfig) (oidcProvider, error) {
		return &mockOIDCProvider{
			exchangeCodeFn: func(ctx context.Context, code string, codeVerifier string) (*oauth2.Token, error) {
				return &oauth2.Token{AccessToken: "at"}, nil
			},
			verifyIDTokenFn: func(ctx context.Context, token *oauth2.Token) (*gooidc.IDToken, error) {
				return &gooidc.IDToken{}, nil
			},
		}, nil
	}
	h.extractClaims = func(idToken *gooidc.IDToken) (*identity.OIDCClaims, error) {
		return &identity.OIDCClaims{Sub: "sub-1", Email: "user@acme.com", Name: "User"}, nil
	}
	h.provisionExternalUser = func(params service.ExternalUserParams) (*model.User, error) {
		return nil, service.ErrEmailConflict
	}
	r := setupSSORouter(h)

	body := fmt.Sprintf(`{"code":"c","state":"%s"}`, state)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/sso/callback", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSSOHandler_SSOCallback_ProvisionError(t *testing.T) {
	db := newTestDBForSSOHandler(t)
	h := newSSOHandlerForTest(t, db)
	seeded := seedIdentitySource(t, db, "Okta", "oidc", `{"issuerUrl":"https://example.com"}`, "", true)
	state, _ := h.stateMgr.Generate(seeded.ID, "")
	h.getOIDCProvider = func(ctx context.Context, sourceID uint, cfg *model.OIDCConfig) (oidcProvider, error) {
		return &mockOIDCProvider{
			exchangeCodeFn: func(ctx context.Context, code string, codeVerifier string) (*oauth2.Token, error) {
				return &oauth2.Token{AccessToken: "at"}, nil
			},
			verifyIDTokenFn: func(ctx context.Context, token *oauth2.Token) (*gooidc.IDToken, error) {
				return &gooidc.IDToken{}, nil
			},
		}, nil
	}
	h.extractClaims = func(idToken *gooidc.IDToken) (*identity.OIDCClaims, error) {
		return &identity.OIDCClaims{Sub: "sub-1", Email: "user@acme.com", Name: "User"}, nil
	}
	h.provisionExternalUser = func(params service.ExternalUserParams) (*model.User, error) {
		return nil, fmt.Errorf("some db error")
	}
	r := setupSSORouter(h)

	body := fmt.Sprintf(`{"code":"c","state":"%s"}`, state)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/sso/callback", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}

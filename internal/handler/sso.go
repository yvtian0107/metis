package handler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"metis/internal/model"
	"metis/internal/pkg/identity"
	"metis/internal/service"
)

// oidcProvider is the subset of identity.OIDCProvider used by SSOHandler.
type oidcProvider interface {
	AuthURL(state string, pkce *identity.PKCEParams) string
	ExchangeCode(ctx context.Context, code string, codeVerifier string) (*oauth2.Token, error)
	VerifyIDToken(ctx context.Context, token *oauth2.Token) (*gooidc.IDToken, error)
}

// SSOHandler handles public SSO endpoints (check-domain, initiate, callback).
type SSOHandler struct {
	svc                   *service.IdentitySourceService
	authSvc               *service.AuthService
	stateMgr              *identity.SSOStateManager
	getOIDCProvider       func(ctx context.Context, sourceID uint, cfg *model.OIDCConfig) (oidcProvider, error)
	extractClaims         func(idToken *gooidc.IDToken) (*identity.OIDCClaims, error)
	provisionExternalUser func(params service.ExternalUserParams) (*model.User, error)
	generateTokenPair     func(user *model.User, ip, ua string) (*service.TokenPair, error)
}

func (h *SSOHandler) resolveOIDCProvider(ctx context.Context, sourceID uint, cfg *model.OIDCConfig) (oidcProvider, error) {
	if h.getOIDCProvider != nil {
		return h.getOIDCProvider(ctx, sourceID, cfg)
	}
	return identity.GetOIDCProvider(ctx, sourceID, cfg)
}

func (h *SSOHandler) resolveExtractClaims(idToken *gooidc.IDToken) (*identity.OIDCClaims, error) {
	if h.extractClaims != nil {
		return h.extractClaims(idToken)
	}
	return identity.ExtractClaims(idToken)
}

func (h *SSOHandler) resolveProvisionExternalUser(params service.ExternalUserParams) (*model.User, error) {
	if h.provisionExternalUser != nil {
		return h.provisionExternalUser(params)
	}
	return h.authSvc.ProvisionExternalUser(params)
}

func (h *SSOHandler) resolveGenerateTokenPair(user *model.User, ip, ua string) (*service.TokenPair, error) {
	if h.generateTokenPair != nil {
		return h.generateTokenPair(user, ip, ua)
	}
	return h.authSvc.GenerateTokenPair(user, ip, ua)
}

// CheckDomain checks if an email domain is bound to an identity source.
// GET /api/v1/auth/check-domain?email=user@acme.com
func (h *SSOHandler) CheckDomain(c *gin.Context) {
	email := c.Query("email")
	if email == "" {
		Fail(c, http.StatusBadRequest, "email is required")
		return
	}

	domain := service.ExtractDomain(email)
	if domain == "" {
		Fail(c, http.StatusBadRequest, "invalid email format")
		return
	}

	source, err := h.svc.FindByDomain(domain)
	if err != nil {
		Fail(c, http.StatusNotFound, "no identity source for this domain")
		return
	}

	OK(c, gin.H{
		"id":       source.ID,
		"name":     source.Name,
		"type":     source.Type,
		"forceSso": source.ForceSso,
	})
}

// InitiateSSO starts the OIDC SSO flow for the given identity source.
// GET /api/v1/auth/sso/:id/authorize
func (h *SSOHandler) InitiateSSO(c *gin.Context) {
	id, err := parseIdentityID(c)
	if err != nil {
		Fail(c, http.StatusBadRequest, "invalid id")
		return
	}

	source, cfg, err := h.svc.GetDecryptedConfig(id)
	if err != nil {
		if errors.Is(err, service.ErrSourceNotFound) {
			Fail(c, http.StatusNotFound, "identity source not found")
		} else {
			Fail(c, http.StatusInternalServerError, err.Error())
		}
		return
	}

	if !source.Enabled {
		Fail(c, http.StatusBadRequest, "identity source not available")
		return
	}

	if source.Type != "oidc" {
		Fail(c, http.StatusBadRequest, "SSO initiation is only supported for OIDC sources")
		return
	}

	oidcCfg, ok := cfg.(*model.OIDCConfig)
	if !ok || oidcCfg == nil {
		Fail(c, http.StatusInternalServerError, "invalid OIDC config")
		return
	}

	ctx := context.Background()
	provider, err := h.resolveOIDCProvider(ctx, source.ID, oidcCfg)
	if err != nil {
		Fail(c, http.StatusBadGateway, "OIDC discovery failed: "+err.Error())
		return
	}

	var pkce *identity.PKCEParams
	codeVerifier := ""
	if oidcCfg.UsePKCE {
		pkce, err = identity.GeneratePKCE()
		if err != nil {
			Fail(c, http.StatusInternalServerError, "PKCE generation failed")
			return
		}
		codeVerifier = pkce.Verifier
	}

	state, err := h.stateMgr.Generate(source.ID, codeVerifier)
	if err != nil {
		Fail(c, http.StatusInternalServerError, "state generation failed")
		return
	}

	authURL := provider.AuthURL(state, pkce)

	OK(c, gin.H{
		"authUrl": authURL,
		"state":   state,
	})
}

type ssoCallbackRequest struct {
	Code  string `json:"code" binding:"required"`
	State string `json:"state" binding:"required"`
}

// SSOCallback handles the OIDC callback after user authentication.
// POST /api/v1/auth/sso/callback
func (h *SSOHandler) SSOCallback(c *gin.Context) {
	var req ssoCallbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	stateMeta, err := h.stateMgr.Validate(req.State)
	if err != nil {
		Fail(c, http.StatusBadRequest, "invalid or expired state")
		return
	}

	source, cfg, err := h.svc.GetDecryptedConfig(stateMeta.SourceID)
	if err != nil {
		Fail(c, http.StatusInternalServerError, "identity source not found")
		return
	}

	if source.Type != "oidc" {
		Fail(c, http.StatusBadRequest, "invalid source type for SSO callback")
		return
	}

	oidcCfg, ok := cfg.(*model.OIDCConfig)
	if !ok {
		Fail(c, http.StatusInternalServerError, "invalid OIDC config")
		return
	}

	ctx := context.Background()

	provider, err := h.resolveOIDCProvider(ctx, source.ID, oidcCfg)
	if err != nil {
		Fail(c, http.StatusBadGateway, "OIDC discovery failed")
		return
	}

	token, err := provider.ExchangeCode(ctx, req.Code, stateMeta.CodeVerifier)
	if err != nil {
		Fail(c, http.StatusBadGateway, "code exchange failed: "+err.Error())
		return
	}

	idToken, err := provider.VerifyIDToken(ctx, token)
	if err != nil {
		Fail(c, http.StatusBadGateway, "ID token verification failed: "+err.Error())
		return
	}

	claims, err := h.resolveExtractClaims(idToken)
	if err != nil {
		Fail(c, http.StatusInternalServerError, "failed to extract claims")
		return
	}

	providerName := fmt.Sprintf("oidc_%d", source.ID)
	user, err := h.resolveProvisionExternalUser(service.ExternalUserParams{
		Provider:          providerName,
		ExternalID:        claims.Sub,
		Email:             claims.Email,
		DisplayName:       claims.Name,
		AvatarURL:         claims.Picture,
		DefaultRoleID:     source.DefaultRoleID,
		ConflictStrategy:  source.ConflictStrategy,
		PreferredUsername: claims.Name,
	})
	if err != nil {
		if errors.Is(err, service.ErrEmailConflict) {
			Fail(c, http.StatusConflict, err.Error())
		} else {
			slog.Error("SSO JIT provision failed", "error", err)
			Fail(c, http.StatusInternalServerError, "user provisioning failed")
		}
		return
	}

	ip := c.ClientIP()
	ua := c.GetHeader("User-Agent")
	tokenPair, err := h.resolveGenerateTokenPair(user, ip, ua)
	if err != nil {
		Fail(c, http.StatusInternalServerError, "token generation failed")
		return
	}

	OK(c, tokenPair)
}

package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"metis/internal/model"
	"metis/internal/pkg/oauth"
	"metis/internal/service"
)

type AuthHandler struct {
	auth        *service.AuthService
	userSvc     *service.UserService
	menuSvc     *service.MenuService
	providerSvc *service.AuthProviderService
	connSvc     *service.UserConnectionService
	stateMgr    *oauth.StateManager
	auditSvc    *service.AuditLogService
}

type loginReq struct {
	Username      string `json:"username" binding:"required"`
	Password      string `json:"password" binding:"required"`
	CaptchaID     string `json:"captchaId"`
	CaptchaAnswer string `json:"captchaAnswer"`
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req loginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	ip := c.ClientIP()
	ua := c.GetHeader("User-Agent")

	pair, err := h.auth.Login(req.Username, req.Password, req.CaptchaID, req.CaptchaAnswer, ip, ua)
	if err != nil {
		// Audit: login failure
		reason := "unknown"
		switch {
		case errors.Is(err, service.ErrInvalidCredentials):
			reason = "invalid_credentials"
			Fail(c, http.StatusUnauthorized, err.Error())
		case errors.Is(err, service.ErrAccountDisabled):
			reason = "account_disabled"
			Fail(c, http.StatusUnauthorized, err.Error())
		case errors.Is(err, service.ErrAccountLocked):
			reason = "account_locked"
			Fail(c, http.StatusLocked, err.Error())
		case errors.Is(err, service.ErrCaptchaRequired):
			reason = "captcha_required"
			Fail(c, http.StatusBadRequest, err.Error())
		case errors.Is(err, service.ErrCaptchaInvalid):
			reason = "captcha_invalid"
			Fail(c, http.StatusBadRequest, err.Error())
		case errors.Is(err, service.ErrForcedSSO):
			reason = "forced_sso"
			Fail(c, http.StatusForbidden, err.Error())
		default:
			reason = "internal_error"
			Fail(c, http.StatusInternalServerError, err.Error())
		}

		detailJSON, _ := json.Marshal(map[string]string{"reason": reason})
		detail := string(detailJSON)
		h.auditSvc.Log(model.AuditLog{
			Category:  model.AuditCategoryAuth,
			Action:    "login_failed",
			Username:  req.Username,
			Summary:   "登录失败: " + req.Username,
			Level:     model.AuditLevelWarn,
			IPAddress: ip,
			UserAgent: ua,
			Detail:    &detail,
		})
		return
	}

	// 2FA required — return 202 with twoFactorToken
	if pair.NeedsTwoFactor {
		c.JSON(http.StatusAccepted, gin.H{
			"code":           0,
			"message":        "2FA required",
			"needsTwoFactor": true,
			"twoFactorToken": pair.TwoFactorToken,
		})
		return
	}

	// Audit: login success
	h.auditSvc.Log(model.AuditLog{
		Category:  model.AuditCategoryAuth,
		Action:    "login_success",
		UserID:    &pair.UserID,
		Username:  req.Username,
		Summary:   "用户 " + req.Username + " 登录成功",
		Level:     model.AuditLevelInfo,
		IPAddress: ip,
		UserAgent: ua,
	})

	OK(c, pair)
}

type registerReq struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	Email    string `json:"email"`
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req registerReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	ip := c.ClientIP()
	ua := c.GetHeader("User-Agent")
	pair, err := h.auth.Register(req.Username, req.Password, req.Email, ip, ua)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrRegistrationClosed):
			Fail(c, http.StatusForbidden, err.Error())
		case errors.Is(err, service.ErrUsernameExists):
			Fail(c, http.StatusBadRequest, err.Error())
		case errors.Is(err, service.ErrPasswordViolation):
			Fail(c, http.StatusBadRequest, err.Error())
		case errors.Is(err, service.ErrDefaultRoleNotFound):
			Fail(c, http.StatusInternalServerError, err.Error())
		default:
			Fail(c, http.StatusInternalServerError, err.Error())
		}
		return
	}

	OK(c, pair)
}

func (h *AuthHandler) RegistrationStatus(c *gin.Context) {
	OK(c, gin.H{"registrationOpen": h.auth.IsRegistrationOpen()})
}

type logoutReq struct {
	RefreshToken string `json:"refreshToken" binding:"required"`
}

func (h *AuthHandler) Logout(c *gin.Context) {
	var req logoutReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	_ = h.auth.Logout(req.RefreshToken)

	// Audit: logout
	userID := c.GetUint("userId")
	h.auditSvc.Log(model.AuditLog{
		Category:  model.AuditCategoryAuth,
		Action:    "logout",
		UserID:    &userID,
		Summary:   "用户登出",
		Level:     model.AuditLevelInfo,
		IPAddress: c.ClientIP(),
	})

	OK(c, nil)
}

type refreshReq struct {
	RefreshToken string `json:"refreshToken" binding:"required"`
}

func (h *AuthHandler) Refresh(c *gin.Context) {
	var req refreshReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	pair, err := h.auth.RefreshTokens(req.RefreshToken, c.ClientIP(), c.GetHeader("User-Agent"))
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidRefreshToken),
			errors.Is(err, service.ErrRefreshTokenExpired),
			errors.Is(err, service.ErrTokenReuse),
			errors.Is(err, service.ErrAccountDisabled):
			Fail(c, http.StatusUnauthorized, err.Error())
		default:
			Fail(c, http.StatusInternalServerError, err.Error())
		}
		return
	}

	OK(c, pair)
}

func (h *AuthHandler) GetMe(c *gin.Context) {
	userID := c.GetUint("userId")
	user, err := h.auth.GetCurrentUser(userID)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	resp := user.ToResponse()

	// Attach connections
	conns, _ := h.auth.GetUserConnections(userID)
	if conns != nil {
		connResps := make([]any, 0, len(conns))
		for _, c := range conns {
			connResps = append(connResps, gin.H{
				"provider":     c.Provider,
				"externalName": c.ExternalName,
			})
		}
		resp.Connections = nil // clear the default
		_ = connResps         // we'll use the gin.H approach below
	}

	permissions := h.menuSvc.GetUserPermissions(user.Role.Code)

	// Build connections list for response
	var connections []gin.H
	if conns != nil {
		for _, conn := range conns {
			connections = append(connections, gin.H{
				"provider":     conn.Provider,
				"externalName": conn.ExternalName,
			})
		}
	}

	OK(c, gin.H{
		"user":        resp,
		"permissions": permissions,
		"connections": connections,
	})
}

type changePasswordReq struct {
	OldPassword string `json:"oldPassword" binding:"required"`
	NewPassword string `json:"newPassword" binding:"required"`
}

func (h *AuthHandler) ChangePassword(c *gin.Context) {
	var req changePasswordReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	userID := c.GetUint("userId")
	if err := h.auth.ChangePassword(userID, req.OldPassword, req.NewPassword); err != nil {
		if errors.Is(err, service.ErrOldPasswordWrong) {
			Fail(c, http.StatusBadRequest, err.Error())
			return
		}
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	OK(c, nil)
}

type updateProfileReq struct {
	Locale   *string `json:"locale"`
	Timezone *string `json:"timezone"`
}

func (h *AuthHandler) UpdateProfile(c *gin.Context) {
	var req updateProfileReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	userID := c.GetUint("userId")
	user, err := h.auth.GetCurrentUser(userID)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	if req.Locale != nil {
		user.Locale = *req.Locale
	}
	if req.Timezone != nil {
		user.Timezone = *req.Timezone
	}

	if err := h.userSvc.UpdateProfile(user); err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	OK(c, user.ToResponse())
}

// --- OAuth endpoints ---

// ListProviders returns enabled OAuth providers (public, no auth required).
func (h *AuthHandler) ListProviders(c *gin.Context) {
	providers, err := h.providerSvc.ListEnabled()
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	result := make([]any, 0, len(providers))
	for _, p := range providers {
		result = append(result, p.ToPublicInfo())
	}
	OK(c, result)
}

// InitiateOAuth generates an authorization URL for the given provider.
func (h *AuthHandler) InitiateOAuth(c *gin.Context) {
	providerKey := c.Param("provider")

	p, err := h.providerSvc.FindByKey(providerKey)
	if err != nil || !p.Enabled {
		Fail(c, http.StatusBadRequest, "provider not available")
		return
	}

	op, err := h.providerSvc.BuildOAuthProvider(p)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	state, err := h.stateMgr.Generate(providerKey)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	authURL := op.GetAuthURL(state)
	OK(c, gin.H{"authURL": authURL, "state": state})
}

type oauthCallbackReq struct {
	Provider string `json:"provider" binding:"required"`
	Code     string `json:"code" binding:"required"`
	State    string `json:"state" binding:"required"`
}

// OAuthCallback handles the OAuth callback from the frontend.
func (h *AuthHandler) OAuthCallback(c *gin.Context) {
	var req oauthCallbackReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	// Validate state
	meta, err := h.stateMgr.Validate(req.State)
	if err != nil {
		Fail(c, http.StatusBadRequest, "invalid or expired state")
		return
	}

	// Handle bind mode (authenticated user binding an account)
	if meta.BindMode {
		h.handleBindCallback(c, meta, req)
		return
	}

	// Login mode
	p, err := h.providerSvc.FindByKey(req.Provider)
	if err != nil || !p.Enabled {
		Fail(c, http.StatusBadRequest, "provider not available")
		return
	}

	op, err := h.providerSvc.BuildOAuthProvider(p)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	userInfo, err := op.ExchangeCode(c.Request.Context(), req.Code)
	if err != nil {
		Fail(c, http.StatusBadRequest, "failed to exchange code: "+err.Error())
		return
	}

	pair, err := h.auth.OAuthLogin(
		req.Provider, userInfo.ID, userInfo.Name, userInfo.Email, userInfo.AvatarURL,
		c.ClientIP(), c.GetHeader("User-Agent"),
	)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrEmailConflict):
			Fail(c, http.StatusConflict, err.Error())
		case errors.Is(err, service.ErrAccountDisabled):
			Fail(c, http.StatusUnauthorized, "account disabled")
		default:
			Fail(c, http.StatusInternalServerError, err.Error())
		}
		return
	}

	OK(c, pair)
}

func (h *AuthHandler) handleBindCallback(c *gin.Context, meta *oauth.StateMeta, req oauthCallbackReq) {
	p, err := h.providerSvc.FindByKey(req.Provider)
	if err != nil || !p.Enabled {
		Fail(c, http.StatusBadRequest, "provider not available")
		return
	}

	op, err := h.providerSvc.BuildOAuthProvider(p)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	userInfo, err := op.ExchangeCode(c.Request.Context(), req.Code)
	if err != nil {
		Fail(c, http.StatusBadRequest, "failed to exchange code: "+err.Error())
		return
	}

	if err := h.connSvc.Bind(meta.UserID, req.Provider, userInfo.ID, userInfo.Name, userInfo.Email, userInfo.AvatarURL); err != nil {
		switch {
		case errors.Is(err, service.ErrAlreadyBound):
			Fail(c, http.StatusBadRequest, err.Error())
		case errors.Is(err, service.ErrExternalIDBound):
			Fail(c, http.StatusConflict, err.Error())
		default:
			Fail(c, http.StatusInternalServerError, err.Error())
		}
		return
	}

	OK(c, nil)
}

// --- Connection management endpoints (authenticated) ---

// ListConnections returns the current user's OAuth connections.
func (h *AuthHandler) ListConnections(c *gin.Context) {
	userID := c.GetUint("userId")
	conns, err := h.connSvc.ListByUser(userID)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	result := make([]any, 0, len(conns))
	for _, conn := range conns {
		result = append(result, conn.ToResponse())
	}
	OK(c, result)
}

// InitiateBind starts the OAuth flow to bind an external account.
func (h *AuthHandler) InitiateBind(c *gin.Context) {
	providerKey := c.Param("provider")
	userID := c.GetUint("userId")

	p, err := h.providerSvc.FindByKey(providerKey)
	if err != nil || !p.Enabled {
		Fail(c, http.StatusBadRequest, "provider not available")
		return
	}

	// Check if already bound
	if _, err := h.connSvc.ListByUser(userID); err == nil {
		conns, _ := h.connSvc.ListByUser(userID)
		for _, conn := range conns {
			if conn.Provider == providerKey {
				Fail(c, http.StatusBadRequest, "already bound to this provider")
				return
			}
		}
	}

	op, err := h.providerSvc.BuildOAuthProvider(p)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	state, err := h.stateMgr.GenerateForBind(providerKey, userID)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	authURL := op.GetAuthURL(state)
	OK(c, gin.H{"authURL": authURL, "state": state})
}

// BindCallback handles the bind callback from the frontend.
func (h *AuthHandler) BindCallback(c *gin.Context) {
	var req oauthCallbackReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	meta, err := h.stateMgr.Validate(req.State)
	if err != nil {
		Fail(c, http.StatusBadRequest, "invalid or expired state")
		return
	}

	// Ensure the bind is for the current user
	userID := c.GetUint("userId")
	if !meta.BindMode || meta.UserID != userID {
		Fail(c, http.StatusBadRequest, "invalid bind state")
		return
	}

	h.handleBindCallback(c, meta, req)
}

// Unbind removes an OAuth connection for the current user.
func (h *AuthHandler) Unbind(c *gin.Context) {
	providerKey := c.Param("provider")
	userID := c.GetUint("userId")

	if err := h.connSvc.Unbind(userID, providerKey); err != nil {
		switch {
		case errors.Is(err, service.ErrConnectionNotFound):
			Fail(c, http.StatusNotFound, err.Error())
		case errors.Is(err, service.ErrLastLoginMethod):
			Fail(c, http.StatusBadRequest, err.Error())
		default:
			Fail(c, http.StatusInternalServerError, err.Error())
		}
		return
	}

	OK(c, nil)
}

package service

import (
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/samber/do/v2"
	"gorm.io/gorm"

	"metis/internal/model"
	"metis/internal/pkg/token"
	"metis/internal/repository"
)

var (
	ErrInvalidCredentials  = errors.New("error.auth.invalid_credentials")
	ErrAccountDisabled     = errors.New("error.auth.account_disabled")
	ErrAccountLocked       = errors.New("error.auth.account_locked")
	ErrInvalidRefreshToken = errors.New("error.auth.invalid_refresh_token")
	ErrRefreshTokenExpired = errors.New("error.auth.refresh_token_expired")
	ErrTokenReuse          = errors.New("error.auth.token_reuse")
	ErrOldPasswordWrong    = errors.New("error.auth.old_password_wrong")
	ErrEmailConflict       = errors.New("error.auth.email_conflict")
	ErrForcedSSO           = errors.New("error.auth.forced_sso")
	ErrCaptchaRequired     = errors.New("error.auth.captcha_required")
	ErrCaptchaInvalid      = errors.New("error.auth.captcha_invalid")
	ErrRegistrationClosed  = errors.New("error.auth.registration_closed")
	ErrDefaultRoleNotFound = errors.New("error.auth.default_role_not_found")
)

type AuthService struct {
	userRepo         *repository.UserRepo
	refreshTokenRepo *repository.RefreshTokenRepo
	connRepo         *repository.UserConnectionRepo
	sysConfigRepo    *repository.SysConfigRepo
	roleRepo         *repository.RoleRepo
	menuSvc          *MenuService
	settingsSvc      *SettingsService
	captchaSvc       *CaptchaService
	blacklist        *token.TokenBlacklist
	jwtSecret        []byte
	identitySvc      *IdentitySourceService // optional, nil when identity features not registered
}

func NewAuth(i do.Injector) (*AuthService, error) {
	svc := &AuthService{
		userRepo:         do.MustInvoke[*repository.UserRepo](i),
		refreshTokenRepo: do.MustInvoke[*repository.RefreshTokenRepo](i),
		connRepo:         do.MustInvoke[*repository.UserConnectionRepo](i),
		sysConfigRepo:    do.MustInvoke[*repository.SysConfigRepo](i),
		roleRepo:         do.MustInvoke[*repository.RoleRepo](i),
		menuSvc:          do.MustInvoke[*MenuService](i),
		settingsSvc:      do.MustInvoke[*SettingsService](i),
		captchaSvc:       do.MustInvoke[*CaptchaService](i),
		blacklist:        do.MustInvoke[*token.TokenBlacklist](i),
		jwtSecret:        do.MustInvoke[[]byte](i),
	}

	// Optionally resolve IdentitySourceService (nil when identity features not registered).
	identitySvc, err := do.Invoke[*IdentitySourceService](i)
	if err == nil {
		svc.identitySvc = identitySvc
	}

	return svc, nil
}

// TokenPair is the response for login and refresh operations.
type TokenPair struct {
	AccessToken           string   `json:"accessToken,omitempty"`
	RefreshToken          string   `json:"refreshToken,omitempty"`
	ExpiresIn             int64    `json:"expiresIn,omitempty"`
	Permissions           []string `json:"permissions,omitempty"`
	UserID                uint     `json:"-"`
	NeedsTwoFactor        bool     `json:"needsTwoFactor,omitempty"`
	TwoFactorToken        string   `json:"twoFactorToken,omitempty"`
	RequireTwoFactorSetup bool     `json:"requireTwoFactorSetup,omitempty"`
}

func (s *AuthService) Login(username, password, captchaID, captchaAnswer, ip, ua string) (*TokenPair, error) {
	user, err := s.userRepo.FindByUsername(username)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// User not found locally — try external auth if available
			if s.identitySvc != nil {
				return s.tryExternalAuth(username, password, ip, ua)
			}
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}

	// Check lockout before anything else
	if user.IsLocked() {
		return nil, ErrAccountLocked
	}

	if !user.IsActive {
		return nil, ErrAccountDisabled
	}

	// Verify captcha (if enabled)
	if s.settingsSvc.GetCaptchaProvider() != "none" {
		if captchaID == "" || captchaAnswer == "" {
			return nil, ErrCaptchaRequired
		}
		if !s.captchaSvc.Verify(captchaID, captchaAnswer) {
			return nil, ErrCaptchaInvalid
		}
	}

	// Check forced SSO: if user's email domain requires SSO, reject password login
	if s.identitySvc != nil && user.Email != "" && s.identitySvc.IsForcedSSO(user.Email) {
		return nil, ErrForcedSSO
	}

	// OAuth-only users (no password) cannot login with password
	if !user.HasPassword() {
		return nil, ErrInvalidCredentials
	}

	if !token.CheckPassword(user.Password, password) {
		// Local password failed — try external auth (e.g., LDAP) if available
		if s.identitySvc != nil {
			return s.tryExternalAuth(username, password, ip, ua)
		}
		// Track failed attempt and potentially lock the account
		s.handleFailedLogin(user.ID)
		return nil, ErrInvalidCredentials
	}

	// Successful login — reset failed attempts
	if user.FailedLoginAttempts > 0 {
		_ = s.userRepo.ResetFailedAttempts(user.ID)
	}

	// Check 2FA: if enabled, return a temporary token for 2FA verification
	if user.TwoFactorEnabled {
		tfToken, err := token.GenerateTwoFactorToken(user.ID, s.jwtSecret)
		if err != nil {
			return nil, err
		}
		return &TokenPair{
			NeedsTwoFactor: true,
			TwoFactorToken: tfToken,
			UserID:         user.ID,
		}, nil
	}

	// Check if 2FA is required but not set up (require_two_factor policy)
	if s.settingsSvc.IsTwoFactorRequired() {
		// Issue normal tokens but flag that 2FA setup is required
		s.enforceConcurrentLimit(user.ID)
		pair, err := s.GenerateTokenPair(user, ip, ua)
		if err != nil {
			return nil, err
		}
		pair.RequireTwoFactorSetup = true
		return pair, nil
	}

	// Enforce concurrent session limit
	s.enforceConcurrentLimit(user.ID)

	return s.GenerateTokenPair(user, ip, ua)
}

// tryExternalAuth attempts authentication via an ExternalAuthenticator (e.g., LDAP).
func (s *AuthService) tryExternalAuth(username, password, ip, ua string) (*TokenPair, error) {
	user, err := s.identitySvc.AuthenticateByPassword(username, password)
	if err != nil {
		return nil, ErrInvalidCredentials
	}
	if !user.IsActive {
		return nil, ErrAccountDisabled
	}
	s.enforceConcurrentLimit(user.ID)
	return s.GenerateTokenPair(user, ip, ua)
}

func (s *AuthService) Logout(refreshToken string) error {
	return s.refreshTokenRepo.Revoke(refreshToken)
}

func (s *AuthService) RefreshTokens(oldRefreshToken, ip, ua string) (*TokenPair, error) {
	// Try to find the token (regardless of status) for reuse detection
	rt, err := s.refreshTokenRepo.FindByToken(oldRefreshToken)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInvalidRefreshToken
		}
		return nil, err
	}

	// Reuse detection: if already revoked, revoke all tokens for this user
	if rt.Revoked {
		_ = s.refreshTokenRepo.RevokeAllForUser(rt.UserID)
		return nil, ErrTokenReuse
	}

	// Check expiry
	if time.Now().After(rt.ExpiresAt) {
		return nil, ErrRefreshTokenExpired
	}

	// Revoke old token (rotation)
	if err := s.refreshTokenRepo.Revoke(oldRefreshToken); err != nil {
		return nil, err
	}

	// Load user
	user, err := s.userRepo.FindByID(rt.UserID)
	if err != nil {
		return nil, err
	}

	if !user.IsActive {
		return nil, ErrAccountDisabled
	}

	return s.GenerateTokenPair(user, ip, ua)
}

func (s *AuthService) GetCurrentUser(userID uint) (*model.User, error) {
	return s.userRepo.FindByID(userID)
}

// GetUserConnections returns all OAuth connections for a user.
func (s *AuthService) GetUserConnections(userID uint) ([]model.UserConnection, error) {
	return s.connRepo.FindByUserID(userID)
}

// OAuthLogin handles OAuth login: find existing connection or create new user.
func (s *AuthService) OAuthLogin(provider, externalID, externalName, externalEmail, avatarURL, ip, ua string) (*TokenPair, error) {
	// Check if connection already exists
	conn, err := s.connRepo.FindByProviderAndExternalID(provider, externalID)
	if err == nil {
		// Existing connection — load user
		user, err := s.userRepo.FindByID(conn.UserID)
		if err != nil {
			return nil, err
		}
		if !user.IsActive {
			return nil, ErrAccountDisabled
		}

		// Update external info if changed
		changed := false
		if conn.ExternalName != externalName {
			conn.ExternalName = externalName
			changed = true
		}
		if conn.ExternalEmail != externalEmail {
			conn.ExternalEmail = externalEmail
			changed = true
		}
		if conn.AvatarURL != avatarURL {
			conn.AvatarURL = avatarURL
			changed = true
		}
		if changed {
			_ = s.connRepo.Update(conn)
		}

		s.enforceConcurrentLimit(user.ID)
		return s.GenerateTokenPair(user, ip, ua)
	}

	// New connection — check email conflict
	if externalEmail != "" {
		if existing, err := s.userRepo.FindByEmail(externalEmail); err == nil && existing != nil {
			return nil, ErrEmailConflict
		}
	}

	// Find default "user" role
	userRole, err := s.roleRepo.FindByCode(model.RoleUser)
	if err != nil {
		return nil, fmt.Errorf("find default role: %w", err)
	}

	// Create new user with auto-generated username
	username := fmt.Sprintf("%s_%s", provider, externalID)
	user := &model.User{
		Username: username,
		Email:    externalEmail,
		Avatar:   avatarURL,
		RoleID:   userRole.ID,
		IsActive: true,
	}
	if err := s.userRepo.Create(user); err != nil {
		return nil, fmt.Errorf("create oauth user: %w", err)
	}

	// Reload user with role preloaded
	user, err = s.userRepo.FindByID(user.ID)
	if err != nil {
		return nil, err
	}

	// Create connection
	newConn := &model.UserConnection{
		UserID:        user.ID,
		Provider:      provider,
		ExternalID:    externalID,
		ExternalName:  externalName,
		ExternalEmail: externalEmail,
		AvatarURL:     avatarURL,
	}
	if err := s.connRepo.Create(newConn); err != nil {
		return nil, fmt.Errorf("create connection: %w", err)
	}

	return s.GenerateTokenPair(user, ip, ua)
}

func (s *AuthService) ChangePassword(userID uint, oldPassword, newPassword string) error {
	// Validate password policy
	if violations := token.ValidatePassword(newPassword, s.settingsSvc.GetPasswordPolicy()); len(violations) > 0 {
		return fmt.Errorf("%w: %s", ErrPasswordViolation, violations[0])
	}

	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return err
	}

	if !token.CheckPassword(user.Password, oldPassword) {
		return ErrOldPasswordWrong
	}

	hashed, err := token.HashPassword(newPassword)
	if err != nil {
		return err
	}
	now := time.Now()
	user.Password = hashed
	user.PasswordChangedAt = &now
	user.ForcePasswordReset = false
	if err := s.userRepo.Update(user); err != nil {
		return err
	}

	// Blacklist all active access tokens for this user before revoking
	jtis, _ := s.refreshTokenRepo.GetActiveTokenJTIsByUserID(userID)
	for _, jti := range jtis {
		s.blacklist.Add(jti, time.Now().Add(token.AccessTokenDuration))
	}

	// Revoke all refresh tokens after password change
	return s.refreshTokenRepo.RevokeAllForUser(userID)
}

// GenerateTokenPair creates JWT access + refresh tokens for the given user.
// Exported so that pluggable Apps (e.g., identity) can issue tokens after external auth.
func (s *AuthService) GenerateTokenPair(user *model.User, ip, ua string) (*TokenPair, error) {
	accessToken, claims, err := token.GenerateAccessToken(
		user.ID, user.Role.Code, s.jwtSecret,
		token.WithPasswordMeta(user.PasswordChangedAt, user.ForcePasswordReset),
	)
	if err != nil {
		return nil, err
	}

	refreshTokenStr, err := token.GenerateRefreshToken()
	if err != nil {
		return nil, err
	}

	now := time.Now()
	refreshDuration := time.Duration(s.settingsSvc.GetSessionTimeoutMinutes()) * time.Minute
	rt := &model.RefreshToken{
		Token:          refreshTokenStr,
		UserID:         user.ID,
		ExpiresAt:      now.Add(refreshDuration),
		IPAddress:      ip,
		UserAgent:      ua,
		LastSeenAt:     now,
		AccessTokenJTI: claims.ID,
	}
	if err := s.refreshTokenRepo.Create(rt); err != nil {
		return nil, err
	}

	permissions := s.menuSvc.GetUserPermissions(user.Role.Code)

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshTokenStr,
		ExpiresIn:    int64(token.AccessTokenDuration.Seconds()),
		Permissions:  permissions,
		UserID:       user.ID,
	}, nil
}

// GenerateTokenPairByID loads a user by ID and generates a token pair.
// Used by 2FA login flow after verifying the TOTP code.
func (s *AuthService) GenerateTokenPairByID(userID uint, ip, ua string) (*TokenPair, error) {
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return nil, err
	}
	if !user.IsActive {
		return nil, ErrAccountDisabled
	}
	s.enforceConcurrentLimit(user.ID)
	return s.GenerateTokenPair(user, ip, ua)
}

// handleFailedLogin increments the failed attempt counter and locks the account if threshold reached.
func (s *AuthService) handleFailedLogin(userID uint) {
	if err := s.userRepo.IncrementFailedAttempts(userID); err != nil {
		slog.Error("failed to increment login attempts", "userId", userID, "error", err)
		return
	}

	maxAttempts, lockoutMinutes := s.settingsSvc.GetLoginLockoutSettings()
	if maxAttempts <= 0 {
		return // lockout disabled
	}

	attempts, err := s.userRepo.GetFailedAttempts(userID)
	if err != nil {
		slog.Error("failed to get login attempts", "userId", userID, "error", err)
		return
	}

	if attempts >= maxAttempts {
		duration := time.Duration(lockoutMinutes) * time.Minute
		if err := s.userRepo.LockUser(userID, duration); err != nil {
			slog.Error("failed to lock user", "userId", userID, "error", err)
		}
	}
}

// enforceConcurrentLimit checks and enforces the max concurrent sessions per user.
func (s *AuthService) enforceConcurrentLimit(userID uint) {
	maxSessions := s.getMaxConcurrentSessions()
	if maxSessions <= 0 {
		return
	}

	active, err := s.refreshTokenRepo.GetActiveByUserID(userID)
	if err != nil || len(active) < maxSessions {
		return
	}

	// Kick oldest sessions to make room for the new one
	excess := len(active) - maxSessions + 1
	for i := 0; i < excess && i < len(active); i++ {
		rt := active[i]
		_ = s.refreshTokenRepo.RevokeByID(rt.ID)
		if rt.AccessTokenJTI != "" {
			s.blacklist.Add(rt.AccessTokenJTI, time.Now().Add(token.AccessTokenDuration))
		}
	}
}

func (s *AuthService) getMaxConcurrentSessions() int {
	cfg, err := s.sysConfigRepo.Get("security.max_concurrent_sessions")
	if err != nil {
		return 5 // default
	}
	v, err := strconv.Atoi(cfg.Value)
	if err != nil {
		return 5
	}
	if v < 0 {
		return 5
	}
	return v
}

// BlacklistUserTokens blacklists all active access tokens for a user (used by session kick).
func (s *AuthService) BlacklistUserTokens(userID uint) {
	jtis, err := s.refreshTokenRepo.GetActiveTokenJTIsByUserID(userID)
	if err != nil {
		slog.Error("failed to get active JTIs", "userId", userID, "error", err)
		return
	}
	for _, jti := range jtis {
		s.blacklist.Add(jti, time.Now().Add(token.AccessTokenDuration))
	}
}

// Register creates a new user account via self-registration.
func (s *AuthService) Register(username, password, email, ip, ua string) (*TokenPair, error) {
	if !s.settingsSvc.IsRegistrationOpen() {
		return nil, ErrRegistrationClosed
	}

	// Validate password policy
	if violations := token.ValidatePassword(password, s.settingsSvc.GetPasswordPolicy()); len(violations) > 0 {
		return nil, fmt.Errorf("%w: %s", ErrPasswordViolation, violations[0])
	}

	// Check username uniqueness
	exists, err := s.userRepo.ExistsByUsername(username)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrUsernameExists
	}

	// Find default role
	roleCode := s.settingsSvc.GetDefaultRoleCode()
	if roleCode == "" {
		roleCode = model.RoleUser
	}
	role, err := s.roleRepo.FindByCode(roleCode)
	if err != nil {
		return nil, ErrDefaultRoleNotFound
	}

	hashed, err := token.HashPassword(password)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	user := &model.User{
		Username:          username,
		Password:          hashed,
		Email:             email,
		RoleID:            role.ID,
		IsActive:          true,
		PasswordChangedAt: &now,
	}
	if err := s.userRepo.Create(user); err != nil {
		return nil, err
	}

	// Reload with Role association
	user, err = s.userRepo.FindByID(user.ID)
	if err != nil {
		return nil, err
	}

	return s.GenerateTokenPair(user, ip, ua)
}

// IsRegistrationOpen returns whether self-registration is enabled.
func (s *AuthService) IsRegistrationOpen() bool {
	return s.settingsSvc.IsRegistrationOpen()
}

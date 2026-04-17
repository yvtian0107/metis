package handler

import (
	"github.com/casbin/casbin/v2"
	"github.com/gin-gonic/gin"
	"github.com/samber/do/v2"

	"metis/internal/app"
	"metis/internal/middleware"
	"metis/internal/pkg/identity"
	"metis/internal/pkg/oauth"
	"metis/internal/pkg/token"
	"metis/internal/repository"
	"metis/internal/service"
)

type Handler struct {
	sysCfg       *service.SysConfigService
	settingsSvc  *service.SettingsService
	auditSvc     *service.AuditLogService
	auth         *AuthHandler
	authProvider *AuthProviderHandler
	captcha      *CaptchaHandler
	user         *UserHandler
	role         *RoleHandler
	menu         *MenuHandler
	task         *TaskHandler
	session      *SessionHandler
	notification *NotificationHandler
	announcement *AnnouncementHandler
	channel      *ChannelHandler
	auditLog       *AuditLogHandler
	twoFactor      *TwoFactorHandler
	identitySource *IdentitySourceHandler
	sso            *SSOHandler
	// DataScope dependencies
	orgResolver app.OrgResolver
	roleRepo    *repository.RoleRepo
}

func New(i do.Injector) (*Handler, error) {
	sysCfg := do.MustInvoke[*service.SysConfigService](i)
	authSvc := do.MustInvoke[*service.AuthService](i)
	userSvc := do.MustInvoke[*service.UserService](i)
	roleSvc := do.MustInvoke[*service.RoleService](i)
	menuSvc := do.MustInvoke[*service.MenuService](i)
	casbinSvc := do.MustInvoke[*service.CasbinService](i)
	taskSvc := do.MustInvoke[*service.TaskService](i)
	sessionSvc := do.MustInvoke[*service.SessionService](i)
	settingsSvc := do.MustInvoke[*service.SettingsService](i)
	notifSvc := do.MustInvoke[*service.NotificationService](i)
	channelSvc := do.MustInvoke[*service.MessageChannelService](i)
	providerSvc := do.MustInvoke[*service.AuthProviderService](i)
	connSvc := do.MustInvoke[*service.UserConnectionService](i)
	connRepo := do.MustInvoke[*repository.UserConnectionRepo](i)
	roleRepo := do.MustInvoke[*repository.RoleRepo](i)
	stateMgr := do.MustInvoke[*oauth.StateManager](i)
	auditSvc := do.MustInvoke[*service.AuditLogService](i)
	captchaSvc := do.MustInvoke[*service.CaptchaService](i)
	tfSvc := do.MustInvoke[*service.TwoFactorService](i)
	identitySvc := do.MustInvoke[*service.IdentitySourceService](i)
	jwtSecret := do.MustInvoke[[]byte](i)

	// OrgResolver is optional (nil when Org App not installed)
	orgResolver, _ := do.InvokeAs[app.OrgResolver](i)

	return &Handler{
		sysCfg:      sysCfg,
		settingsSvc: settingsSvc,
		auditSvc:    auditSvc,
		auth: &AuthHandler{
			auth:        authSvc,
			userSvc:     userSvc,
			menuSvc:     menuSvc,
			providerSvc: providerSvc,
			connSvc:     connSvc,
			stateMgr:    stateMgr,
			auditSvc:    auditSvc,
		},
		authProvider: &AuthProviderHandler{svc: providerSvc},
		captcha:      &CaptchaHandler{captchaSvc: captchaSvc, settingsSvc: settingsSvc},
		user:         &UserHandler{userSvc: userSvc, connRepo: connRepo},
		role:         &RoleHandler{roleSvc: roleSvc, casbinSvc: casbinSvc, menuSvc: menuSvc, roleRepo: roleRepo},
		menu:         &MenuHandler{menuSvc: menuSvc},
		task:         &TaskHandler{svc: taskSvc},
		session:      &SessionHandler{sessionSvc: sessionSvc},
		notification: &NotificationHandler{notifSvc: notifSvc},
		announcement: &AnnouncementHandler{notifSvc: notifSvc},
		channel:      &ChannelHandler{svc: channelSvc},
		auditLog:     &AuditLogHandler{auditSvc: auditSvc},
		twoFactor:      &TwoFactorHandler{tfSvc: tfSvc, authSvc: authSvc, jwtSecret: jwtSecret},
		identitySource: &IdentitySourceHandler{svc: identitySvc},
		sso: &SSOHandler{
			svc:      identitySvc,
			authSvc:  authSvc,
			stateMgr: identity.NewSSOStateManager(),
		},
		orgResolver: orgResolver,
		roleRepo:    roleRepo,
	}, nil
}

func (h *Handler) Register(r *gin.Engine, jwtSecret []byte, enforcer *casbin.Enforcer, blacklist *token.TokenBlacklist) *gin.RouterGroup {
	v1 := r.Group("/api/v1")

	// Public routes (no auth required)
	v1.POST("/auth/login", h.auth.Login)
	v1.POST("/auth/refresh", h.auth.Refresh)
	v1.GET("/auth/providers", h.auth.ListProviders)
	v1.GET("/auth/oauth/:provider", h.auth.InitiateOAuth)
	v1.POST("/auth/oauth/callback", h.auth.OAuthCallback)
	v1.GET("/site-info", h.GetSiteInfo)
	v1.GET("/site-info/logo", h.GetLogo)
	v1.GET("/captcha", h.captcha.Generate)
	v1.POST("/auth/2fa/login", h.twoFactor.Login)
	v1.POST("/auth/register", h.auth.Register)
	v1.GET("/auth/registration-status", h.auth.RegistrationStatus)
	v1.GET("/auth/check-domain", h.sso.CheckDomain)
	v1.GET("/auth/sso/:id/authorize", h.sso.InitiateSSO)
	v1.POST("/auth/sso/callback", h.sso.SSOCallback)

	// Authenticated routes with Casbin permission checking
	authed := v1.Group("")
	authed.Use(middleware.JWTAuth(jwtSecret, blacklist))
	authed.Use(middleware.PasswordExpiry(h.settingsSvc.GetPasswordExpiryDays))
	authed.Use(middleware.CasbinAuth(enforcer))
	authed.Use(middleware.DataScopeMiddleware(h.orgResolver, h.roleRepo.GetScopeByCode))
	authed.Use(middleware.Audit(h.auditSvc))
	{
		// Auth routes (whitelisted in CasbinAuth)
		authed.POST("/auth/logout", h.auth.Logout)
		authed.GET("/auth/me", h.auth.GetMe)
		authed.PUT("/auth/password", h.auth.ChangePassword)
		authed.PUT("/auth/profile", h.auth.UpdateProfile)

		// 2FA management (whitelisted in CasbinAuth)
		authed.POST("/auth/2fa/setup", h.twoFactor.Setup)
		authed.POST("/auth/2fa/confirm", h.twoFactor.Confirm)
		authed.DELETE("/auth/2fa", h.twoFactor.Disable)

		// OAuth connection management (whitelisted in CasbinAuth)
		authed.GET("/auth/connections", h.auth.ListConnections)
		authed.POST("/auth/connections/:provider", h.auth.InitiateBind)
		authed.POST("/auth/connections/callback", h.auth.BindCallback)
		authed.DELETE("/auth/connections/:provider", h.auth.Unbind)

		// User menu tree (whitelisted in CasbinAuth)
		authed.GET("/menus/user-tree", h.menu.GetUserTree)

		// Session management
		authed.GET("/sessions", h.session.List)
		authed.DELETE("/sessions/:id", h.session.Kick)

		// Settings routes (typed, replace generic config CRUD)
		authed.GET("/settings/security", h.GetSecuritySettings)
		authed.PUT("/settings/security", h.UpdateSecuritySettings)
		authed.GET("/settings/scheduler", h.GetSchedulerSettings)
		authed.PUT("/settings/scheduler", h.UpdateSchedulerSettings)

		// Site info management
		authed.PUT("/site-info", h.UpdateSiteInfo)
		authed.PUT("/site-info/logo", h.UploadLogo)
		authed.DELETE("/site-info/logo", h.DeleteLogo)

		// User management
		authed.GET("/users", h.user.List)
		authed.POST("/users", h.user.Create)
		authed.GET("/users/:id", h.user.Get)
		authed.PUT("/users/:id", h.user.Update)
		authed.DELETE("/users/:id", h.user.Delete)
		authed.POST("/users/:id/reset-password", h.user.ResetPassword)
		authed.POST("/users/:id/activate", h.user.Activate)
		authed.POST("/users/:id/deactivate", h.user.Deactivate)
		authed.POST("/users/:id/unlock", h.user.Unlock)
		authed.GET("/users/:id/manager-chain", h.user.GetManagerChain)

		// Role management
		authed.GET("/roles", h.role.List)
		authed.POST("/roles", h.role.Create)
		authed.GET("/roles/:id", h.role.Get)
		authed.PUT("/roles/:id", h.role.Update)
		authed.DELETE("/roles/:id", h.role.Delete)
		authed.GET("/roles/:id/permissions", h.role.GetPermissions)
		authed.PUT("/roles/:id/permissions", h.role.SetPermissions)
		authed.PUT("/roles/:id/data-scope", h.role.UpdateDataScope)

		// Menu management
		authed.GET("/menus/tree", h.menu.GetTree)
		authed.PUT("/menus/sort", h.menu.Reorder)
		authed.POST("/menus", h.menu.Create)
		authed.PUT("/menus/:id", h.menu.Update)
		authed.DELETE("/menus/:id", h.menu.Delete)

		// Task management
		authed.GET("/tasks", h.task.ListTasks)
		authed.GET("/tasks/stats", h.task.GetStats)
		authed.GET("/tasks/:name", h.task.GetTask)
		authed.GET("/tasks/:name/executions", h.task.ListExecutions)
		authed.POST("/tasks/:name/pause", h.task.PauseTask)
		authed.POST("/tasks/:name/resume", h.task.ResumeTask)
		authed.POST("/tasks/:name/trigger", h.task.TriggerTask)

		// Notification center (whitelisted in CasbinAuth — any logged-in user)
		authed.GET("/notifications", h.notification.List)
		authed.GET("/notifications/unread-count", h.notification.GetUnreadCount)
		authed.PUT("/notifications/:id/read", h.notification.MarkAsRead)
		authed.PUT("/notifications/read-all", h.notification.MarkAllAsRead)

		// Announcement management (Casbin-protected)
		authed.GET("/announcements", h.announcement.List)
		authed.POST("/announcements", h.announcement.Create)
		authed.PUT("/announcements/:id", h.announcement.Update)
		authed.DELETE("/announcements/:id", h.announcement.Delete)

		// Message channel management (Casbin-protected)
		authed.GET("/channels", h.channel.List)
		authed.POST("/channels", h.channel.Create)
		authed.GET("/channels/:id", h.channel.Get)
		authed.PUT("/channels/:id", h.channel.Update)
		authed.DELETE("/channels/:id", h.channel.Delete)
		authed.PUT("/channels/:id/toggle", h.channel.Toggle)
		authed.POST("/channels/:id/test", h.channel.Test)
		authed.POST("/channels/:id/send-test", h.channel.SendTest)

		// Auth provider management (admin only, Casbin-protected)
		authed.GET("/admin/auth-providers", h.authProvider.ListAll)
		authed.PUT("/admin/auth-providers/:key", h.authProvider.Update)
		authed.PATCH("/admin/auth-providers/:key/toggle", h.authProvider.Toggle)

		// Audit logs (read-only, Casbin-protected)
		authed.GET("/audit-logs", h.auditLog.List)

		// Identity sources (Casbin-protected)
		authed.GET("/identity-sources", h.identitySource.List)
		authed.POST("/identity-sources", h.identitySource.Create)
		authed.PUT("/identity-sources/:id", h.identitySource.Update)
		authed.DELETE("/identity-sources/:id", h.identitySource.Delete)
		authed.PATCH("/identity-sources/:id/toggle", h.identitySource.Toggle)
		authed.POST("/identity-sources/:id/test", h.identitySource.TestConnection)
	}

	return authed
}

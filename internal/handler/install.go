package handler

import (
	"context"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/casbin/casbin/v2"
	"github.com/gin-gonic/gin"
	"github.com/samber/do/v2"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"gorm.io/gorm"

	casbinpkg "metis/internal/casbin"
	"metis/internal/config"
	"metis/internal/database"
	"metis/internal/model"
	"metis/internal/pkg/oauth"
	"metis/internal/pkg/token"
	"metis/internal/repository"
	"metis/internal/scheduler"
	"metis/internal/seed"
	"metis/internal/service"
	"metis/internal/telemetry"

	"metis/internal/app"
)

// InstallHandler handles the installation wizard API.
type InstallHandler struct {
	db       *database.DB
	injector do.Injector
	engine   *gin.Engine
}

// NewInstall creates an InstallHandler.
func NewInstall(db *database.DB, injector do.Injector, engine *gin.Engine) *InstallHandler {
	return &InstallHandler{db: db, injector: injector, engine: engine}
}

// RegisterInstallRoutes registers install-only routes on the Gin engine.
func (h *InstallHandler) RegisterInstallRoutes(r *gin.Engine) {
	v1 := r.Group("/api/v1")
	v1.GET("/install/status", h.Status)
	v1.POST("/install/check-db", h.CheckDB)
	v1.POST("/install/execute", h.Execute)
}

// Status returns the installation state.
func (h *InstallHandler) Status(c *gin.Context) {
	installed := seed.IsInstalled(h.db.DB)
	OK(c, gin.H{"installed": installed})
}

// CheckDBRequest is the request body for testing a database connection.
type CheckDBRequest struct {
	Driver   string `json:"driver" binding:"required"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	DBName   string `json:"dbname"`
}

// CheckDB tests a database connection.
func (h *InstallHandler) CheckDB(c *gin.Context) {
	if seed.IsInstalled(h.db.DB) {
		Fail(c, http.StatusForbidden, "system already installed")
		return
	}

	var req CheckDBRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	if req.Driver != "postgres" {
		OK(c, gin.H{"success": true})
		return
	}

	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		req.Host, req.Port, req.User, req.Password, req.DBName)

	testDB, err := database.Open("postgres", dsn)
	if err != nil {
		OK(c, gin.H{"success": false, "error": err.Error()})
		return
	}
	testDB.Shutdown()

	OK(c, gin.H{"success": true})
}

// ExecuteRequest is the request body for the installation.
type ExecuteRequest struct {
	// Database
	DBDriver   string `json:"db_driver" binding:"required"`
	DBHost     string `json:"db_host"`
	DBPort     int    `json:"db_port"`
	DBUser     string `json:"db_user"`
	DBPassword string `json:"db_password"`
	DBName     string `json:"db_name"`

	// Site
	SiteName string `json:"site_name" binding:"required"`

	// Admin
	AdminUsername string `json:"admin_username" binding:"required"`
	AdminPassword string `json:"admin_password" binding:"required,min=8"`
	AdminEmail    string `json:"admin_email" binding:"required,email"`
}

// Execute performs the full installation.
func (h *InstallHandler) Execute(c *gin.Context) {
	if seed.IsInstalled(h.db.DB) {
		Fail(c, http.StatusForbidden, "system already installed")
		return
	}

	var req ExecuteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	// 1. Build config
	cfg := &config.MetisConfig{
		DBDriver: req.DBDriver,
	}

	if req.DBDriver == "postgres" {
		cfg.DBDSN = fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
			req.DBHost, req.DBPort, req.DBUser, req.DBPassword, req.DBName)
	} else {
		cfg.DBDSN = "metis.db?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)"
	}

	// 2. Generate secrets
	if err := cfg.GenerateSecrets(); err != nil {
		Fail(c, http.StatusInternalServerError, "failed to generate secrets: "+err.Error())
		return
	}

	// 3. If postgres, switch to the new DB
	var db *database.DB
	if req.DBDriver == "postgres" {
		var err error
		db, err = database.Open("postgres", cfg.DBDSN)
		if err != nil {
			Fail(c, http.StatusBadRequest, "database connection failed: "+err.Error())
			return
		}
	} else {
		db = h.db // reuse existing SQLite connection
	}

	// 4. AutoMigrate
	if err := database.AutoMigrateKernel(db.DB); err != nil {
		Fail(c, http.StatusInternalServerError, "database migration failed: "+err.Error())
		return
	}

	// Migrate app models
	for _, a := range app.All() {
		if models := a.Models(); len(models) > 0 {
			if err := db.DB.AutoMigrate(models...); err != nil {
				Fail(c, http.StatusInternalServerError, fmt.Sprintf("app %s migration failed: %s", a.Name(), err.Error()))
				return
			}
		}
	}

	// 5. Init Casbin enforcer
	enforcer, err := casbinpkg.NewEnforcerWithDB(db.DB)
	if err != nil {
		Fail(c, http.StatusInternalServerError, "casbin init failed: "+err.Error())
		return
	}

	// 6. Seed
	result, err := seed.Install(db.DB, enforcer)
	if err != nil {
		Fail(c, http.StatusInternalServerError, "seed failed: "+err.Error())
		return
	}
	slog.Info("install: seed complete",
		"roles_created", result.RolesCreated,
		"menus_created", result.MenusCreated,
		"policies_added", result.PoliciesAdded,
	)

	// 7. Set site name
	if err := seed.SetSiteName(db.DB, req.SiteName); err != nil {
		Fail(c, http.StatusInternalServerError, "failed to set site name: "+err.Error())
		return
	}

	// 8. Create admin user
	adminRole, err := findAdminRole(db.DB)
	if err != nil {
		Fail(c, http.StatusInternalServerError, "admin role not found: "+err.Error())
		return
	}

	hashedPassword, err := token.HashPassword(req.AdminPassword)
	if err != nil {
		Fail(c, http.StatusInternalServerError, "failed to hash password: "+err.Error())
		return
	}

	admin := &model.User{
		Username: req.AdminUsername,
		Password: hashedPassword,
		Email:    req.AdminEmail,
		RoleID:   adminRole.ID,
		IsActive: true,
	}
	if err := db.DB.Create(admin).Error; err != nil {
		Fail(c, http.StatusInternalServerError, "failed to create admin: "+err.Error())
		return
	}

	// 9. Mark installed
	if err := seed.SetInstalled(db.DB); err != nil {
		Fail(c, http.StatusInternalServerError, "failed to mark installed: "+err.Error())
		return
	}

	// 10. Write metis.yaml
	if err := cfg.Save("metis.yaml"); err != nil {
		Fail(c, http.StatusInternalServerError, "failed to write config: "+err.Error())
		return
	}

	// 11. Hot switch: register all business services and routes
	if err := h.hotSwitch(cfg, db, enforcer); err != nil {
		slog.Error("install: hot switch failed", "error", err)
		// Installation is complete but hot switch failed — user needs to restart
		OK(c, gin.H{"restart_required": true})
		return
	}

	OK(c, nil)
}

func (h *InstallHandler) hotSwitch(cfg *config.MetisConfig, db *database.DB, enforcer *casbin.Enforcer) error {
	injector := h.injector

	// Provide the config and new DB to IOC
	do.ProvideValue(injector, cfg)
	if db != h.db {
		// PostgreSQL: override the DB in the container
		do.OverrideValue(injector, db)
	}

	// JWT secret from config
	jwtSecret, err := hex.DecodeString(cfg.JWTSecret)
	if err != nil {
		return fmt.Errorf("decode jwt_secret: %w", err)
	}
	do.OverrideValue(injector, jwtSecret)

	blacklist := do.MustInvoke[*token.TokenBlacklist](injector)

	// Register remaining kernel providers
	do.Provide(injector, casbinpkg.NewEnforcer)
	do.Provide(injector, repository.NewUser)
	do.Provide(injector, repository.NewRefreshToken)
	do.Provide(injector, repository.NewRole)
	do.Provide(injector, repository.NewMenu)
	do.Provide(injector, repository.NewNotification)
	do.Provide(injector, repository.NewMessageChannel)
	do.Provide(injector, repository.NewAuthProvider)
	do.Provide(injector, repository.NewUserConnection)
	do.Provide(injector, repository.NewAuditLog)
	do.Provide(injector, repository.NewTwoFactorSecret)
	do.Provide(injector, service.NewCasbin)
	do.Provide(injector, service.NewRole)
	do.Provide(injector, service.NewMenu)
	do.Provide(injector, service.NewAuth)
	do.Provide(injector, service.NewUser)
	do.Provide(injector, service.NewNotification)
	do.Provide(injector, service.NewMessageChannel)
	do.Provide(injector, service.NewSession)
	do.Provide(injector, service.NewSettings)
	do.Provide(injector, service.NewAuthProvider)
	do.Provide(injector, service.NewUserConnection)
	do.Provide(injector, service.NewAuditLog)
	do.Provide(injector, service.NewCaptcha)
	do.Provide(injector, service.NewTwoFactor)
	do.Provide(injector, repository.NewIdentitySource)
	do.Provide(injector, service.NewIdentitySource)
	do.ProvideValue(injector, oauth.NewStateManager())
	do.Provide(injector, New)
	do.Provide(injector, scheduler.New)

	// Boot apps
	for _, a := range app.All() {
		a.Providers(injector)
		if err := a.Seed(db.DB, enforcer); err != nil {
			slog.Error("install: app seed failed", "app", a.Name(), "error", err)
		}
	}

	// Resolve handler
	h2 := do.MustInvoke[*Handler](injector)

	// OTel from DB
	sysConfigRepo := do.MustInvoke[*repository.SysConfigRepo](injector)
	otelCfg := telemetry.LoadOTelConfigFromDB(func(key string) string {
		cfg, err := sysConfigRepo.Get(key)
		if err != nil {
			return ""
		}
		return cfg.Value
	})
	telemetry.Init(context.Background(), otelCfg)

	// Register business routes
	h.engine.Use(otelgin.Middleware("metis"))
	authedGroup := h2.Register(h.engine, jwtSecret, enforcer, blacklist)

	for _, a := range app.All() {
		a.Routes(authedGroup)
	}

	// Start scheduler
	engine := do.MustInvoke[*scheduler.Engine](injector)

	scheduler.SetCleanupHandler(
		scheduler.HistoryCleanupTask,
		func(key string) (string, error) {
			cfg, err := sysConfigRepo.Get(key)
			if err != nil {
				return "", err
			}
			return cfg.Value, nil
		},
		engine.GetStore().(*scheduler.GormStore),
	)
	engine.Register(scheduler.HistoryCleanupTask)

	scheduler.SetBlacklistCleanupHandler(scheduler.BlacklistCleanupTask, blacklist.Cleanup)
	engine.Register(scheduler.BlacklistCleanupTask)

	refreshTokenRepo := do.MustInvoke[*repository.RefreshTokenRepo](injector)
	scheduler.SetExpiredTokenCleanupHandler(scheduler.ExpiredTokenCleanupTask, refreshTokenRepo.DeleteExpiredTokens)
	engine.Register(scheduler.ExpiredTokenCleanupTask)

	auditLogSvc := do.MustInvoke[*service.AuditLogService](injector)
	scheduler.SetAuditLogCleanupHandler(scheduler.AuditLogCleanupTask, auditLogSvc.Cleanup)
	engine.Register(scheduler.AuditLogCleanupTask)

	for _, a := range app.All() {
		for _, t := range a.Tasks() {
			engine.Register(&t)
		}
	}

	if err := engine.Start(); err != nil {
		return fmt.Errorf("scheduler start: %w", err)
	}

	slog.Info("install: hot switch complete — system is now fully operational")
	return nil
}

func findAdminRole(db *gorm.DB) (*model.Role, error) {
	var role model.Role
	if err := db.Where("code = ?", model.RoleAdmin).First(&role).Error; err != nil {
		return nil, err
	}
	return &role, nil
}

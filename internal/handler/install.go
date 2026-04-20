package handler

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/casbin/casbin/v2"
	"github.com/gin-gonic/gin"
	"github.com/samber/do/v2"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"gorm.io/gorm"

	casbinpkg "metis/internal/casbin"
	"metis/internal/config"
	"metis/internal/database"
	"metis/internal/model"
	"metis/internal/pkg/crypto"
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
	injector          do.Injector
	engine            *gin.Engine
	overrideProviders func(do.Injector)
	configPath        string
	installed         bool
}

// NewInstall creates an InstallHandler.
func NewInstall(injector do.Injector, engine *gin.Engine, overrideProviders func(do.Injector)) *InstallHandler {
	return &InstallHandler{
		injector:          injector,
		engine:            engine,
		overrideProviders: overrideProviders,
		configPath:        do.MustInvokeNamed[string](injector, "configPath"),
	}
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
	OK(c, gin.H{"installed": h.installed})
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
	if h.installed {
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
	Locale   string `json:"locale"`
	Timezone string `json:"timezone"`

	// Admin
	AdminUsername string `json:"admin_username" binding:"required"`
	AdminPassword string `json:"admin_password" binding:"required,min=8"`
	AdminEmail    string `json:"admin_email" binding:"required,email"`
	// OTel (optional)
	OTelEnabled          *bool  `json:"otel_enabled"`
	OTelExporterEndpoint string `json:"otel_exporter_endpoint"`
	OTelServiceName      string `json:"otel_service_name"`
	OTelSampleRate       string `json:"otel_sample_rate"`

	// FalkorDB (optional — required for AI knowledge graph features)
	FalkorDBAddr     string `json:"falkordb_addr"`
	FalkorDBPassword string `json:"falkordb_password"`
	FalkorDBDatabase int    `json:"falkordb_database"`

	// ClickHouse (optional — required for APM features)
	ClickHouseDSN string `json:"clickhouse_dsn"`
}

// Execute performs the full installation.
func (h *InstallHandler) Execute(c *gin.Context) {
	if h.installed {
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

	// FalkorDB (optional)
	if req.FalkorDBAddr != "" {
		cfg.FalkorDB = &config.FalkorDBConfig{
			Addr:     req.FalkorDBAddr,
			Password: req.FalkorDBPassword,
			Database: req.FalkorDBDatabase,
		}
	}

	// ClickHouse (optional)
	if req.ClickHouseDSN != "" {
		cfg.ClickHouse = &config.ClickHouseConfig{
			DSN: req.ClickHouseDSN,
		}
	}

	// 2. Generate secrets
	if err := cfg.GenerateSecrets(); err != nil {
		Fail(c, http.StatusInternalServerError, "failed to generate secrets: "+err.Error())
		return
	}

	// 3. Open database connection based on user's choice
	db, err := database.Open(cfg.DBDriver, cfg.DBDSN)
	if err != nil {
		Fail(c, http.StatusBadRequest, "database connection failed: "+err.Error())
		return
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

	// Set locale and timezone
	if err := seed.SetLocaleTimezone(db.DB, req.Locale, req.Timezone); err != nil {
		Fail(c, http.StatusInternalServerError, "failed to set locale/timezone: "+err.Error())
		return
	}

	// Set OTel config if provided
	if req.OTelEnabled != nil {
		enabled := strconv.FormatBool(*req.OTelEnabled)
		endpoint := req.OTelExporterEndpoint
		if endpoint == "" {
			endpoint = "http://localhost:4318"
		}
		serviceName := req.OTelServiceName
		if serviceName == "" {
			serviceName = "metis"
		}
		sampleRate := req.OTelSampleRate
		if sampleRate == "" {
			sampleRate = "1.0"
		}
		if err := seed.SetOTelConfig(db.DB, enabled, endpoint, serviceName, sampleRate); err != nil {
			Fail(c, http.StatusInternalServerError, "failed to set otel config: "+err.Error())
			return
		}
	}

	// 8. Register IOC providers (needed for UserService and hot switch)
	do.OverrideValue(h.injector, cfg)
	do.OverrideValue(h.injector, db)
	do.Override(h.injector, repository.NewSysConfig)
	do.Override(h.injector, service.NewSysConfig)
	h.overrideProviders(h.injector)

	// 9. Create admin user via UserService
	adminRole, err := findAdminRole(db.DB)
	if err != nil {
		Fail(c, http.StatusInternalServerError, "admin role not found: "+err.Error())
		return
	}

	userSvc, err := do.Invoke[*service.UserService](h.injector)
	if err != nil {
		Fail(c, http.StatusInternalServerError, "failed to initialize user service: "+err.Error())
		return
	}
	if _, err := userSvc.Create(req.AdminUsername, req.AdminPassword, req.AdminEmail, "", adminRole.ID); err != nil {
		if errors.Is(err, service.ErrUsernameExists) {
			if err := upsertInstallAdmin(db.DB, req.AdminUsername, req.AdminPassword, req.AdminEmail, adminRole.ID); err != nil {
				Fail(c, http.StatusInternalServerError, "failed to reuse existing admin: "+err.Error())
				return
			}
		} else {
			Fail(c, http.StatusInternalServerError, "failed to create admin: "+err.Error())
			return
		}
	}
	// 9. Mark installed
	if err := seed.SetInstalled(db.DB); err != nil {
		Fail(c, http.StatusInternalServerError, "failed to mark installed: "+err.Error())
		return
	}

	// 10. Write config file
	if err := cfg.Save(h.configPath); err != nil {
		Fail(c, http.StatusInternalServerError, "failed to write config: "+err.Error())
		return
	}

	// Mark installed so Status endpoint returns true
	h.installed = true

	// 11. Hot switch: register all business services and routes
	if err := h.hotSwitch(cfg, db, enforcer, req.AdminUsername); err != nil {
		slog.Error("install: hot switch failed", "error", err)
		// Installation is complete but hot switch failed — user needs to restart
		OK(c, gin.H{"restart_required": true})
		return
	}

	OK(c, nil)
}

func assignInstallAdminOrgIdentity(db *gorm.DB, username string) error {
	var user struct{ ID uint }
	if err := db.Table("users").Where("username = ?", username).Select("id").First(&user).Error; err != nil {
		return err
	}

	type departmentRow struct{ ID uint }
	var dept departmentRow
	if err := db.Table("departments").Where("code = ?", "it").Select("id").First(&dept).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}

	type positionRow struct{ ID uint }
	var pos positionRow
	if err := db.Table("positions").Where("code = ?", "it_admin").Select("id").First(&pos).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}

	type userPositionRow struct{ ID uint }
	var existing userPositionRow
	if err := db.Table("user_positions").Where("user_id = ? AND department_id = ? AND position_id = ?", user.ID, dept.ID, pos.ID).Select("id").First(&existing).Error; err == nil {
		return nil
	}

	return db.Table("user_positions").Create(map[string]any{
		"user_id":       user.ID,
		"department_id": dept.ID,
		"position_id":   pos.ID,
		"is_primary":    true,
	}).Error
}

func upsertInstallAdmin(db *gorm.DB, username, password, email string, roleID uint) error {
	hashed, err := token.HashPassword(password)
	if err != nil {
		return err
	}

	now := time.Now()
	updates := map[string]any{
		"password":            hashed,
		"email":               email,
		"role_id":             roleID,
		"is_active":           true,
		"password_changed_at": &now,
	}

	result := db.Model(&model.User{}).Where("username = ?", username).Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (h *InstallHandler) hotSwitch(cfg *config.MetisConfig, db *database.DB, enforcer *casbin.Enforcer, adminUsername string) error {
	injector := h.injector

	// Config, DB, and kernel providers already registered in Execute

	// JWT secret from config
	jwtSecret, err := hex.DecodeString(cfg.JWTSecret)
	if err != nil {
		return fmt.Errorf("decode jwt_secret: %w", err)
	}
	do.OverrideValue(injector, jwtSecret)

	// Encryption key from secret_key
	do.OverrideValue(injector, crypto.EncryptionKey(crypto.DeriveKey(cfg.SecretKey)))

	blacklist := do.MustInvoke[*token.TokenBlacklist](injector)

	// Boot apps
	for _, a := range app.All() {
		a.Providers(injector)
		if err := a.Seed(db.DB, enforcer, true); err != nil {
			slog.Error("install: app seed failed", "app", a.Name(), "error", err)
		}
	}

	if err := assignInstallAdminOrgIdentity(db.DB, adminUsername); err != nil {
		return fmt.Errorf("assign install admin org identity: %w", err)
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

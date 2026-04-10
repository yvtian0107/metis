package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/casbin/casbin/v2"
	"github.com/gin-gonic/gin"
	"github.com/samber/do/v2"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"

	casbinpkg "metis/internal/casbin"
	"metis/internal/config"
	"metis/internal/database"
	"metis/internal/handler"
	"metis/internal/middleware"
	"metis/internal/pkg/oauth"
	"metis/internal/pkg/token"
	"metis/internal/repository"
	"metis/internal/scheduler"
	"metis/internal/seed"
	"metis/internal/service"
	"metis/internal/telemetry"

	"metis/internal/app"
)

func main() {
	// 1. Try to load metis.yaml
	cfg, err := config.Load("metis.yaml")
	if err != nil && !errors.Is(err, config.ErrConfigNotFound) {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// 2. IOC container
	injector := do.New()

	// If config exists, provide it to IOC so database.New can use it
	if cfg != nil {
		do.ProvideValue(injector, cfg)
	}

	// 3. Connect database (defaults to SQLite in install mode)
	do.Provide(injector, database.New)
	do.Provide(injector, repository.NewSysConfig)
	do.Provide(injector, service.NewSysConfig)

	db := do.MustInvoke[*database.DB](injector)

	// In install mode, ensure the system_configs table exists
	if cfg == nil {
		database.AutoMigrateKernel(db.DB)
	}

	// 4. Check installation state
	installed := cfg != nil && seed.IsInstalled(db.DB)

	// Gin engine
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(middleware.Logger(), middleware.Recovery())

	if !installed {
		// ──────────────────────────────────
		//  INSTALL MODE
		// ──────────────────────────────────
		slog.Info("system not installed — entering install mode")

		// Token blacklist (needed for hot switch)
		do.ProvideValue(injector, token.NewBlacklist())

		installHandler := handler.NewInstall(db, injector, r)
		installHandler.RegisterInstallRoutes(r)
		handler.RegisterStatic(r)

		// Also register install status on the same route used by normal mode
		// so the frontend can always check /api/v1/install/status
		startServer(r, "8080", injector, func(context.Context) {})
	} else {
		// ──────────────────────────────────
		//  NORMAL MODE
		// ──────────────────────────────────

		// JWT secret from config
		jwtSecret := []byte(cfg.JWTSecret)
		do.ProvideValue(injector, jwtSecret)

		// Token blacklist
		do.ProvideValue(injector, token.NewBlacklist())

		// Register all kernel providers
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
		do.Provide(injector, handler.New)
		do.Provide(injector, scheduler.New)

		// Resolve DB and enforcer
		enforcer := do.MustInvoke[*casbin.Enforcer](injector)

		// Incremental sync: add new roles/menus/policies from code
		if result, err := seed.Sync(db.DB, enforcer); err != nil {
			slog.Error("sync failed", "error", err)
			os.Exit(1)
		} else {
			slog.Info("seed sync complete",
				"roles_created", result.RolesCreated,
				"menus_created", result.MenusCreated,
				"policies_added", result.PoliciesAdded,
			)
		}

		// Boot pluggable Apps
		for _, a := range app.All() {
			if models := a.Models(); len(models) > 0 {
				if err := db.DB.AutoMigrate(models...); err != nil {
					slog.Error("app auto-migrate failed", "app", a.Name(), "error", err)
					os.Exit(1)
				}
			}
			a.Providers(injector)
			if err := a.Seed(db.DB, enforcer); err != nil {
				slog.Error("app seed failed", "app", a.Name(), "error", err)
				os.Exit(1)
			}
		}

		// Resolve handler
		h := do.MustInvoke[*handler.Handler](injector)
		blacklist := do.MustInvoke[*token.TokenBlacklist](injector)

		// Initialize OTel from DB config
		sysConfigRepo := do.MustInvoke[*repository.SysConfigRepo](injector)
		otelCfg := telemetry.LoadOTelConfigFromDB(func(key string) string {
			cfg, err := sysConfigRepo.Get(key)
			if err != nil {
				return ""
			}
			return cfg.Value
		})
		otelShutdown, err := telemetry.Init(context.Background(), otelCfg)
		if err != nil {
			slog.Error("opentelemetry init failed", "error", err)
			os.Exit(1)
		}

		// Initialize scheduler engine
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

		if err := engine.Start(); err != nil {
			slog.Error("scheduler start failed", "error", err)
			os.Exit(1)
		}

		// Routes
		r.Use(otelgin.Middleware("metis"))
		authedGroup := h.Register(r, jwtSecret, enforcer, blacklist)

		for _, a := range app.All() {
			a.Routes(authedGroup)
			for _, t := range a.Tasks() {
				engine.Register(&t)
			}
		}

		// Install status endpoint (always available)
		r.GET("/api/v1/install/status", func(c *gin.Context) {
			handler.OK(c, gin.H{"installed": true})
		})

		handler.RegisterStatic(r)

		// Read port from DB
		port := "8080"
		if portCfg, err := sysConfigRepo.Get("server_port"); err == nil && portCfg.Value != "" {
			port = portCfg.Value
		}

		startServer(r, port, injector, otelShutdown)
	}
}

func startServer(r *gin.Engine, port string, injector do.Injector, otelShutdown func(context.Context)) {
	srv := &http.Server{
		Addr:    ":" + port,
		Handler: r,
	}

	go func() {
		slog.Info("server starting", "port", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	slog.Info("shutting down", "signal", sig)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("server shutdown error", "error", err)
	}

	otelShutdown(ctx)

	report := injector.Shutdown()
	if errMsg := report.Error(); errMsg != "" {
		slog.Error("injector shutdown error", "error", errMsg)
	}

	slog.Info("server stopped")
}

package main

import (
	"context"
	"errors"
	"flag"
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

	"metis/internal/config"
	"metis/internal/database"
	"metis/internal/handler"
	"metis/internal/locales"
	"metis/internal/middleware"
	"metis/internal/pkg/crypto"
	"metis/internal/pkg/token"
	"metis/internal/repository"
	"metis/internal/scheduler"
	"metis/internal/seed"
	"metis/internal/service"
	"metis/internal/telemetry"

	"metis/internal/app"
)

func main() {
	configPath := flag.String("config", "config.yml", "path to config file")
	host := flag.String("host", "0.0.0.0", "server host")
	port := flag.String("port", "8000", "server port")
	flag.Parse()

	// 1. Try to load config
	cfg, err := config.Load(*configPath)
	if err != nil && !errors.Is(err, config.ErrConfigNotFound) {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// 2. IOC container
	injector := do.New()
	do.ProvideNamedValue(injector, "configPath", *configPath)

	// 3. Check installation state and connect database
	var db *database.DB
	installed := false

	if cfg != nil {
		// Config exists — provide to IOC and connect database
		do.ProvideValue(injector, cfg)
		do.Provide(injector, database.New)
		do.Provide(injector, repository.NewSysConfig)
		do.Provide(injector, service.NewSysConfig)
		db = do.MustInvoke[*database.DB](injector)
		installed = seed.IsInstalled(db.DB)
	}
	// If cfg == nil → first run, skip DB entirely; install wizard handles it

	// Gin engine
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(middleware.Logger(), middleware.Recovery())
	do.ProvideValue(injector, r)

	if !installed {
		// ──────────────────────────────────
		//  INSTALL MODE
		// ──────────────────────────────────
		slog.Info("system not installed — entering install mode")

		// Token blacklist (needed for hot switch)
		do.ProvideValue(injector, token.NewBlacklist())

		installHandler := handler.NewInstall(injector, r, overrideKernelProviders)
		installHandler.RegisterInstallRoutes(r)
		handler.RegisterStatic(r)

		// Also register install status on the same route used by normal mode
		// so the frontend can always check /api/v1/install/status
		startServer(r, *host, *port, injector, func(context.Context) {})
	} else {
		// ──────────────────────────────────
		//  NORMAL MODE
		// ──────────────────────────────────

		// JWT secret from config
		jwtSecret := []byte(cfg.JWTSecret)
		do.ProvideValue(injector, jwtSecret)

		// License key secret from config (for license private key encryption)
		licenseKeySecret := []byte(cfg.LicenseKeySecret)
		do.ProvideNamedValue(injector, "licenseKeySecret", licenseKeySecret)

		// Encryption key from secret_key (for API key encryption etc.)
		do.ProvideValue(injector, crypto.EncryptionKey(crypto.DeriveKey(cfg.SecretKey)))

		// Locale service
		localeSvc, err := locales.New()
		if err != nil {
			slog.Error("failed to init locale service", "error", err)
			os.Exit(1)
		}
		do.ProvideValue(injector, localeSvc)

		// Token blacklist
		do.ProvideValue(injector, token.NewBlacklist())

		// Register all kernel providers
		registerKernelProviders(injector)

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
			// Load app locale files if provided
			if lp, ok := a.(app.LocaleProvider); ok {
				if err := localeSvc.LoadAppLocales(lp.Locales()); err != nil {
					slog.Warn("failed to load app locales", "app", a.Name(), "error", err)
				}
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

		// Register app tasks BEFORE engine.Start() so they are included in state sync
		for _, a := range app.All() {
			for _, t := range a.Tasks() {
				engine.Register(&t)
			}
		}

		if err := engine.Start(); err != nil {
			slog.Error("scheduler start failed", "error", err)
			os.Exit(1)
		}

		// Routes
		r.Use(otelgin.Middleware("metis"))
		authedGroup := h.Register(r, jwtSecret, enforcer, blacklist)

		for _, a := range app.All() {
			a.Routes(authedGroup)
		}

		// Install status endpoint (always available)
		r.GET("/api/v1/install/status", func(c *gin.Context) {
			handler.OK(c, gin.H{"installed": true})
		})

		handler.RegisterStatic(r)

		// Read port from DB, CLI flag overrides DB config
		serverPort := *port
		if portCfg, err := sysConfigRepo.Get("server_port"); err == nil && portCfg.Value != "" {
			serverPort = portCfg.Value
		}
		// CLI port flag overrides DB config if explicitly set
		if p := flag.Lookup("port"); p != nil && p.Value.String() != "8000" {
			serverPort = *port
		}

		startServer(r, *host, serverPort, injector, otelShutdown)
	}
}

func startServer(r *gin.Engine, host, port string, injector do.Injector, otelShutdown func(context.Context)) {
	addr := host + ":" + port
	srv := &http.Server{
		Addr:    addr,
		Handler: r,
	}

	go func() {
		slog.Info("server starting", "addr", addr)
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

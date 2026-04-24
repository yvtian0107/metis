//go:build dev

package main

import (
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strconv"

	"github.com/samber/do/v2"
	"gorm.io/gorm"

	"metis/internal/app"
	casbinpkg "metis/internal/casbin"
	"metis/internal/config"
	"metis/internal/database"
	"metis/internal/model"
	"metis/internal/pkg/crypto"
	"metis/internal/repository"
	"metis/internal/seed"
	"metis/internal/service"
)

const (
	seedDevAdminUsername = "admin"
	seedDevAdminPassword = "password"
	seedDevAdminEmail    = "admin@local.dev"

	seedDevITSMFallbackAssigneeKey = "itsm.smart_ticket.guard.fallback_assignee"
)

func runSeedDevCommand(args []string) {
	fs := flag.NewFlagSet("seed-dev", flag.ExitOnError)
	configPath := fs.String("config", "config.yml", "path to config file")
	envPath := fs.String("env", devAIConfigPath, "path to dev environment file")
	fs.Parse(args)

	if err := runSeedDev(*configPath, *envPath); err != nil {
		slog.Error("seed-dev failed", "error", err)
		os.Exit(1)
	}
	slog.Info("seed-dev: all done", "admin_username", seedDevAdminUsername, "admin_password", seedDevAdminPassword)
}

func maybeRunSeedDev(configPath, envPath string, cfg *config.MetisConfig) (bool, error) {
	if _, err := os.Stat(envPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("check %s: %w", envPath, err)
	}

	if cfg == nil {
		return true, runSeedDev(configPath, envPath)
	}

	db, err := database.Open(cfg.DBDriver, cfg.DBDSN)
	if err != nil {
		return false, fmt.Errorf("open database for dev install check: %w", err)
	}
	installed := seed.IsInstalled(db.DB)
	if err := db.Shutdown(); err != nil {
		return false, fmt.Errorf("close database after dev install check: %w", err)
	}
	if installed {
		return false, nil
	}
	return true, runSeedDev(configPath, envPath)
}

func runSeedDev(configPath, envPath string) error {
	if _, ok, err := loadDevAIConfig(envPath); err != nil {
		return err
	} else if !ok {
		return fmt.Errorf("%s is required for seed-dev", envPath)
	}

	cfg, err := loadOrCreateSeedDevConfig(configPath)
	if err != nil {
		return err
	}
	if cfg.SecretKey == "" {
		return fmt.Errorf("%s secret_key is required for seed-dev", configPath)
	}

	db, err := database.Open(cfg.DBDriver, cfg.DBDSN)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Shutdown()

	if err := database.AutoMigrateKernel(db.DB); err != nil {
		return fmt.Errorf("kernel migration: %w", err)
	}
	for _, a := range app.All() {
		if models := a.Models(); len(models) > 0 {
			if err := db.DB.AutoMigrate(models...); err != nil {
				return fmt.Errorf("app %s migration: %w", a.Name(), err)
			}
		}
	}

	injector := do.New()
	do.ProvideValue(injector, cfg)
	do.ProvideValue(injector, db)
	do.Provide(injector, repository.NewSysConfig)
	do.Provide(injector, service.NewSysConfig)
	do.ProvideValue(injector, crypto.EncryptionKey(crypto.DeriveKey(cfg.SecretKey)))
	registerKernelProviders(injector)

	enforcer, err := casbinpkg.NewEnforcerWithDB(db.DB)
	if err != nil {
		return fmt.Errorf("casbin init: %w", err)
	}
	if result, err := seed.Install(db.DB, enforcer); err != nil {
		return fmt.Errorf("kernel seed install: %w", err)
	} else {
		slog.Info("seed-dev: kernel install seed complete",
			"roles_created", result.RolesCreated,
			"menus_created", result.MenusCreated,
			"policies_added", result.PoliciesAdded,
		)
	}

	if err := seed.SetSiteName(db.DB, "Metis"); err != nil {
		return fmt.Errorf("set site name: %w", err)
	}
	if err := seed.SetLocaleTimezone(db.DB, "zh-CN", "Asia/Shanghai"); err != nil {
		return fmt.Errorf("set locale/timezone: %w", err)
	}

	for _, a := range app.All() {
		a.Providers(injector)
		if err := a.Seed(db.DB, enforcer, true); err != nil {
			return fmt.Errorf("app %s seed: %w", a.Name(), err)
		}
		slog.Info("seed-dev: app seed complete", "app", a.Name())
	}

	adminRole, err := findSeedDevAdminRole(db.DB)
	if err != nil {
		return fmt.Errorf("admin role not found: %w", err)
	}
	if err := seed.UpsertInstallAdmin(db.DB, seedDevAdminUsername, seedDevAdminPassword, seedDevAdminEmail, adminRole.ID); err != nil {
		return fmt.Errorf("upsert dev admin: %w", err)
	}
	if err := seed.AssignInstallAdminOrgIdentity(db.DB, seedDevAdminUsername); err != nil {
		return fmt.Errorf("assign dev admin org identity: %w", err)
	}
	if err := seedDevDefaultITSMFallbackAssignee(db.DB); err != nil {
		return fmt.Errorf("seed dev ITSM fallback assignee: %w", err)
	}

	if err := runDevBootstrap(db.DB, cfg, envPath); err != nil {
		return fmt.Errorf("dev AI bootstrap: %w", err)
	}
	if err := seed.SetInstalled(db.DB); err != nil {
		return fmt.Errorf("mark installed: %w", err)
	}
	return nil
}

func seedDevDefaultITSMFallbackAssignee(db *gorm.DB) error {
	var admin model.User
	if err := db.Where("username = ? AND is_active = ?", seedDevAdminUsername, true).First(&admin).Error; err != nil {
		return fmt.Errorf("load dev admin: %w", err)
	}

	var cfg model.SystemConfig
	err := db.Where("\"key\" = ?", seedDevITSMFallbackAssigneeKey).First(&cfg).Error
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("load %s: %w", seedDevITSMFallbackAssigneeKey, err)
		}
		cfg = model.SystemConfig{
			Key:   seedDevITSMFallbackAssigneeKey,
			Value: strconv.FormatUint(uint64(admin.ID), 10),
		}
		if err := db.Create(&cfg).Error; err != nil {
			return fmt.Errorf("create %s: %w", seedDevITSMFallbackAssigneeKey, err)
		}
		return nil
	}
	if cfg.Value != "" && cfg.Value != "0" {
		return nil
	}
	cfg.Value = strconv.FormatUint(uint64(admin.ID), 10)
	if err := db.Save(&cfg).Error; err != nil {
		return fmt.Errorf("update %s: %w", seedDevITSMFallbackAssigneeKey, err)
	}
	return nil
}

func loadOrCreateSeedDevConfig(configPath string) (*config.MetisConfig, error) {
	cfg, err := config.Load(configPath)
	if err == nil {
		return cfg, nil
	}
	if !errors.Is(err, config.ErrConfigNotFound) {
		return nil, fmt.Errorf("load config: %w", err)
	}

	cfg = config.DefaultSQLiteConfig()
	if err := cfg.GenerateSecrets(); err != nil {
		return nil, fmt.Errorf("generate dev config secrets: %w", err)
	}
	if err := cfg.Save(configPath); err != nil {
		return nil, fmt.Errorf("write dev config: %w", err)
	}
	slog.Info("seed-dev: generated config", "path", configPath)
	return cfg, nil
}

func findSeedDevAdminRole(db *gorm.DB) (*model.Role, error) {
	var role model.Role
	if err := db.Where("code = ?", model.RoleAdmin).First(&role).Error; err != nil {
		return nil, err
	}
	return &role, nil
}

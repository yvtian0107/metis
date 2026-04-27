//go:build dev

package main

import (
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strconv"

	"github.com/casbin/casbin/v2"
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
	seedDevAdminEmail    = "admin@local.dev"
	seedDevPassword      = "password"

	seedDevRoleITSMServiceManager = "itsm_service_manager"
	seedDevRoleVPNApplicant       = "vpn_applicant"
	seedDevRoleVPNApprover        = "vpn_approver"

	seedDevUserITSMServiceManager  = "itsm_service_manager"
	seedDevUserVPNApplicant        = "vpn_applicant"
	seedDevUserVPNNetworkApprover  = "vpn_network_approver"
	seedDevUserVPNSecurityApprover = "vpn_security_approver"

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
	slog.Info("seed-dev: all done", "admin_username", seedDevAdminUsername, "admin_password", seedDevPassword)
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
	if err := seed.UpsertInstallAdmin(db.DB, seedDevAdminUsername, seedDevPassword, seedDevAdminEmail, adminRole.ID); err != nil {
		return fmt.Errorf("upsert dev admin: %w", err)
	}
	if err := seed.AssignInstallAdminOrgIdentity(db.DB, seedDevAdminUsername); err != nil {
		return fmt.Errorf("assign dev admin org identity: %w", err)
	}
	if err := seedDevDefaultITSMFallbackAssignee(db.DB); err != nil {
		return fmt.Errorf("seed dev ITSM fallback assignee: %w", err)
	}
	if err := seedDevVPNUsersAndRoles(db.DB, enforcer); err != nil {
		return fmt.Errorf("seed dev VPN users and roles: %w", err)
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

func seedDevVPNUsersAndRoles(db *gorm.DB, enforcer *casbin.Enforcer) error {
	roles := []model.Role{
		{Name: "ITSM 服务目录管理员", Code: seedDevRoleITSMServiceManager, Description: "开发环境服务目录与参考路径生成测试角色", Sort: 20, DataScope: model.DataScopeAll},
		{Name: "VPN 申请人", Code: seedDevRoleVPNApplicant, Description: "开发环境 VPN 申请测试角色", Sort: 21, DataScope: model.DataScopeSelf},
		{Name: "VPN 处理人", Code: seedDevRoleVPNApprover, Description: "开发环境 VPN 待办处理测试角色", Sort: 22, DataScope: model.DataScopeSelf},
	}
	roleIDs := make(map[string]uint, len(roles))
	for _, role := range roles {
		id, err := upsertSeedDevRole(db, role)
		if err != nil {
			return err
		}
		roleIDs[role.Code] = id
	}

	users := []struct {
		Username   string
		Email      string
		RoleCode   string
		Identities []seed.UserOrgIdentity
	}{
		{Username: seedDevUserITSMServiceManager, Email: "itsm_service_manager@local.dev", RoleCode: seedDevRoleITSMServiceManager},
		{Username: seedDevUserVPNApplicant, Email: "vpn_applicant@local.dev", RoleCode: seedDevRoleVPNApplicant},
		{
			Username: seedDevUserVPNNetworkApprover,
			Email:    "vpn_network_approver@local.dev",
			RoleCode: seedDevRoleVPNApprover,
			Identities: []seed.UserOrgIdentity{
				{DeptCode: "it", PosCode: "network_admin", Primary: true},
			},
		},
		{
			Username: seedDevUserVPNSecurityApprover,
			Email:    "vpn_security_approver@local.dev",
			RoleCode: seedDevRoleVPNApprover,
			Identities: []seed.UserOrgIdentity{
				{DeptCode: "it", PosCode: "security_admin", Primary: true},
			},
		},
	}
	for _, user := range users {
		roleID := roleIDs[user.RoleCode]
		if roleID == 0 {
			return fmt.Errorf("missing seed-dev role id: %s", user.RoleCode)
		}
		if err := seed.UpsertLocalUser(db, user.Username, seedDevPassword, user.Email, roleID); err != nil {
			return fmt.Errorf("upsert dev user %s: %w", user.Username, err)
		}
		if len(user.Identities) > 0 {
			if err := seed.AssignUserOrgIdentities(db, user.Username, user.Identities); err != nil {
				return fmt.Errorf("assign dev user %s org identities: %w", user.Username, err)
			}
		}
	}

	return seedDevVPNPolicies(enforcer)
}

func upsertSeedDevRole(db *gorm.DB, role model.Role) (uint, error) {
	var existing model.Role
	err := db.Where("code = ?", role.Code).First(&existing).Error
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, fmt.Errorf("load role %s: %w", role.Code, err)
		}
		if err := db.Create(&role).Error; err != nil {
			return 0, fmt.Errorf("create role %s: %w", role.Code, err)
		}
		return role.ID, nil
	}
	updates := map[string]any{
		"name":        role.Name,
		"description": role.Description,
		"sort":        role.Sort,
		"is_system":   false,
		"data_scope":  role.DataScope,
	}
	if err := db.Model(&existing).Updates(updates).Error; err != nil {
		return 0, fmt.Errorf("update role %s: %w", role.Code, err)
	}
	return existing.ID, nil
}

func seedDevVPNPolicies(enforcer *casbin.Enforcer) error {
	policies := map[string][][]string{
		seedDevRoleITSMServiceManager: {
			{seedDevRoleITSMServiceManager, "/api/v1/itsm/catalogs/tree", "GET"},
			{seedDevRoleITSMServiceManager, "/api/v1/itsm/services", "GET"},
			{seedDevRoleITSMServiceManager, "/api/v1/itsm/services/:id", "GET"},
			{seedDevRoleITSMServiceManager, "/api/v1/itsm/services/:id/actions", "GET"},
			{seedDevRoleITSMServiceManager, "/api/v1/itsm/services/:id/knowledge-documents", "GET"},
			{seedDevRoleITSMServiceManager, "/api/v1/itsm/sla", "GET"},
			{seedDevRoleITSMServiceManager, "/api/v1/itsm/workflows/generate", "POST"},
			{seedDevRoleITSMServiceManager, "itsm", "read"},
			{seedDevRoleITSMServiceManager, "itsm:service:list", "read"},
		},
		seedDevRoleVPNApplicant: {
			{seedDevRoleVPNApplicant, "/api/v1/itsm/smart-staffing/config", "GET"},
			{seedDevRoleVPNApplicant, "/api/v1/itsm/service-desk/sessions/:sid/state", "GET"},
			{seedDevRoleVPNApplicant, "/api/v1/itsm/service-desk/sessions/:sid/draft/submit", "POST"},
			{seedDevRoleVPNApplicant, "/api/v1/itsm/tickets/mine", "GET"},
			{seedDevRoleVPNApplicant, "/api/v1/itsm/tickets/:id", "GET"},
			{seedDevRoleVPNApplicant, "/api/v1/itsm/tickets/:id/timeline", "GET"},
			{seedDevRoleVPNApplicant, "/api/v1/itsm/tickets/:id/activities", "GET"},
			{seedDevRoleVPNApplicant, "/api/v1/ai/sessions", "GET"},
			{seedDevRoleVPNApplicant, "/api/v1/ai/sessions", "POST"},
			{seedDevRoleVPNApplicant, "/api/v1/ai/sessions/:sid", "GET"},
			{seedDevRoleVPNApplicant, "/api/v1/ai/sessions/:sid", "DELETE"},
			{seedDevRoleVPNApplicant, "/api/v1/ai/sessions/:sid/chat", "POST"},
			{seedDevRoleVPNApplicant, "/api/v1/ai/sessions/:sid/cancel", "POST"},
			{seedDevRoleVPNApplicant, "/api/v1/ai/sessions/:sid/images", "POST"},
			{seedDevRoleVPNApplicant, "itsm", "read"},
			{seedDevRoleVPNApplicant, "itsm:service-desk:use", "read"},
			{seedDevRoleVPNApplicant, "itsm:ticket:mine", "read"},
		},
		seedDevRoleVPNApprover: {
			{seedDevRoleVPNApprover, "/api/v1/itsm/tickets/approvals/pending", "GET"},
			{seedDevRoleVPNApprover, "/api/v1/itsm/tickets/approvals/history", "GET"},
			{seedDevRoleVPNApprover, "/api/v1/itsm/tickets/:id", "GET"},
			{seedDevRoleVPNApprover, "/api/v1/itsm/tickets/:id/timeline", "GET"},
			{seedDevRoleVPNApprover, "/api/v1/itsm/tickets/:id/activities", "GET"},
			{seedDevRoleVPNApprover, "/api/v1/itsm/tickets/:id/progress", "POST"},
			{seedDevRoleVPNApprover, "itsm", "read"},
			{seedDevRoleVPNApprover, "itsm:ticket", "read"},
			{seedDevRoleVPNApprover, "itsm:ticket:approval:pending", "read"},
			{seedDevRoleVPNApprover, "itsm:ticket:approval:history", "read"},
		},
	}

	for roleCode, rolePolicies := range policies {
		if _, err := enforcer.RemoveFilteredPolicy(0, roleCode); err != nil {
			return fmt.Errorf("clear policies for %s: %w", roleCode, err)
		}
		if len(rolePolicies) == 0 {
			continue
		}
		if _, err := enforcer.AddPolicies(rolePolicies); err != nil {
			return fmt.Errorf("add policies for %s: %w", roleCode, err)
		}
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

	if os.Getenv("METIS_DEV_DB") == "sqlite" {
		cfg = config.DefaultSQLiteConfig()
		slog.Info("seed-dev: using SQLite mode")
	} else {
		cfg = config.DefaultDevConfig()
		slog.Info("seed-dev: using PostgreSQL mode (docker-compose)")
	}
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

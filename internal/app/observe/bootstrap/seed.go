package bootstrap

import (
	"log/slog"

	"github.com/casbin/casbin/v2"
	"gorm.io/gorm"

	"metis/internal/model"
)

func SeedObserve(db *gorm.DB, enforcer *casbin.Enforcer) error {
	// 1. Menus
	var observeDir model.Menu
	if err := db.Where("permission = ?", "observe").First(&observeDir).Error; err != nil {
		observeDir = model.Menu{
			Name:       "Integrations",
			Type:       model.MenuTypeDirectory,
			Icon:       "Plug",
			Permission: "observe",
			Sort:       400,
		}
		if err := db.Create(&observeDir).Error; err != nil {
			return err
		}
		slog.Info("seed: created menu", "name", observeDir.Name, "permission", observeDir.Permission)
	}

	// Integration Catalog menu
	var catalogMenu model.Menu
	if err := db.Where("permission = ?", "observe:integrations").First(&catalogMenu).Error; err != nil {
		catalogMenu = model.Menu{
			ParentID:   &observeDir.ID,
			Name:       "Integration Catalog",
			Type:       model.MenuTypeMenu,
			Path:       "/observe/integrations",
			Icon:       "LayoutGrid",
			Permission: "observe:integrations",
			Sort:       0,
		}
		if err := db.Create(&catalogMenu).Error; err != nil {
			return err
		}
		slog.Info("seed: created menu", "name", catalogMenu.Name, "permission", catalogMenu.Permission)
	}

	// API Tokens menu
	var tokensMenu model.Menu
	if err := db.Where("permission = ?", "observe:tokens").First(&tokensMenu).Error; err != nil {
		tokensMenu = model.Menu{
			ParentID:   &observeDir.ID,
			Name:       "API Tokens",
			Type:       model.MenuTypeMenu,
			Path:       "/observe/tokens",
			Icon:       "KeyRound",
			Permission: "observe:tokens",
			Sort:       1,
		}
		if err := db.Create(&tokensMenu).Error; err != nil {
			return err
		}
		slog.Info("seed: created menu", "name", tokensMenu.Name, "permission", tokensMenu.Permission)
	}

	// Button permissions under tokens menu
	tokenButtons := []model.Menu{
		{Name: "Create Token", Type: model.MenuTypeButton, Permission: "observe:token:create", Sort: 0},
		{Name: "Revoke Token", Type: model.MenuTypeButton, Permission: "observe:token:revoke", Sort: 1},
	}
	for _, btn := range tokenButtons {
		var existing model.Menu
		if err := db.Where("permission = ?", btn.Permission).First(&existing).Error; err != nil {
			btn.ParentID = &tokensMenu.ID
			if err := db.Create(&btn).Error; err != nil {
				slog.Error("seed: failed to create button menu", "permission", btn.Permission, "error", err)
				continue
			}
			slog.Info("seed: created menu", "name", btn.Name, "permission", btn.Permission)
		}
	}

	// 2. Casbin policies for admin role
	policies := [][]string{
		{"admin", "/api/v1/observe/tokens", "POST"},
		{"admin", "/api/v1/observe/tokens", "GET"},
		{"admin", "/api/v1/observe/tokens/:id", "DELETE"},
		{"admin", "/api/v1/observe/settings", "GET"},
	}
	menuPerms := [][]string{
		{"admin", "observe", "read"},
		{"admin", "observe:integrations", "read"},
		{"admin", "observe:tokens", "read"},
		{"admin", "observe:token:create", "read"},
		{"admin", "observe:token:revoke", "read"},
	}
	for _, p := range append(policies, menuPerms...) {
		if has, _ := enforcer.HasPolicy(p); !has {
			if _, err := enforcer.AddPolicy(p); err != nil {
				slog.Error("seed: failed to add policy", "policy", p, "error", err)
			}
		}
	}

	// 3. SystemConfig default for OTel endpoint
	var existing model.SystemConfig
	if err := db.Where("\"key\" = ?", "observe.otel_endpoint").First(&existing).Error; err != nil {
		cfg := model.SystemConfig{
			Key:    "observe.otel_endpoint",
			Value:  "",
			Remark: "OTel 数据接入端点（Traefik 转发地址）",
		}
		if err := db.Create(&cfg).Error; err != nil {
			slog.Error("seed: failed to create config", "key", cfg.Key, "error", err)
		} else {
			slog.Info("seed: created config", "key", cfg.Key)
		}
	}

	return nil
}

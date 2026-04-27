package bootstrap

import (
	"log/slog"

	"github.com/casbin/casbin/v2"
	"gorm.io/gorm"

	"metis/internal/model"
)

func SeedLicense(db *gorm.DB, enforcer *casbin.Enforcer) error {
	// 1. Seed menus: 「许可管理」directory + 「商品管理」menu
	var licenseDir model.Menu
	if err := db.Where("permission = ?", "license").First(&licenseDir).Error; err != nil {
		licenseDir = model.Menu{
			Name:       "许可管理",
			Type:       model.MenuTypeDirectory,
			Icon:       "KeyRound",
			Permission: "license",
			Sort:       200,
		}
		if err := db.Create(&licenseDir).Error; err != nil {
			return err
		}
		slog.Info("seed: created menu", "name", licenseDir.Name, "permission", licenseDir.Permission)
	}

	var productMenu model.Menu
	if err := db.Where("permission = ?", "license:product:list").First(&productMenu).Error; err != nil {
		productMenu = model.Menu{
			ParentID:   &licenseDir.ID,
			Name:       "商品管理",
			Type:       model.MenuTypeMenu,
			Path:       "/license/products",
			Icon:       "Package",
			Permission: "license:product:list",
			Sort:       0,
		}
		if err := db.Create(&productMenu).Error; err != nil {
			return err
		}
		slog.Info("seed: created menu", "name", productMenu.Name, "permission", productMenu.Permission)
	}

	// Seed button permissions under product menu
	buttons := []model.Menu{
		{Name: "新增商品", Type: model.MenuTypeButton, Permission: "license:product:create", Sort: 0},
		{Name: "编辑商品", Type: model.MenuTypeButton, Permission: "license:product:update", Sort: 1},
		{Name: "管理套餐", Type: model.MenuTypeButton, Permission: "license:plan:manage", Sort: 2},
		{Name: "管理密钥", Type: model.MenuTypeButton, Permission: "license:key:manage", Sort: 3},
	}
	for _, btn := range buttons {
		var existing model.Menu
		if err := db.Where("permission = ?", btn.Permission).First(&existing).Error; err != nil {
			btn.ParentID = &productMenu.ID
			if err := db.Create(&btn).Error; err != nil {
				slog.Error("seed: failed to create button menu", "permission", btn.Permission, "error", err)
				continue
			}
			slog.Info("seed: created menu", "name", btn.Name, "permission", btn.Permission)
		}
	}

	// 3. Seed 「授权主体」menu under 「许可管理」
	var licenseeMenu model.Menu
	if err := db.Where("permission = ?", "license:licensee:list").First(&licenseeMenu).Error; err != nil {
		licenseeMenu = model.Menu{
			ParentID:   &licenseDir.ID,
			Name:       "授权主体",
			Type:       model.MenuTypeMenu,
			Path:       "/license/licensees",
			Icon:       "Building2",
			Permission: "license:licensee:list",
			Sort:       1,
		}
		if err := db.Create(&licenseeMenu).Error; err != nil {
			return err
		}
		slog.Info("seed: created menu", "name", licenseeMenu.Name, "permission", licenseeMenu.Permission)
	}

	// Seed button permissions under licensee menu
	licenseeButtons := []model.Menu{
		{Name: "新增授权主体", Type: model.MenuTypeButton, Permission: "license:licensee:create", Sort: 0},
		{Name: "编辑授权主体", Type: model.MenuTypeButton, Permission: "license:licensee:update", Sort: 1},
		{Name: "归档授权主体", Type: model.MenuTypeButton, Permission: "license:licensee:archive", Sort: 2},
	}
	for _, btn := range licenseeButtons {
		var existing model.Menu
		if err := db.Where("permission = ?", btn.Permission).First(&existing).Error; err != nil {
			btn.ParentID = &licenseeMenu.ID
			if err := db.Create(&btn).Error; err != nil {
				slog.Error("seed: failed to create button menu", "permission", btn.Permission, "error", err)
				continue
			}
			slog.Info("seed: created menu", "name", btn.Name, "permission", btn.Permission)
		}
	}

	// 3. Seed 「许可签发」menu under 「许可管理」
	var licenseMenu model.Menu
	if err := db.Where("permission = ?", "license:license:list").First(&licenseMenu).Error; err != nil {
		licenseMenu = model.Menu{
			ParentID:   &licenseDir.ID,
			Name:       "许可签发",
			Type:       model.MenuTypeMenu,
			Path:       "/license/licenses",
			Icon:       "FileBadge",
			Permission: "license:license:list",
			Sort:       2,
		}
		if err := db.Create(&licenseMenu).Error; err != nil {
			return err
		}
		slog.Info("seed: created menu", "name", licenseMenu.Name, "permission", licenseMenu.Permission)
	}

	// Seed button permissions under license menu
	licenseButtons := []model.Menu{
		{Name: "签发许可", Type: model.MenuTypeButton, Permission: "license:license:issue", Sort: 0},
		{Name: "吊销许可", Type: model.MenuTypeButton, Permission: "license:license:revoke", Sort: 1},
		{Name: "续期许可", Type: model.MenuTypeButton, Permission: "license:license:renew", Sort: 2},
		{Name: "升级许可", Type: model.MenuTypeButton, Permission: "license:license:upgrade", Sort: 3},
		{Name: "暂停许可", Type: model.MenuTypeButton, Permission: "license:license:suspend", Sort: 4},
		{Name: "恢复许可", Type: model.MenuTypeButton, Permission: "license:license:reactivate", Sort: 5},
	}
	for _, btn := range licenseButtons {
		var existing model.Menu
		if err := db.Where("permission = ?", btn.Permission).First(&existing).Error; err != nil {
			btn.ParentID = &licenseMenu.ID
			if err := db.Create(&btn).Error; err != nil {
				slog.Error("seed: failed to create button menu", "permission", btn.Permission, "error", err)
				continue
			}
			slog.Info("seed: created menu", "name", btn.Name, "permission", btn.Permission)
		}
	}

	// 4. Seed 「注册码管理」menu under 「许可管理」
	var regMenu model.Menu
	if err := db.Where("permission = ?", "license:registration:list").First(&regMenu).Error; err != nil {
		regMenu = model.Menu{
			ParentID:   &licenseDir.ID,
			Name:       "注册码管理",
			Type:       model.MenuTypeMenu,
			Path:       "/license/registrations",
			Icon:       "Ticket",
			Permission: "license:registration:list",
			Sort:       3,
		}
		if err := db.Create(&regMenu).Error; err != nil {
			return err
		}
		slog.Info("seed: created menu", "name", regMenu.Name, "permission", regMenu.Permission)
	}

	regButtons := []model.Menu{
		{Name: "新增注册码", Type: model.MenuTypeButton, Permission: "license:registration:create", Sort: 0},
		{Name: "自动生成注册码", Type: model.MenuTypeButton, Permission: "license:registration:generate", Sort: 1},
	}
	for _, btn := range regButtons {
		var existing model.Menu
		if err := db.Where("permission = ?", btn.Permission).First(&existing).Error; err != nil {
			btn.ParentID = &regMenu.ID
			if err := db.Create(&btn).Error; err != nil {
				slog.Error("seed: failed to create button menu", "permission", btn.Permission, "error", err)
				continue
			}
			slog.Info("seed: created menu", "name", btn.Name, "permission", btn.Permission)
		}
	}

	// 2. Seed Casbin policies for admin role
	policies := [][]string{
		// Products
		{"admin", "/api/v1/license/products", "GET"},
		{"admin", "/api/v1/license/products", "POST"},
		{"admin", "/api/v1/license/products/:id", "GET"},
		{"admin", "/api/v1/license/products/:id", "PUT"},
		{"admin", "/api/v1/license/products/:id/schema", "PUT"},
		{"admin", "/api/v1/license/products/:id/status", "PATCH"},
		{"admin", "/api/v1/license/products/:id/rotate-key", "POST"},
		{"admin", "/api/v1/license/products/:id/rotate-key-impact", "GET"},
		{"admin", "/api/v1/license/products/:id/bulk-reissue", "POST"},
		{"admin", "/api/v1/license/products/:id/public-key", "GET"},
		// Plans
		{"admin", "/api/v1/license/products/:id/plans", "POST"},
		{"admin", "/api/v1/license/plans/:id", "PUT"},
		{"admin", "/api/v1/license/plans/:id", "DELETE"},
		{"admin", "/api/v1/license/plans/:id/default", "PATCH"},
		// Licensees
		{"admin", "/api/v1/license/licensees", "GET"},
		{"admin", "/api/v1/license/licensees", "POST"},
		{"admin", "/api/v1/license/licensees/:id", "GET"},
		{"admin", "/api/v1/license/licensees/:id", "PUT"},
		{"admin", "/api/v1/license/licensees/:id/status", "PATCH"},
		// Licenses
		{"admin", "/api/v1/license/licenses", "GET"},
		{"admin", "/api/v1/license/licenses", "POST"},
		{"admin", "/api/v1/license/licenses/:id", "GET"},
		{"admin", "/api/v1/license/licenses/:id/renew", "POST"},
		{"admin", "/api/v1/license/licenses/:id/upgrade", "POST"},
		{"admin", "/api/v1/license/licenses/:id/suspend", "POST"},
		{"admin", "/api/v1/license/licenses/:id/reactivate", "POST"},
		{"admin", "/api/v1/license/licenses/:id/revoke", "PATCH"},
		{"admin", "/api/v1/license/licenses/:id/export", "GET"},
		// Registrations
		{"admin", "/api/v1/license/registrations", "GET"},
		{"admin", "/api/v1/license/registrations", "POST"},
		{"admin", "/api/v1/license/registrations/generate", "POST"},
	}

	// Also add menu permissions
	menuPerms := [][]string{
		{"admin", "license", "read"},
		{"admin", "license:product:list", "read"},
		{"admin", "license:product:create", "read"},
		{"admin", "license:product:update", "read"},
		{"admin", "license:plan:manage", "read"},
		{"admin", "license:key:manage", "read"},
		{"admin", "license:licensee:list", "read"},
		{"admin", "license:licensee:create", "read"},
		{"admin", "license:licensee:update", "read"},
		{"admin", "license:licensee:archive", "read"},
		{"admin", "license:license:list", "read"},
		{"admin", "license:license:issue", "read"},
		{"admin", "license:license:revoke", "read"},
		{"admin", "license:license:renew", "read"},
		{"admin", "license:license:upgrade", "read"},
		{"admin", "license:license:suspend", "read"},
		{"admin", "license:license:reactivate", "read"},
		{"admin", "license:registration:list", "read"},
		{"admin", "license:registration:create", "read"},
		{"admin", "license:registration:generate", "read"},
	}

	allPolicies := append(policies, menuPerms...)
	for _, p := range allPolicies {
		if has, _ := enforcer.HasPolicy(p); !has {
			if _, err := enforcer.AddPolicy(p); err != nil {
				slog.Error("seed: failed to add policy", "policy", p, "error", err)
			}
		}
	}

	return nil
}

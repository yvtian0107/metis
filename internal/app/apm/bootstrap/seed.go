package bootstrap

import (
	"log/slog"

	"github.com/casbin/casbin/v2"
	"gorm.io/gorm"

	"metis/internal/model"
)

func SeedAPM(db *gorm.DB, enforcer *casbin.Enforcer) error {
	// 1. APM directory menu
	var apmDir model.Menu
	if err := db.Where("permission = ?", "apm").First(&apmDir).Error; err != nil {
		apmDir = model.Menu{
			Name:       "APM",
			Type:       model.MenuTypeDirectory,
			Icon:       "Activity",
			Permission: "apm",
			Sort:       350,
		}
		if err := db.Create(&apmDir).Error; err != nil {
			return err
		}
		slog.Info("seed: created menu", "name", apmDir.Name, "permission", apmDir.Permission)
	}

	// Traces menu
	var tracesMenu model.Menu
	if err := db.Where("permission = ?", "apm:traces").First(&tracesMenu).Error; err != nil {
		tracesMenu = model.Menu{
			ParentID:   &apmDir.ID,
			Name:       "Traces",
			Type:       model.MenuTypeMenu,
			Path:       "/apm/traces",
			Icon:       "GitBranch",
			Permission: "apm:traces",
			Sort:       1,
		}
		if err := db.Create(&tracesMenu).Error; err != nil {
			return err
		}
		slog.Info("seed: created menu", "name", tracesMenu.Name, "permission", tracesMenu.Permission)
	}

	// Services menu
	var servicesMenu model.Menu
	if err := db.Where("permission = ?", "apm:services").First(&servicesMenu).Error; err != nil {
		servicesMenu = model.Menu{
			ParentID:   &apmDir.ID,
			Name:       "Services",
			Type:       model.MenuTypeMenu,
			Path:       "/apm/services",
			Icon:       "Server",
			Permission: "apm:services",
			Sort:       0,
		}
		if err := db.Create(&servicesMenu).Error; err != nil {
			return err
		}
		slog.Info("seed: created menu", "name", servicesMenu.Name, "permission", servicesMenu.Permission)
	}

	// Topology menu
	var topologyMenu model.Menu
	if err := db.Where("permission = ?", "apm:topology").First(&topologyMenu).Error; err != nil {
		topologyMenu = model.Menu{
			ParentID:   &apmDir.ID,
			Name:       "Topology",
			Type:       model.MenuTypeMenu,
			Path:       "/apm/topology",
			Icon:       "Network",
			Permission: "apm:topology",
			Sort:       2,
		}
		if err := db.Create(&topologyMenu).Error; err != nil {
			return err
		}
		slog.Info("seed: created menu", "name", topologyMenu.Name, "permission", topologyMenu.Permission)
	}

	// 2. Casbin policies for admin role
	policies := [][]string{
		{"admin", "/api/v1/apm/traces", "GET"},
		{"admin", "/api/v1/apm/traces/:traceId", "GET"},
		{"admin", "/api/v1/apm/traces/:traceId/logs", "GET"},
		{"admin", "/api/v1/apm/services", "GET"},
		{"admin", "/api/v1/apm/services/:name", "GET"},
		{"admin", "/api/v1/apm/timeseries", "GET"},
		{"admin", "/api/v1/apm/topology", "GET"},
		{"admin", "/api/v1/apm/spans/search", "GET"},
		{"admin", "/api/v1/apm/analytics", "GET"},
		{"admin", "/api/v1/apm/latency-distribution", "GET"},
		{"admin", "/api/v1/apm/errors", "GET"},
	}
	menuPerms := [][]string{
		{"admin", "apm", "read"},
		{"admin", "apm:traces", "read"},
		{"admin", "apm:services", "read"},
		{"admin", "apm:topology", "read"},
	}
	for _, p := range append(policies, menuPerms...) {
		if has, _ := enforcer.HasPolicy(p); !has {
			if _, err := enforcer.AddPolicy(p); err != nil {
				slog.Error("seed: failed to add policy", "policy", p, "error", err)
			}
		}
	}

	return nil
}

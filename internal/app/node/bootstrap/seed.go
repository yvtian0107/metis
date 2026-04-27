package bootstrap

import (
	"log/slog"
	"metis/internal/app/node/domain"

	"github.com/casbin/casbin/v2"
	"gorm.io/gorm"

	"metis/internal/model"
)

func SeedNode(db *gorm.DB, enforcer *casbin.Enforcer) error {
	// Migration: binary+args+reload_signal → start_command+stop_command
	migrateProcessDefColumns(db)

	// 1. Seed menus: 「节点管理」directory
	var nodeDir model.Menu
	if err := db.Where("permission = ?", "node").First(&nodeDir).Error; err != nil {
		nodeDir = model.Menu{
			Name:       "节点管理",
			Type:       model.MenuTypeDirectory,
			Icon:       "Server",
			Permission: "node",
			Sort:       300,
		}
		if err := db.Create(&nodeDir).Error; err != nil {
			return err
		}
		slog.Info("seed: created menu", "name", nodeDir.Name, "permission", nodeDir.Permission)
	}

	// 「节点列表」menu
	var nodeListMenu model.Menu
	if err := db.Where("permission = ?", "node:list").First(&nodeListMenu).Error; err != nil {
		nodeListMenu = model.Menu{
			ParentID:   &nodeDir.ID,
			Name:       "节点列表",
			Type:       model.MenuTypeMenu,
			Path:       "/node/nodes",
			Icon:       "Monitor",
			Permission: "node:list",
			Sort:       0,
		}
		if err := db.Create(&nodeListMenu).Error; err != nil {
			return err
		}
		slog.Info("seed: created menu", "name", nodeListMenu.Name, "permission", nodeListMenu.Permission)
	}

	// Button permissions under node list
	nodeButtons := []model.Menu{
		{Name: "新增节点", Type: model.MenuTypeButton, Permission: "node:create", Sort: 0},
		{Name: "编辑节点", Type: model.MenuTypeButton, Permission: "node:update", Sort: 1},
		{Name: "删除节点", Type: model.MenuTypeButton, Permission: "node:delete", Sort: 2},
		{Name: "轮换令牌", Type: model.MenuTypeButton, Permission: "node:rotate-token", Sort: 3},
	}
	for _, btn := range nodeButtons {
		var existing model.Menu
		if err := db.Where("permission = ?", btn.Permission).First(&existing).Error; err != nil {
			btn.ParentID = &nodeListMenu.ID
			if err := db.Create(&btn).Error; err != nil {
				slog.Error("seed: failed to create button menu", "permission", btn.Permission, "error", err)
				continue
			}
			slog.Info("seed: created menu", "name", btn.Name, "permission", btn.Permission)
		}
	}

	// 「进程定义」menu
	var processDefMenu model.Menu
	if err := db.Where("permission = ?", "node:process-def:list").First(&processDefMenu).Error; err != nil {
		processDefMenu = model.Menu{
			ParentID:   &nodeDir.ID,
			Name:       "进程定义",
			Type:       model.MenuTypeMenu,
			Path:       "/node/process-defs",
			Icon:       "Cog",
			Permission: "node:process-def:list",
			Sort:       1,
		}
		if err := db.Create(&processDefMenu).Error; err != nil {
			return err
		}
		slog.Info("seed: created menu", "name", processDefMenu.Name, "permission", processDefMenu.Permission)
	}

	processDefButtons := []model.Menu{
		{Name: "新增进程定义", Type: model.MenuTypeButton, Permission: "node:process-def:create", Sort: 0},
		{Name: "编辑进程定义", Type: model.MenuTypeButton, Permission: "node:process-def:update", Sort: 1},
		{Name: "删除进程定义", Type: model.MenuTypeButton, Permission: "node:process-def:delete", Sort: 2},
	}
	for _, btn := range processDefButtons {
		var existing model.Menu
		if err := db.Where("permission = ?", btn.Permission).First(&existing).Error; err != nil {
			btn.ParentID = &processDefMenu.ID
			if err := db.Create(&btn).Error; err != nil {
				slog.Error("seed: failed to create button menu", "permission", btn.Permission, "error", err)
				continue
			}
			slog.Info("seed: created menu", "name", btn.Name, "permission", btn.Permission)
		}
	}

	// 2. Seed Casbin policies for admin role
	policies := [][]string{
		// Nodes
		{"admin", "/api/v1/nodes", "POST"},
		{"admin", "/api/v1/nodes", "GET"},
		{"admin", "/api/v1/nodes/:id", "GET"},
		{"admin", "/api/v1/nodes/:id", "PUT"},
		{"admin", "/api/v1/nodes/:id", "DELETE"},
		{"admin", "/api/v1/nodes/:id/rotate-token", "POST"},
		// domain.Node processes
		{"admin", "/api/v1/nodes/:id/processes", "POST"},
		{"admin", "/api/v1/nodes/:id/processes", "GET"},
		{"admin", "/api/v1/nodes/:id/processes/:processId", "DELETE"},
		{"admin", "/api/v1/nodes/:id/processes/:processId/start", "POST"},
		{"admin", "/api/v1/nodes/:id/processes/:processId/stop", "POST"},
		{"admin", "/api/v1/nodes/:id/processes/:processId/restart", "POST"},
		{"admin", "/api/v1/nodes/:id/processes/:processId/reload", "POST"},
		{"admin", "/api/v1/nodes/:id/processes/:processId/logs", "GET"},
		{"admin", "/api/v1/nodes/:id/commands", "GET"},
		// Process definitions
		{"admin", "/api/v1/process-defs", "POST"},
		{"admin", "/api/v1/process-defs", "GET"},
		{"admin", "/api/v1/process-defs/:id", "GET"},
		{"admin", "/api/v1/process-defs/:id", "PUT"},
		{"admin", "/api/v1/process-defs/:id", "DELETE"},
		{"admin", "/api/v1/process-defs/:id/nodes", "GET"},
	}

	menuPerms := [][]string{
		{"admin", "node", "read"},
		{"admin", "node:list", "read"},
		{"admin", "node:create", "read"},
		{"admin", "node:update", "read"},
		{"admin", "node:delete", "read"},
		{"admin", "node:rotate-token", "read"},
		{"admin", "node:process-def:list", "read"},
		{"admin", "node:process-def:create", "read"},
		{"admin", "node:process-def:update", "read"},
		{"admin", "node:process-def:delete", "read"},
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

// migrateProcessDefColumns handles the migration from binary/args/reload_signal
// columns to the new start_command/stop_command columns.
func migrateProcessDefColumns(db *gorm.DB) {
	migrator := db.Migrator()
	if !migrator.HasColumn(&domain.ProcessDef{}, "binary") {
		return
	}

	slog.Info("seed: migrating process_defs: binary → start_command")

	// Copy binary value into start_command for existing rows
	db.Exec("UPDATE process_defs SET start_command = binary WHERE start_command = '' OR start_command IS NULL")

	// Drop obsolete columns
	for _, col := range []string{"binary", "args", "reload_signal"} {
		if migrator.HasColumn(&domain.ProcessDef{}, col) {
			if err := migrator.DropColumn(&domain.ProcessDef{}, col); err != nil {
				slog.Warn("seed: failed to drop column", "column", col, "error", err)
			}
		}
	}
}

package itsm

import (
	"log/slog"

	"github.com/casbin/casbin/v2"
	"gorm.io/gorm"

	"metis/internal/app/itsm/tools"
	"metis/internal/model"
)

func seedITSM(db *gorm.DB, enforcer *casbin.Enforcer) error {
	if err := seedMenus(db); err != nil {
		return err
	}
	if err := seedPolicies(enforcer); err != nil {
		return err
	}
	if err := seedPriorities(db); err != nil {
		return err
	}
	if err := seedSLATemplates(db); err != nil {
		return err
	}
	if err := tools.SeedTools(db); err != nil {
		return err
	}
	return tools.SeedAgents(db)
}

func seedMenus(db *gorm.DB) error {
	// ITSM 顶级目录
	var itsmDir model.Menu
	if err := db.Where("permission = ?", "itsm").First(&itsmDir).Error; err != nil {
		itsmDir = model.Menu{
			Name:       "ITSM",
			Type:       model.MenuTypeDirectory,
			Icon:       "Headset",
			Permission: "itsm",
			Sort:       400,
		}
		if err := db.Create(&itsmDir).Error; err != nil {
			return err
		}
		slog.Info("seed: created menu", "name", itsmDir.Name, "permission", itsmDir.Permission)
	}

	// 服务目录
	catalogMenu := seedMenu(db, &itsmDir.ID, "服务目录", model.MenuTypeMenu, "/itsm/catalogs", "FolderTree", "itsm:catalog:list", 0)
	seedButtons(db, catalogMenu, []model.Menu{
		{Name: "新增分类", Type: model.MenuTypeButton, Permission: "itsm:catalog:create", Sort: 0},
		{Name: "编辑分类", Type: model.MenuTypeButton, Permission: "itsm:catalog:update", Sort: 1},
		{Name: "删除分类", Type: model.MenuTypeButton, Permission: "itsm:catalog:delete", Sort: 2},
	})

	// 服务定义
	serviceMenu := seedMenu(db, &itsmDir.ID, "服务定义", model.MenuTypeMenu, "/itsm/services", "Cog", "itsm:service:list", 1)
	seedButtons(db, serviceMenu, []model.Menu{
		{Name: "新增服务", Type: model.MenuTypeButton, Permission: "itsm:service:create", Sort: 0},
		{Name: "编辑服务", Type: model.MenuTypeButton, Permission: "itsm:service:update", Sort: 1},
		{Name: "删除服务", Type: model.MenuTypeButton, Permission: "itsm:service:delete", Sort: 2},
	})

	// 工单管理 子目录
	var ticketDir model.Menu
	if err := db.Where("permission = ?", "itsm:ticket").First(&ticketDir).Error; err != nil {
		ticketDir = model.Menu{
			ParentID:   &itsmDir.ID,
			Name:       "工单管理",
			Type:       model.MenuTypeDirectory,
			Icon:       "ClipboardList",
			Permission: "itsm:ticket",
			Sort:       2,
		}
		if err := db.Create(&ticketDir).Error; err != nil {
			return err
		}
		slog.Info("seed: created menu", "name", ticketDir.Name, "permission", ticketDir.Permission)
	}

	// 全部工单
	allTicketMenu := seedMenu(db, &ticketDir.ID, "全部工单", model.MenuTypeMenu, "/itsm/tickets", "List", "itsm:ticket:list", 0)
	seedButtons(db, allTicketMenu, []model.Menu{
		{Name: "创建工单", Type: model.MenuTypeButton, Permission: "itsm:ticket:create", Sort: 0},
		{Name: "指派工单", Type: model.MenuTypeButton, Permission: "itsm:ticket:assign", Sort: 1},
		{Name: "完结工单", Type: model.MenuTypeButton, Permission: "itsm:ticket:complete", Sort: 2},
		{Name: "取消工单", Type: model.MenuTypeButton, Permission: "itsm:ticket:cancel", Sort: 3},
	})

	// 我的工单
	seedMenu(db, &ticketDir.ID, "我的工单", model.MenuTypeMenu, "/itsm/tickets/mine", "User", "itsm:ticket:mine", 1)
	// 我的待办
	seedMenu(db, &ticketDir.ID, "我的待办", model.MenuTypeMenu, "/itsm/tickets/todo", "Clock", "itsm:ticket:todo", 2)
	// 历史工单
	seedMenu(db, &ticketDir.ID, "历史工单", model.MenuTypeMenu, "/itsm/tickets/history", "Archive", "itsm:ticket:history", 3)

	// 我的审批
	seedMenu(db, &ticketDir.ID, "我的审批", model.MenuTypeMenu, "/itsm/tickets/approvals", "CheckCircle", "itsm:ticket:approvals", 4)

	// 优先级管理
	priorityMenu := seedMenu(db, &itsmDir.ID, "优先级管理", model.MenuTypeMenu, "/itsm/priorities", "Flag", "itsm:priority:list", 3)
	seedButtons(db, priorityMenu, []model.Menu{
		{Name: "新增优先级", Type: model.MenuTypeButton, Permission: "itsm:priority:create", Sort: 0},
		{Name: "编辑优先级", Type: model.MenuTypeButton, Permission: "itsm:priority:update", Sort: 1},
		{Name: "删除优先级", Type: model.MenuTypeButton, Permission: "itsm:priority:delete", Sort: 2},
	})

	// SLA 管理
	slaMenu := seedMenu(db, &itsmDir.ID, "SLA 管理", model.MenuTypeMenu, "/itsm/sla", "Timer", "itsm:sla:list", 4)
	seedButtons(db, slaMenu, []model.Menu{
		{Name: "新增SLA", Type: model.MenuTypeButton, Permission: "itsm:sla:create", Sort: 0},
		{Name: "编辑SLA", Type: model.MenuTypeButton, Permission: "itsm:sla:update", Sort: 1},
		{Name: "删除SLA", Type: model.MenuTypeButton, Permission: "itsm:sla:delete", Sort: 2},
	})

	return nil
}

func seedMenu(db *gorm.DB, parentID *uint, name string, menuType model.MenuType, path, icon, permission string, sort int) *model.Menu {
	var menu model.Menu
	if err := db.Where("permission = ?", permission).First(&menu).Error; err != nil {
		menu = model.Menu{
			ParentID:   parentID,
			Name:       name,
			Type:       menuType,
			Path:       path,
			Icon:       icon,
			Permission: permission,
			Sort:       sort,
		}
		if err := db.Create(&menu).Error; err != nil {
			slog.Error("seed: failed to create menu", "permission", permission, "error", err)
			return nil
		}
		slog.Info("seed: created menu", "name", menu.Name, "permission", menu.Permission)
	}
	return &menu
}

func seedButtons(db *gorm.DB, parent *model.Menu, buttons []model.Menu) {
	if parent == nil {
		return
	}
	for _, btn := range buttons {
		var existing model.Menu
		if err := db.Where("permission = ?", btn.Permission).First(&existing).Error; err != nil {
			btn.ParentID = &parent.ID
			if err := db.Create(&btn).Error; err != nil {
				slog.Error("seed: failed to create button", "permission", btn.Permission, "error", err)
				continue
			}
			slog.Info("seed: created menu", "name", btn.Name, "permission", btn.Permission)
		}
	}
}

func seedPolicies(enforcer *casbin.Enforcer) error {
	policies := [][]string{
		// Catalogs
		{"admin", "/api/v1/itsm/catalogs", "POST"},
		{"admin", "/api/v1/itsm/catalogs/tree", "GET"},
		{"admin", "/api/v1/itsm/catalogs/:id", "PUT"},
		{"admin", "/api/v1/itsm/catalogs/:id", "DELETE"},
		// Services
		{"admin", "/api/v1/itsm/services", "POST"},
		{"admin", "/api/v1/itsm/services", "GET"},
		{"admin", "/api/v1/itsm/services/:id", "GET"},
		{"admin", "/api/v1/itsm/services/:id", "PUT"},
		{"admin", "/api/v1/itsm/services/:id", "DELETE"},
		// Service Actions
		{"admin", "/api/v1/itsm/services/:id/actions", "POST"},
		{"admin", "/api/v1/itsm/services/:id/actions", "GET"},
		{"admin", "/api/v1/itsm/services/:id/actions/:actionId", "PUT"},
		{"admin", "/api/v1/itsm/services/:id/actions/:actionId", "DELETE"},
		// Priorities
		{"admin", "/api/v1/itsm/priorities", "POST"},
		{"admin", "/api/v1/itsm/priorities", "GET"},
		{"admin", "/api/v1/itsm/priorities/:id", "PUT"},
		{"admin", "/api/v1/itsm/priorities/:id", "DELETE"},
		// SLA
		{"admin", "/api/v1/itsm/sla", "POST"},
		{"admin", "/api/v1/itsm/sla", "GET"},
		{"admin", "/api/v1/itsm/sla/:id", "PUT"},
		{"admin", "/api/v1/itsm/sla/:id", "DELETE"},
		// Escalation Rules
		{"admin", "/api/v1/itsm/sla/:id/escalations", "POST"},
		{"admin", "/api/v1/itsm/sla/:id/escalations", "GET"},
		{"admin", "/api/v1/itsm/sla/:id/escalations/:escalationId", "PUT"},
		{"admin", "/api/v1/itsm/sla/:id/escalations/:escalationId", "DELETE"},
		// Tickets
		{"admin", "/api/v1/itsm/tickets", "POST"},
		{"admin", "/api/v1/itsm/tickets", "GET"},
		{"admin", "/api/v1/itsm/tickets/mine", "GET"},
		{"admin", "/api/v1/itsm/tickets/todo", "GET"},
		{"admin", "/api/v1/itsm/tickets/history", "GET"},
		{"admin", "/api/v1/itsm/tickets/:id", "GET"},
		{"admin", "/api/v1/itsm/tickets/:id/assign", "PUT"},
		{"admin", "/api/v1/itsm/tickets/:id/complete", "PUT"},
		{"admin", "/api/v1/itsm/tickets/:id/cancel", "PUT"},
		{"admin", "/api/v1/itsm/tickets/:id/timeline", "GET"},
		// Classic engine routes
		{"admin", "/api/v1/itsm/tickets/:id/progress", "POST"},
		{"admin", "/api/v1/itsm/tickets/:id/signal", "POST"},
		{"admin", "/api/v1/itsm/tickets/:id/activities", "GET"},
		// Smart engine override routes
		{"admin", "/api/v1/itsm/tickets/:id/activities/:aid/confirm", "POST"},
		{"admin", "/api/v1/itsm/tickets/:id/activities/:aid/reject", "POST"},
		{"admin", "/api/v1/itsm/tickets/:id/override/jump", "POST"},
		{"admin", "/api/v1/itsm/tickets/:id/override/reassign", "POST"},
		{"admin", "/api/v1/itsm/tickets/:id/override/retry-ai", "POST"},
		// Approval routes
		{"admin", "/api/v1/itsm/tickets/approvals", "GET"},
		{"admin", "/api/v1/itsm/tickets/approvals/count", "GET"},
		{"admin", "/api/v1/itsm/tickets/:id/activities/:aid/approve", "POST"},
		{"admin", "/api/v1/itsm/tickets/:id/activities/:aid/deny", "POST"},
	}

	menuPerms := [][]string{
		{"admin", "itsm", "read"},
		{"admin", "itsm:catalog:list", "read"},
		{"admin", "itsm:catalog:create", "read"},
		{"admin", "itsm:catalog:update", "read"},
		{"admin", "itsm:catalog:delete", "read"},
		{"admin", "itsm:service:list", "read"},
		{"admin", "itsm:service:create", "read"},
		{"admin", "itsm:service:update", "read"},
		{"admin", "itsm:service:delete", "read"},
		{"admin", "itsm:ticket", "read"},
		{"admin", "itsm:ticket:list", "read"},
		{"admin", "itsm:ticket:create", "read"},
		{"admin", "itsm:ticket:assign", "read"},
		{"admin", "itsm:ticket:complete", "read"},
		{"admin", "itsm:ticket:cancel", "read"},
		{"admin", "itsm:ticket:mine", "read"},
		{"admin", "itsm:ticket:todo", "read"},
		{"admin", "itsm:ticket:history", "read"},
		{"admin", "itsm:ticket:approvals", "read"},
		{"admin", "itsm:priority:list", "read"},
		{"admin", "itsm:priority:create", "read"},
		{"admin", "itsm:priority:update", "read"},
		{"admin", "itsm:priority:delete", "read"},
		{"admin", "itsm:sla:list", "read"},
		{"admin", "itsm:sla:create", "read"},
		{"admin", "itsm:sla:update", "read"},
		{"admin", "itsm:sla:delete", "read"},
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

func seedPriorities(db *gorm.DB) error {
	priorities := []Priority{
		{Name: "紧急", Code: "P0", Value: 1, Color: "#FF0000", Description: "紧急问题，需要立即处理", DefaultResponseMinutes: 15, DefaultResolutionMinutes: 120, IsActive: true},
		{Name: "高", Code: "P1", Value: 2, Color: "#FF6600", Description: "高优先级，需要尽快处理", DefaultResponseMinutes: 60, DefaultResolutionMinutes: 480, IsActive: true},
		{Name: "中", Code: "P2", Value: 3, Color: "#FFAA00", Description: "中等优先级", DefaultResponseMinutes: 240, DefaultResolutionMinutes: 1440, IsActive: true},
		{Name: "低", Code: "P3", Value: 4, Color: "#00AA00", Description: "低优先级", DefaultResponseMinutes: 480, DefaultResolutionMinutes: 4320, IsActive: true},
		{Name: "最低", Code: "P4", Value: 5, Color: "#888888", Description: "最低优先级", DefaultResponseMinutes: 1440, DefaultResolutionMinutes: 10080, IsActive: true},
	}

	for _, p := range priorities {
		var existing Priority
		if err := db.Where("code = ?", p.Code).First(&existing).Error; err != nil {
			if err := db.Create(&p).Error; err != nil {
				slog.Error("seed: failed to create priority", "code", p.Code, "error", err)
				continue
			}
			slog.Info("seed: created priority", "code", p.Code, "name", p.Name)
		}
	}

	return nil
}

func seedSLATemplates(db *gorm.DB) error {
	templates := []SLATemplate{
		{Name: "标准", Code: "standard", Description: "标准 SLA，响应 4 小时，解决 24 小时", ResponseMinutes: 240, ResolutionMinutes: 1440, IsActive: true},
		{Name: "紧急", Code: "urgent", Description: "紧急 SLA，响应 30 分钟，解决 4 小时", ResponseMinutes: 30, ResolutionMinutes: 240, IsActive: true},
	}

	for _, t := range templates {
		var existing SLATemplate
		if err := db.Where("code = ?", t.Code).First(&existing).Error; err != nil {
			if err := db.Create(&t).Error; err != nil {
				slog.Error("seed: failed to create SLA template", "code", t.Code, "error", err)
				continue
			}
			slog.Info("seed: created SLA template", "code", t.Code, "name", t.Name)
		}
	}

	return nil
}

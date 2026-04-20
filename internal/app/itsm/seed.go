package itsm

import (
	"log/slog"
	"strconv"

	"github.com/casbin/casbin/v2"
	"gorm.io/gorm"

	"metis/internal/app/itsm/tools"
	"metis/internal/model"
)

func seedITSM(db *gorm.DB, enforcer *casbin.Enforcer) error {
	if err := seedMenus(db); err != nil {
		return err
	}
	if err := seedCatalogs(db); err != nil {
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
	if err := tools.SeedAgents(db); err != nil {
		return err
	}
	if err := seedEngineConfig(db); err != nil {
		return err
	}
	return seedServiceDefinitions(db)
}

func seedCatalogs(db *gorm.DB) error {
	type catalogSeed struct {
		Name        string
		Code        string
		Description string
		Icon        string
		SortOrder   int
		ParentCode  string // empty for root
	}

	seeds := []catalogSeed{
		// ── 一级域 ──────────────────────────────────────────
		{Name: "账号与权限", Code: "account-access", Description: "围绕身份、账户与访问控制的目录分类。", Icon: "ShieldCheck", SortOrder: 10},
		{Name: "终端与办公支持", Code: "workplace-support", Description: "围绕终端设备、办公环境与桌面支持的目录分类。", Icon: "Monitor", SortOrder: 20},
		{Name: "基础设施与网络", Code: "infra-network", Description: "围绕网络、主机、存储和基础运行环境的目录分类。", Icon: "Globe", SortOrder: 30},
		{Name: "应用与平台支持", Code: "application-platform", Description: "围绕企业应用、发布平台和数据库服务的目录分类。", Icon: "Container", SortOrder: 40},
		{Name: "安全与合规", Code: "security-compliance", Description: "围绕安全事件、漏洞治理与审计合规的目录分类。", Icon: "ShieldAlert", SortOrder: 50},
		{Name: "监控与告警", Code: "monitoring-alerting", Description: "围绕监控平台、告警治理和值班机制的目录分类。", Icon: "Bell", SortOrder: 60},

		// ── 账号与权限 子分类 ─────────────────────────────────
		{Name: "账号开通", Code: "account-access:provisioning", ParentCode: "account-access", Description: "员工账号开通、账号重建与账号合并。", Icon: "User", SortOrder: 1},
		{Name: "权限申请", Code: "account-access:authorization", ParentCode: "account-access", Description: "系统角色、数据权限与临时授权相关分类。", Icon: "Lock", SortOrder: 2},
		{Name: "密码与 MFA", Code: "account-access:credential", ParentCode: "account-access", Description: "密码重置、MFA 绑定与身份验证协助。", Icon: "KeyRound", SortOrder: 3},

		// ── 终端与办公支持 子分类 ─────────────────────────────
		{Name: "电脑与外设", Code: "workplace-support:endpoint", ParentCode: "workplace-support", Description: "笔记本、显示器、外设与桌面环境支持。", Icon: "Monitor", SortOrder: 1},
		{Name: "办公软件支持", Code: "workplace-support:office-software", ParentCode: "workplace-support", Description: "办公套件、协作工具与客户端故障处理。", Icon: "LayoutGrid", SortOrder: 2},
		{Name: "打印与会议室设备", Code: "workplace-support:meeting-room", ParentCode: "workplace-support", Description: "打印、投屏、音视频设备与会议室终端支持。", Icon: "Video", SortOrder: 3},

		// ── 基础设施与网络 子分类 ─────────────────────────────
		{Name: "网络与 VPN", Code: "infra-network:network", ParentCode: "infra-network", Description: "办公网络、专线、VPN 与连通性支持。", Icon: "Globe", SortOrder: 1},
		{Name: "服务器与主机", Code: "infra-network:compute", ParentCode: "infra-network", Description: "物理机、云主机与运行环境相关分类。", Icon: "Server", SortOrder: 2},
		{Name: "存储与备份", Code: "infra-network:storage", ParentCode: "infra-network", Description: "共享存储、对象存储与备份恢复支持。", Icon: "Database", SortOrder: 3},

		// ── 应用与平台支持 子分类 ─────────────────────────────
		{Name: "企业应用支持", Code: "application-platform:business-app", ParentCode: "application-platform", Description: "内部业务系统和通用平台的日常支持。", Icon: "LayoutGrid", SortOrder: 1},
		{Name: "发布与变更协助", Code: "application-platform:release", ParentCode: "application-platform", Description: "发布窗口、变更执行与回滚协助。", Icon: "Container", SortOrder: 2},
		{Name: "数据库支持", Code: "application-platform:database", ParentCode: "application-platform", Description: "数据库开通、巡检与性能支持。", Icon: "Database", SortOrder: 3},

		// ── 安全与合规 子分类 ─────────────────────────────────
		{Name: "安全事件协助", Code: "security-compliance:incident", ParentCode: "security-compliance", Description: "安全事件上报、分析与应急支持。", Icon: "Bell", SortOrder: 1},
		{Name: "漏洞与基线", Code: "security-compliance:vulnerability", ParentCode: "security-compliance", Description: "漏洞修复、基线加固与巡检协助。", Icon: "Bug", SortOrder: 2},
		{Name: "审计与合规支持", Code: "security-compliance:audit", ParentCode: "security-compliance", Description: "审计材料准备、合规检查与追踪协助。", Icon: "FileSearch", SortOrder: 3},

		// ── 监控与告警 子分类 ─────────────────────────────────
		{Name: "监控接入", Code: "monitoring-alerting:onboarding", ParentCode: "monitoring-alerting", Description: "新增监控项、采集接入和指标配置。", Icon: "LineChart", SortOrder: 1},
		{Name: "告警治理", Code: "monitoring-alerting:governance", ParentCode: "monitoring-alerting", Description: "告警收敛、规则优化与噪音治理。", Icon: "BellRing", SortOrder: 2},
		{Name: "值班与通知策略", Code: "monitoring-alerting:oncall", ParentCode: "monitoring-alerting", Description: "值班排班、升级策略和通知链路维护。", Icon: "Clock", SortOrder: 3},
	}

	// First pass: create root catalogs (no parent)
	for _, s := range seeds {
		if s.ParentCode != "" {
			continue
		}
		if ok, err := upsertSeedCatalog(db, ServiceCatalog{
			Name: s.Name, Code: s.Code, Description: s.Description,
			Icon: s.Icon, SortOrder: s.SortOrder, IsActive: true,
		}); err != nil {
			slog.Error("seed: failed to create catalog", "code", s.Code, "error", err)
			continue
		} else if !ok {
			continue
		}
		slog.Info("seed: created catalog", "code", s.Code, "name", s.Name)
	}

	// Second pass: create child catalogs
	for _, s := range seeds {
		if s.ParentCode == "" {
			continue
		}
		var existing ServiceCatalog
		if err := db.Where("code = ?", s.Code).First(&existing).Error; err == nil {
			continue
		}
		var parent ServiceCatalog
		if err := db.Where("code = ?", s.ParentCode).First(&parent).Error; err != nil {
			slog.Error("seed: parent catalog not found", "code", s.Code, "parentCode", s.ParentCode, "error", err)
			continue
		}
		cat := ServiceCatalog{
			Name: s.Name, Code: s.Code, Description: s.Description,
			Icon: s.Icon, ParentID: &parent.ID, SortOrder: s.SortOrder, IsActive: true,
		}
		if ok, err := upsertSeedCatalog(db, cat); err != nil {
			slog.Error("seed: failed to create catalog", "code", s.Code, "error", err)
			continue
		} else if !ok {
			continue
		}
		slog.Info("seed: created catalog", "code", s.Code, "name", s.Name)
	}

	return nil
}

func upsertSeedCatalog(db *gorm.DB, cat ServiceCatalog) (bool, error) {
	var existing ServiceCatalog
	if err := db.Where("code = ?", cat.Code).First(&existing).Error; err == nil {
		return false, nil
	}
	if err := db.Unscoped().Where("code = ?", cat.Code).First(&existing).Error; err == nil {
		updates := map[string]any{
			"name":        cat.Name,
			"description": cat.Description,
			"icon":        cat.Icon,
			"parent_id":   cat.ParentID,
			"sort_order":  cat.SortOrder,
			"is_active":   true,
			"deleted_at":  nil,
		}
		return true, db.Unscoped().Model(&ServiceCatalog{}).Where("id = ?", existing.ID).Updates(updates).Error
	}
	return true, db.Create(&cat).Error
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

	// Migrate: flatten old "工单管理" intermediate directory
	var ticketDir model.Menu
	if err := db.Where("permission = ?", "itsm:ticket").First(&ticketDir).Error; err == nil {
		// Move children to ITSM top-level
		db.Model(&model.Menu{}).Where("parent_id = ?", ticketDir.ID).Update("parent_id", itsmDir.ID)
		// Soft-delete the intermediate directory
		db.Delete(&ticketDir)
		slog.Info("seed: flattened ticket menu directory", "oldId", ticketDir.ID)
	}

	// Migrate: remove standalone "服务目录" menu, catalog management is now inline in services page
	var oldCatalogMenu model.Menu
	if err := db.Where("permission = ?", "itsm:catalog:list").First(&oldCatalogMenu).Error; err == nil {
		// Delete associated buttons
		db.Where("parent_id = ?", oldCatalogMenu.ID).Delete(&model.Menu{})
		db.Delete(&oldCatalogMenu)
		slog.Info("seed: removed standalone catalog menu", "oldId", oldCatalogMenu.ID)
	}

	// Migrate: rename "服务定义" to "服务目录" for unified workspace
	var existingServiceMenu model.Menu
	if err := db.Where("permission = ?", "itsm:service:list").First(&existingServiceMenu).Error; err == nil {
		if existingServiceMenu.Name == "服务定义" {
			db.Model(&existingServiceMenu).Update("name", "服务目录")
			slog.Info("seed: renamed service menu to 服务目录")
		}
	}

	// 服务目录 (unified workspace: catalogs + services)
	serviceMenu := seedMenu(db, &itsmDir.ID, "服务目录", model.MenuTypeMenu, "/itsm/services", "Cog", "itsm:service:list", 0)
	seedButtons(db, serviceMenu, []model.Menu{
		{Name: "新增服务", Type: model.MenuTypeButton, Permission: "itsm:service:create", Sort: 0},
		{Name: "编辑服务", Type: model.MenuTypeButton, Permission: "itsm:service:update", Sort: 1},
		{Name: "删除服务", Type: model.MenuTypeButton, Permission: "itsm:service:delete", Sort: 2},
		{Name: "新增分类", Type: model.MenuTypeButton, Permission: "itsm:catalog:create", Sort: 3},
		{Name: "编辑分类", Type: model.MenuTypeButton, Permission: "itsm:catalog:update", Sort: 4},
		{Name: "删除分类", Type: model.MenuTypeButton, Permission: "itsm:catalog:delete", Sort: 5},
	})

	// 全部工单
	allTicketMenu := seedMenu(db, &itsmDir.ID, "全部工单", model.MenuTypeMenu, "/itsm/tickets", "List", "itsm:ticket:list", 2)
	seedButtons(db, allTicketMenu, []model.Menu{
		{Name: "创建工单", Type: model.MenuTypeButton, Permission: "itsm:ticket:create", Sort: 0},
		{Name: "指派工单", Type: model.MenuTypeButton, Permission: "itsm:ticket:assign", Sort: 1},
		{Name: "完结工单", Type: model.MenuTypeButton, Permission: "itsm:ticket:complete", Sort: 2},
		{Name: "取消工单", Type: model.MenuTypeButton, Permission: "itsm:ticket:cancel", Sort: 3},
		{Name: "工单覆写", Type: model.MenuTypeButton, Permission: "itsm:ticket:override", Sort: 4},
	})

	// 我的工单
	seedMenu(db, &itsmDir.ID, "我的工单", model.MenuTypeMenu, "/itsm/tickets/mine", "User", "itsm:ticket:mine", 3)
	// 我的待办
	seedMenu(db, &itsmDir.ID, "我的待办", model.MenuTypeMenu, "/itsm/tickets/todo", "Clock", "itsm:ticket:todo", 4)
	// 历史工单
	seedMenu(db, &itsmDir.ID, "历史工单", model.MenuTypeMenu, "/itsm/tickets/history", "Archive", "itsm:ticket:history", 5)

	// 我的审批
	seedMenu(db, &itsmDir.ID, "我的审批", model.MenuTypeMenu, "/itsm/tickets/approvals", "CheckCircle", "itsm:ticket:approvals", 6)

	// 优先级管理
	priorityMenu := seedMenu(db, &itsmDir.ID, "优先级管理", model.MenuTypeMenu, "/itsm/priorities", "Flag", "itsm:priority:list", 7)
	seedButtons(db, priorityMenu, []model.Menu{
		{Name: "新增优先级", Type: model.MenuTypeButton, Permission: "itsm:priority:create", Sort: 0},
		{Name: "编辑优先级", Type: model.MenuTypeButton, Permission: "itsm:priority:update", Sort: 1},
		{Name: "删除优先级", Type: model.MenuTypeButton, Permission: "itsm:priority:delete", Sort: 2},
	})

	// SLA 管理
	slaMenu := seedMenu(db, &itsmDir.ID, "SLA 管理", model.MenuTypeMenu, "/itsm/sla", "Timer", "itsm:sla:list", 8)
	seedButtons(db, slaMenu, []model.Menu{
		{Name: "新增SLA", Type: model.MenuTypeButton, Permission: "itsm:sla:create", Sort: 0},
		{Name: "编辑SLA", Type: model.MenuTypeButton, Permission: "itsm:sla:update", Sort: 1},
		{Name: "删除SLA", Type: model.MenuTypeButton, Permission: "itsm:sla:delete", Sort: 2},
	})

	// 引擎配置
	seedMenu(db, &itsmDir.ID, "引擎配置", model.MenuTypeMenu, "/itsm/engine-config", "Settings", "itsm:engine:config", 9)

	// 表单管理 - migrated away: remove menu and buttons
	var formMenu model.Menu
	if err := db.Where("permission = ?", "itsm:form:list").First(&formMenu).Error; err == nil {
		db.Where("parent_id = ?", formMenu.ID).Delete(&model.Menu{})
		db.Delete(&formMenu)
		slog.Info("seed: removed form management menu")
	}

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
	} else if menu.Sort != sort || (parentID != nil && (menu.ParentID == nil || *menu.ParentID != *parentID)) {
		// Sync sort and parent if drifted
		db.Model(&menu).Updates(map[string]any{"sort": sort, "parent_id": parentID})
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
		// Service Knowledge Documents
		{"admin", "/api/v1/itsm/services/:id/knowledge-documents", "POST"},
		{"admin", "/api/v1/itsm/services/:id/knowledge-documents", "GET"},
		{"admin", "/api/v1/itsm/services/:id/knowledge-documents/:docId", "DELETE"},
		// Engine Config
		{"admin", "/api/v1/itsm/engine/config", "GET"},
		{"admin", "/api/v1/itsm/engine/config", "PUT"},
		// Workflow Generate
		{"admin", "/api/v1/itsm/workflows/generate", "POST"},
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
		// Process variables
		{"admin", "/api/v1/itsm/tickets/:id/variables", "GET"},
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
		{"admin", "itsm:ticket:override", "read"},
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
		{"admin", "itsm:engine:config", "read"},
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
		{Name: "快速办公支持", Code: "rapid-workplace", Description: "适用于办公终端、账号开通、基础软件支持等高频轻量服务", ResponseMinutes: 15, ResolutionMinutes: 120, IsActive: true},
		{Name: "关键业务", Code: "critical-business", Description: "适用于影响关键业务连续性的高优先级服务与紧急支持场景", ResponseMinutes: 10, ResolutionMinutes: 60, IsActive: true},
		{Name: "基础设施变更", Code: "infra-change", Description: "适用于服务器、网络、数据库等基础设施类服务和变更协作", ResponseMinutes: 60, ResolutionMinutes: 480, IsActive: true},
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

func seedServiceDefinitions(db *gorm.DB) error {
	// Look up the decision agent for smart services
	var decisionAgentID *uint
	var agentRow struct{ ID uint }
	if err := db.Table("ai_agents").Where("code = ?", "itsm.decision").Select("id").First(&agentRow).Error; err == nil {
		decisionAgentID = &agentRow.ID
		slog.Info("seed: found decision agent for smart services", "agentId", agentRow.ID)
	} else {
		slog.Warn("seed: decision agent (code=itsm.decision) not found, smart services will have no agent")
	}

	type serviceSeed struct {
		Name              string
		Code              string
		Description       string
		CatalogCode       string
		SLACode           string
		IntakeFormSchema  string
		CollaborationSpec string
		Actions           []ServiceAction
	}

	serviceRequestFormSchema := `{"version":1,"fields":[{"key":"title","type":"text","label":"请求标题","required":true,"validation":[{"rule":"required","message":"请输入请求标题"}],"width":"full"},{"key":"description","type":"textarea","label":"请求描述","required":true,"validation":[{"rule":"required","message":"请输入请求描述"}],"width":"full","props":{"rows":4}},{"key":"expected_date","type":"date","label":"期望完成日期","width":"half"},{"key":"remarks","type":"textarea","label":"备注","width":"full","props":{"rows":3}}],"layout":{"columns":2,"sections":[{"title":"请求信息","fields":["title","description"]},{"title":"补充信息","fields":["expected_date","remarks"]}]}}`

	seeds := []serviceSeed{
		{
			Name:              "Copilot 账号申请",
			Code:              "copilot-account-request",
			Description:       "用于验证服务申请与审批闭环的内置服务。",
			CatalogCode:       "account-access:provisioning",
			SLACode:           "rapid-workplace",
			IntakeFormSchema:  serviceRequestFormSchema,
			CollaborationSpec: "收集提单用户的Github账号信息和申请理由（可选），交给信息部的IT管理员审批，审批通过后结束流程。",
		},
		{
			Name:              "高风险变更协同申请（Boss）",
			Code:              "boss-serial-change-request",
			Description:       "用于在系统内直接查看复杂表单、表格明细与两级串签流程图的 Boss 级内置服务。",
			CatalogCode:       "application-platform:release",
			SLACode:           "infra-change",
			CollaborationSpec: `用户通过 IT 服务台提交高风险变更协同申请。服务台需要收集申请主题、申请类别、风险等级、期望完成时间、变更开始时间、变更结束时间、影响范围、回滚要求、影响模块以及变更明细表。申请类别必须支持：生产变更(prod_change)、访问授权(access_grant)、应急支持(emergency_support)。风险等级必须支持：低(low)、中(medium)、高(high)。回滚要求必须支持：需要(required)、不需要(not_required)。影响模块必须支持多选：网关(gateway)、支付(payment)、监控(monitoring)、订单(order)。变更明细表至少包含系统、资源、权限级别、生效时段、变更理由。权限级别必须支持：只读(read)、读写(read_write)。申请提交后，先交给指定用户 serial-reviewer 审批，审批参与者类型必须使用 user。serial-reviewer 审批通过后，再交给信息部的运维管理员岗位审批，审批参与者类型必须使用 position_department，部门编码使用 it，岗位编码使用 ops_admin。运维管理员审批通过后直接结束流程，不要生成驳回分支。`,
		},
		{
			Name:              "生产数据库备份白名单临时放行申请",
			Code:              "db-backup-whitelist-action-e2e",
			Description:       "用于验证请求节点预检动作、审批后自动放行动作与工单闭环。",
			CatalogCode:       "application-platform:database",
			SLACode:           "infra-change",
			CollaborationSpec: `用户提交生产数据库备份白名单临时放行申请。系统先进入申请人请求节点，并在进入节点时自动执行预检动作，校验目标数据库、运维来源 IP 和放行时间窗信息是否齐备。申请人提交后，交给信息部的数据库管理员岗位审批，审批参与者类型必须使用 position_department，部门编码使用 it，岗位编码使用 db_admin。审批通过后，在离开审批节点时自动执行白名单放行动作，并在动作成功后直接结束流程。`,
			Actions: []ServiceAction{
				{
					Name: "备份白名单预检", Code: "backup_whitelist_precheck",
					Description: "在申请人提交前校验数据库、时间窗与来源 IP 是否齐备。",
					ActionType:  "http", IsActive: true,
					ConfigJSON: JSONField(`{"url":"/precheck","method":"POST","timeout_seconds":5}`),
				},
				{
					Name: "执行备份白名单放行", Code: "backup_whitelist_apply",
					Description: "审批通过后自动执行数据库备份白名单放行。",
					ActionType:  "http", IsActive: true,
					ConfigJSON: JSONField(`{"url":"/apply","method":"POST","timeout_seconds":5}`),
				},
			},
		},
		{
			Name:              "生产服务器临时访问申请",
			Code:              "prod-server-temporary-access",
			Description:       "用于验证生产服务器临时访问在主机运维、网络诊断与安全审计语境下的真实分支审批。",
			CatalogCode:       "infra-network:compute",
			SLACode:           "critical-business",
			CollaborationSpec: `用户通过 IT 服务台提交生产服务器临时访问申请。服务台需要收集访问服务器、访问时段、操作目的和访问原因。如果访问原因属于应用发布、进程排障、日志排查、磁盘清理、主机巡检或生产运维操作，则交给信息部的运维管理员岗位审批，审批参与者类型必须使用 position_department，部门编码使用 it，岗位编码使用 ops_admin。如果访问原因属于网络抓包、连通性诊断、ACL 调整、负载均衡变更或防火墙策略调整，则交给信息部的网络管理员岗位审批，审批参与者类型必须使用 position_department，部门编码使用 it，岗位编码使用 network_admin。如果访问原因属于安全审计、入侵排查、漏洞修复验证、取证分析或合规检查，则交给信息部的信息安全管理员岗位审批，审批参与者类型必须使用 position_department，部门编码使用 it，岗位编码使用 security_admin。审批通过后直接结束流程，不要生成驳回分支。`,
		},
		{
			Name:              "VPN 开通申请",
			Code:              "vpn-access-request",
			Description:       "用于验证 VPN 开通申请在服务匹配、拟提单确认与分支审批下的完整闭环。",
			CatalogCode:       "infra-network:network",
			SLACode:           "standard",
			CollaborationSpec: `用户通过 IT 服务台提交 VPN 开通申请。服务台需要收集 VPN 账号、设备与用途说明、访问原因。如果访问原因属于线上支持、故障排查、生产应急或网络接入问题，则交给信息部的网络管理员岗位审批，审批参与者类型必须使用 position_department，部门编码使用 it，岗位编码使用 network_admin。如果访问原因属于外部协作、长期远程办公、跨境访问或安全合规事项，则交给信息部的信息安全管理员岗位审批，审批参与者类型必须使用 position_department，部门编码使用 it，岗位编码使用 security_admin。审批通过后直接结束流程，不要生成驳回分支。`,
		},
	}

	for _, s := range seeds {
		var existing ServiceDefinition
		if err := db.Where("code = ?", s.Code).First(&existing).Error; err == nil {
			continue
		}

		var catalog ServiceCatalog
		if err := db.Where("code = ?", s.CatalogCode).First(&catalog).Error; err != nil {
			slog.Error("seed: catalog not found for service", "serviceCode", s.Code, "catalogCode", s.CatalogCode, "error", err)
			continue
		}

		var slaID *uint
		var sla SLATemplate
		if err := db.Where("code = ?", s.SLACode).First(&sla).Error; err == nil {
			slaID = &sla.ID
		} else {
			slog.Warn("seed: SLA not found for service, setting to nil", "serviceCode", s.Code, "slaCode", s.SLACode)
		}

		var intakeFormSchema JSONField
		if s.IntakeFormSchema != "" {
			intakeFormSchema = JSONField(s.IntakeFormSchema)
		}

		svc := ServiceDefinition{
			Name:              s.Name,
			Code:              s.Code,
			Description:       s.Description,
			CatalogID:         catalog.ID,
			EngineType:        "smart",
			SLAID:             slaID,
			IntakeFormSchema:  intakeFormSchema,
			AgentID:           decisionAgentID,
			CollaborationSpec: s.CollaborationSpec,
			IsActive:          true,
		}
		if err := db.Create(&svc).Error; err != nil {
			slog.Error("seed: failed to create service definition", "code", s.Code, "error", err)
			continue
		}
		slog.Info("seed: created service definition", "code", s.Code, "name", s.Name)

		for _, action := range s.Actions {
			var existingAction ServiceAction
			if err := db.Where("service_id = ? AND code = ?", svc.ID, action.Code).First(&existingAction).Error; err == nil {
				continue
			}
			action.ServiceID = svc.ID
			if err := db.Create(&action).Error; err != nil {
				slog.Error("seed: failed to create service action", "serviceCode", s.Code, "actionCode", action.Code, "error", err)
				continue
			}
			slog.Info("seed: created service action", "serviceCode", s.Code, "actionCode", action.Code)
		}
	}

	// Backfill: update existing smart services that have no agent
	if decisionAgentID != nil {
		db.Model(&ServiceDefinition{}).
			Where("engine_type = ? AND agent_id IS NULL", "smart").
			Update("agent_id", *decisionAgentID)
	}

	return nil
}

// seedEngineConfig creates internal agents and default SystemConfig for ITSM engine.
func seedEngineConfig(db *gorm.DB) error {
	type agentSeed struct {
		Name         string
		Code         string
		SystemPrompt string
		Temperature  float64
	}

	agents := []agentSeed{
		{
			Name:         "ITSM 工作流解析",
			Code:         "itsm.generator",
			Temperature:  0.3,
			SystemPrompt: itsmGeneratorSystemPrompt,
		},
	}

	for _, a := range agents {
		var existing struct{ ID uint }
		if err := db.Table("ai_agents").Where("code = ?", a.Code).Select("id").First(&existing).Error; err == nil {
			continue
		}
		if err := db.Table("ai_agents").Create(map[string]any{
			"name":          a.Name,
			"code":          a.Code,
			"type":          "internal",
			"system_prompt": a.SystemPrompt,
			"temperature":   a.Temperature,
			"is_active":     true,
			"visibility":    "team",
			"created_by":    0,
		}).Error; err != nil {
			slog.Error("seed: failed to create internal agent", "code", a.Code, "error", err)
			continue
		}
		slog.Info("seed: created internal agent", "code", a.Code, "name", a.Name)
	}

	defaults := map[string]string{
		"itsm.engine.decision.decision_mode":  "direct_first",
		"itsm.engine.general.max_retries":     "3",
		"itsm.engine.general.timeout_seconds": "120",
		"itsm.engine.general.reasoning_log":   "full",
	}

	for key, value := range defaults {
		var existing model.SystemConfig
		if err := db.Where("\"key\" = ?", key).First(&existing).Error; err == nil {
			continue
		}
		cfg := model.SystemConfig{Key: key, Value: value}
		if err := db.Create(&cfg).Error; err != nil {
			slog.Error("seed: failed to create system config", "key", key, "error", err)
			continue
		}
		slog.Info("seed: created system config", "key", key, "value", value)
	}

	// Seed default agent_id for servicedesk and decision from preset agents
	agentDefaults := map[string]string{
		"itsm.engine.servicedesk.agent_id": "itsm.servicedesk",
		"itsm.engine.decision.agent_id":    "itsm.decision",
	}
	for configKey, agentCode := range agentDefaults {
		var existing model.SystemConfig
		if err := db.Where("\"key\" = ?", configKey).First(&existing).Error; err == nil {
			continue
		}
		value := "0"
		var agentRow struct{ ID uint }
		if err := db.Table("ai_agents").Where("code = ?", agentCode).Select("id").First(&agentRow).Error; err == nil {
			value = strconv.FormatUint(uint64(agentRow.ID), 10)
		}
		cfg := model.SystemConfig{Key: configKey, Value: value}
		if err := db.Create(&cfg).Error; err != nil {
			slog.Error("seed: failed to create system config", "key", configKey, "error", err)
			continue
		}
		slog.Info("seed: created system config", "key", configKey, "value", value)
	}

	return nil
}

const itsmGeneratorSystemPrompt = `你是 ITSM 工作流解析引擎。根据用户的协作规范（Collaboration Spec）生成工作流 JSON。

## 输出格式

输出必须是合法 JSON，包含 nodes 和 edges 两个数组：

{
  "nodes": [
    {
      "id": "string (唯一标识，如 node_1)",
      "type": "string (节点类型，见下方枚举)",
      "position": {"x": number, "y": number},
      "data": {
        "label": "string (节点显示名称)",
        "nodeType": "string (与外层 type 相同)",
        ... (其他字段见下方说明)
      }
    }
  ],
  "edges": [
    {
      "id": "string (唯一标识，如 edge_1)",
      "source": "string (源节点 id)",
      "target": "string (目标节点 id)",
      "data": {
        "outcome": "string (可选，如 approved/rejected)",
        "default": boolean (可选，网关默认路径)
      }
    }
  ]
}

## 节点类型（type）枚举

| 类型 | 说明 | data 必需字段 |
|------|------|--------------|
| start | 起始节点（有且仅有一个） | label, nodeType |
| end | 结束节点（至少一个） | label, nodeType |
| form | 表单填写节点 | label, nodeType, participants, formSchema |
| approve | 审批节点 | label, nodeType, participants, executionMode(single/parallel/sequential) |
| process | 人工处理节点 | label, nodeType, participants |
| action | 自动动作节点（webhook/脚本） | label, nodeType, actionId (关联可用动作) |
| exclusive | 排他网关（条件分支） | label, nodeType (至少两条出边) |
| notify | 通知节点 | label, nodeType |
| wait | 等待节点（定时/信号） | label, nodeType, waitMode(signal/timer), duration(如 "2h") |

**重要**：每个节点的 data 中必须包含 nodeType 字段，值与外层 type 一致。

## 参与人（participants）格式

participants 是数组，每个元素：
- type: "user" | "position" | "department" | "position_department" | "requester_manager"

各类型的附加字段：
- user: name（用户名或标识）
- position: name（岗位名称）
- department: name（部门名称）
- position_department: department_code（部门编码）+ position_code（岗位编码）
- requester_manager: 无附加字段

当协作规范中提到"提交人的直属上级"或"发起人经理"时，使用 requester_manager 类型。
当提到具体岗位（如"IT主管"）时，使用 position 类型。
当提到部门（如"IT部门"）时，使用 department 类型。
当提到特定部门中的特定岗位（如"信息部的网络管理员"）时，使用 position_department 类型，设置 department_code 和 position_code。
当提到具体用户（如"serial-reviewer"）时，使用 user 类型，设置 name。

## 表单字段（formSchema）格式

form 节点必须包含 formSchema，描述该节点需要收集的字段：

{
  "fields": [
    { "key": "request_kind", "type": "select", "label": "请求类型", "options": ["VPN新开通", "VPN故障排查", "网络支持"] },
    { "key": "urgency", "type": "select", "label": "紧急程度", "options": ["低", "中", "高", "紧急"] },
    { "key": "description", "type": "textarea", "label": "问题描述" },
    { "key": "contact_phone", "type": "text", "label": "联系电话" }
  ]
}

字段 type 可选值：text, textarea, select, number, date, checkbox
根据协作规范中描述的业务场景，推断合理的表单字段。排他网关 condition 中引用的 form.xxx 字段必须在上游 form 节点的 formSchema.fields 中有对应 key。

## 排他网关（exclusive）条件格式

排他网关的路由条件配置在**出边的 data.condition** 中（不是节点上）：

条件边的 data：
{
  "condition": {
    "field": "form.request_kind",
    "operator": "equals",
    "value": "network_support",
    "edge_id": "edge_xxx"
  }
}

默认边（兜底）的 data：
{
  "default": true
}

condition 字段说明：
- field: 条件字段路径（如 "form.urgency", "form.request_kind"）
- operator: equals | not_equals | contains_any | gt | lt | gte | lte
- value: 比较值
- edge_id: 此条件对应的出边 id

排他网关必须有至少两条出边，其中一条应标记 data.default = true 作为兜底。

## 布局规则

- 起始节点 position 从 {x: 400, y: 50} 开始
- 纵向排列，每层间距约 150px
- 并行分支横向展开，间距约 250px

## 约束

1. 严格基于协作规范描述，不发明未提及的角色、部门或步骤
2. 每条从 start 到 end 的路径必须连通，不能有孤立节点
3. 开始节点有且仅有一条出边，无入边
4. 结束节点无出边
5. 仅输出 JSON，不要包含任何解释文字或 markdown 标记`

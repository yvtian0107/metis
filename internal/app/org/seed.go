package org

import (
	"log/slog"

	"github.com/casbin/casbin/v2"
	"gorm.io/gorm"

	"metis/internal/model"
)

func seedOrg(db *gorm.DB, enforcer *casbin.Enforcer, install bool) error {
	// Migration: drop old unique index on (user_id, department_id) in favor of (user_id, department_id, position_id)
	if db.Migrator().HasIndex(&UserPosition{}, "idx_user_pos_user_dep") {
		if err := db.Migrator().DropIndex(&UserPosition{}, "idx_user_pos_user_dep"); err != nil {
			slog.Warn("seed: failed to drop old index idx_user_pos_user_dep", "error", err)
		} else {
			slog.Info("seed: dropped old index idx_user_pos_user_dep")
		}
	}

	// 1. Seed menus: 组织管理 directory
	var orgDir model.Menu
	if err := db.Where("permission = ?", "org").First(&orgDir).Error; err != nil {
		orgDir = model.Menu{
			Name:       "组织管理",
			Type:       model.MenuTypeDirectory,
			Icon:       "Users",
			Permission: "org",
			Sort:       350,
		}
		if err := db.Create(&orgDir).Error; err != nil {
			return err
		}
		slog.Info("seed: created menu", "name", orgDir.Name, "permission", orgDir.Permission)
	}

	// 部门管理 menu (renamed to 组织架构)
	var deptMenu model.Menu
	if err := db.Where("permission = ?", "org:department:list").First(&deptMenu).Error; err != nil {
		deptMenu = model.Menu{
			ParentID:   &orgDir.ID,
			Name:       "组织架构",
			Type:       model.MenuTypeMenu,
			Path:       "/org/departments",
			Icon:       "Network",
			Permission: "org:department:list",
			Sort:       0,
		}
		if err := db.Create(&deptMenu).Error; err != nil {
			return err
		}
		slog.Info("seed: created menu", "name", deptMenu.Name, "permission", deptMenu.Permission)
	} else if deptMenu.Name == "部门管理" {
		db.Model(&deptMenu).Update("name", "组织架构")
		slog.Info("seed: renamed menu", "old", "部门管理", "new", "组织架构")
	}

	deptButtons := []model.Menu{
		{Name: "新增部门", Type: model.MenuTypeButton, Permission: "org:department:create", Sort: 0},
		{Name: "编辑部门", Type: model.MenuTypeButton, Permission: "org:department:update", Sort: 1},
		{Name: "删除部门", Type: model.MenuTypeButton, Permission: "org:department:delete", Sort: 2},
	}
	for _, btn := range deptButtons {
		var existing model.Menu
		if err := db.Where("permission = ?", btn.Permission).First(&existing).Error; err != nil {
			btn.ParentID = &deptMenu.ID
			if err := db.Create(&btn).Error; err != nil {
				slog.Error("seed: failed to create button menu", "permission", btn.Permission, "error", err)
				continue
			}
			slog.Info("seed: created menu", "name", btn.Name, "permission", btn.Permission)
		}
	}

	// 岗位管理 menu
	var posMenu model.Menu
	if err := db.Where("permission = ?", "org:position:list").First(&posMenu).Error; err != nil {
		posMenu = model.Menu{
			ParentID:   &orgDir.ID,
			Name:       "岗位管理",
			Type:       model.MenuTypeMenu,
			Path:       "/org/positions",
			Icon:       "Briefcase",
			Permission: "org:position:list",
			Sort:       1,
		}
		if err := db.Create(&posMenu).Error; err != nil {
			return err
		}
		slog.Info("seed: created menu", "name", posMenu.Name, "permission", posMenu.Permission)
	}

	posButtons := []model.Menu{
		{Name: "新增岗位", Type: model.MenuTypeButton, Permission: "org:position:create", Sort: 0},
		{Name: "编辑岗位", Type: model.MenuTypeButton, Permission: "org:position:update", Sort: 1},
		{Name: "删除岗位", Type: model.MenuTypeButton, Permission: "org:position:delete", Sort: 2},
	}
	for _, btn := range posButtons {
		var existing model.Menu
		if err := db.Where("permission = ?", btn.Permission).First(&existing).Error; err != nil {
			btn.ParentID = &posMenu.ID
			if err := db.Create(&btn).Error; err != nil {
				slog.Error("seed: failed to create button menu", "permission", btn.Permission, "error", err)
				continue
			}
			slog.Info("seed: created menu", "name", btn.Name, "permission", btn.Permission)
		}
	}

	// 人员分配 menu — removed from UI (merged into department detail page)
	// Soft-delete if it exists from a previous install
	var assignMenu model.Menu
	if err := db.Where("permission = ?", "org:assignment:list").First(&assignMenu).Error; err == nil {
		db.Delete(&assignMenu)
		// Also remove child buttons
		db.Where("parent_id = ?", assignMenu.ID).Delete(&model.Menu{})
		slog.Info("seed: removed assignment menu (merged into department detail)")
	}

	// 2. Seed Casbin policies for admin role
	policies := [][]string{
		// Departments
		{"admin", "/api/v1/org/departments", "POST"},
		{"admin", "/api/v1/org/departments", "GET"},
		{"admin", "/api/v1/org/departments/tree", "GET"},
		{"admin", "/api/v1/org/departments/:id", "GET"},
		{"admin", "/api/v1/org/departments/:id", "PUT"},
		{"admin", "/api/v1/org/departments/:id", "DELETE"},
		{"admin", "/api/v1/org/departments/:id/positions", "GET"},
		{"admin", "/api/v1/org/departments/:id/positions", "PUT"},
		// Positions
		{"admin", "/api/v1/org/positions", "POST"},
		{"admin", "/api/v1/org/positions", "GET"},
		{"admin", "/api/v1/org/positions/:id", "GET"},
		{"admin", "/api/v1/org/positions/:id", "PUT"},
		{"admin", "/api/v1/org/positions/:id", "DELETE"},
		// Assignments
		{"admin", "/api/v1/org/users/:id/positions", "GET"},
		{"admin", "/api/v1/org/users/:id/positions", "POST"},
		{"admin", "/api/v1/org/users/:id/positions/:assignmentId", "PUT"},
		{"admin", "/api/v1/org/users/:id/positions/:assignmentId", "DELETE"},
		{"admin", "/api/v1/org/users/:id/positions/:assignmentId/primary", "PUT"},
		{"admin", "/api/v1/org/users/:id/departments/:deptId/positions", "PUT"},
		{"admin", "/api/v1/org/users", "GET"},
	}

	menuPerms := [][]string{
		{"admin", "org", "read"},
		{"admin", "org:department:list", "read"},
		{"admin", "org:department:create", "read"},
		{"admin", "org:department:update", "read"},
		{"admin", "org:department:delete", "read"},
		{"admin", "org:position:list", "read"},
		{"admin", "org:position:create", "read"},
		{"admin", "org:position:update", "read"},
		{"admin", "org:position:delete", "read"},
		{"admin", "org:assignment:list", "read"},
		{"admin", "org:assignment:create", "read"},
		{"admin", "org:assignment:update", "read"},
		{"admin", "org:assignment:delete", "read"},
	}

	allPolicies := append(policies, menuPerms...)
	for _, p := range allPolicies {
		if has, _ := enforcer.HasPolicy(p); !has {
			if _, err := enforcer.AddPolicy(p); err != nil {
				slog.Error("seed: failed to add policy", "policy", p, "error", err)
			}
		}
	}

	if install {
		if err := seedDepartments(db); err != nil {
			return err
		}
		if err := seedPositions(db); err != nil {
			return err
		}
		if err := seedDepartmentPositions(db); err != nil {
			return err
		}
	}

	// Seed org_context builtin tool into ai_tools (if AI App tables exist)
	if db.Migrator().HasTable("ai_tools") {
		var count int64
		db.Table("ai_tools").Where("name = ?", "organization.org_context").Count(&count)
		if count == 0 {
			if err := db.Table("ai_tools").Create(map[string]any{
				"toolkit":           "organization",
				"name":              "organization.org_context",
				"display_name":      "组织架构查询",
				"description":       "读取人员、部门、岗位关系信息，用于流程决策和参与者解析。支持按用户名、部门代码、岗位代码筛选。",
				"parameters_schema": `{"type":"object","properties":{"username":{"type":"string","description":"按用户名查询"},"department_code":{"type":"string","description":"按部门代码筛选"},"position_code":{"type":"string","description":"按岗位代码筛选"},"include_inactive":{"type":"boolean","description":"是否包含停用记录，默认 false"}}}`,
				"is_active":         true,
			}).Error; err != nil {
				slog.Warn("seed: failed to create org_context tool", "error", err)
			} else {
				slog.Info("seed: created org_context builtin tool")
			}
		}
	}

	return nil
}

func seedDepartments(db *gorm.DB) error {
	// Root department
	var hq Department
	if err := db.Where("code = ?", "headquarters").First(&hq).Error; err != nil {
		hq = Department{
			Name:        "总部",
			Code:        "headquarters",
			Description: "公司总部",
			Sort:        0,
			IsActive:    true,
		}
		if err := db.Create(&hq).Error; err != nil {
			return err
		}
		slog.Info("seed: created department", "name", hq.Name, "code", hq.Code)
	}

	children := []Department{
		{Name: "研发部", Code: "rd", Description: "负责产品研发", Sort: 1},
		{Name: "运维部", Code: "ops", Description: "负责系统运维", Sort: 2},
		{Name: "测试部", Code: "qa", Description: "负责质量测试", Sort: 3},
		{Name: "市场部", Code: "marketing", Description: "负责市场推广", Sort: 4},
		{Name: "销售部", Code: "sales", Description: "负责销售业务", Sort: 5},
		{Name: "信息部", Code: "it", Description: "负责信息技术支持", Sort: 6},
	}

	for _, dept := range children {
		var existing Department
		if err := db.Where("code = ?", dept.Code).First(&existing).Error; err != nil {
			dept.ParentID = &hq.ID
			dept.IsActive = true
			if err := db.Create(&dept).Error; err != nil {
				slog.Error("seed: failed to create department", "code", dept.Code, "error", err)
				continue
			}
			slog.Info("seed: created department", "name", dept.Name, "code", dept.Code)
		}
	}

	return nil
}

func seedPositions(db *gorm.DB) error {
	positions := []Position{
		{Name: "IT管理员", Code: "it_admin", Description: "负责IT基础设施的日常管理和维护", Sort: 1},
		{Name: "数据库管理员", Code: "db_admin", Description: "负责数据库系统的管理、维护和优化", Sort: 2},
		{Name: "网络管理员", Code: "network_admin", Description: "负责网络设备和网络安全的管理维护", Sort: 3},
		{Name: "安全管理员", Code: "security_admin", Description: "负责信息安全策略制定和安全事件响应", Sort: 4},
		{Name: "应用管理员", Code: "app_admin", Description: "负责业务应用系统的部署和运维管理", Sort: 5},
		{Name: "运维管理员", Code: "ops_admin", Description: "负责整体运维工作的协调和管理", Sort: 6},
		{Name: "总部助理", Code: "assistant", Description: "负责总部审批与流程协作", Sort: 7},
		{Name: "串行评审人", Code: "serial_reviewer", Description: "负责内置高风险变更流程的首级串行审批", Sort: 8},
	}

	for _, pos := range positions {
		var existing Position
		if err := db.Where("code = ?", pos.Code).First(&existing).Error; err != nil {
			pos.IsActive = true
			if err := db.Create(&pos).Error; err != nil {
				slog.Error("seed: failed to create position", "code", pos.Code, "error", err)
				continue
			}
			slog.Info("seed: created position", "name", pos.Name, "code", pos.Code)
		}
	}

	return nil
}

func seedDepartmentPositions(db *gorm.DB) error {
	// Map department codes to allowed position codes
	deptPositions := map[string][]string{
		"headquarters": {"assistant", "serial_reviewer"},
		"it":           {"it_admin", "network_admin", "security_admin", "db_admin", "ops_admin"},
		"rd":           {"app_admin"},
		"ops":          {"ops_admin", "it_admin", "network_admin"},
	}

	for deptCode, posCodes := range deptPositions {
		var dept Department
		if err := db.Where("code = ?", deptCode).First(&dept).Error; err != nil {
			continue
		}
		for _, posCode := range posCodes {
			var pos Position
			if err := db.Where("code = ?", posCode).First(&pos).Error; err != nil {
				continue
			}
			var existing DepartmentPosition
			if err := db.Where("department_id = ? AND position_id = ?", dept.ID, pos.ID).First(&existing).Error; err != nil {
				dp := DepartmentPosition{DepartmentID: dept.ID, PositionID: pos.ID}
				if err := db.Create(&dp).Error; err != nil {
					slog.Error("seed: failed to create dept-position", "dept", deptCode, "pos", posCode, "error", err)
					continue
				}
				slog.Info("seed: created dept-position", "dept", deptCode, "pos", posCode)
			}
		}
	}
	return nil
}

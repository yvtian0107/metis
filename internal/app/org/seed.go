package org

import (
	"log/slog"

	"github.com/casbin/casbin/v2"
	"gorm.io/gorm"

	"metis/internal/model"
)

func seedOrg(db *gorm.DB, enforcer *casbin.Enforcer, install bool) error {
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

	// 部门管理 menu
	var deptMenu model.Menu
	if err := db.Where("permission = ?", "org:department:list").First(&deptMenu).Error; err != nil {
		deptMenu = model.Menu{
			ParentID:   &orgDir.ID,
			Name:       "部门管理",
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

	// 人员分配 menu
	var assignMenu model.Menu
	if err := db.Where("permission = ?", "org:assignment:list").First(&assignMenu).Error; err != nil {
		assignMenu = model.Menu{
			ParentID:   &orgDir.ID,
			Name:       "人员分配",
			Type:       model.MenuTypeMenu,
			Path:       "/org/assignments",
			Icon:       "UserCog",
			Permission: "org:assignment:list",
			Sort:       2,
		}
		if err := db.Create(&assignMenu).Error; err != nil {
			return err
		}
		slog.Info("seed: created menu", "name", assignMenu.Name, "permission", assignMenu.Permission)
	}

	assignButtons := []model.Menu{
		{Name: "分配岗位", Type: model.MenuTypeButton, Permission: "org:assignment:create", Sort: 0},
		{Name: "编辑分配", Type: model.MenuTypeButton, Permission: "org:assignment:update", Sort: 1},
		{Name: "移除分配", Type: model.MenuTypeButton, Permission: "org:assignment:delete", Sort: 2},
	}
	for _, btn := range assignButtons {
		var existing model.Menu
		if err := db.Where("permission = ?", btn.Permission).First(&existing).Error; err != nil {
			btn.ParentID = &assignMenu.ID
			if err := db.Create(&btn).Error; err != nil {
				slog.Error("seed: failed to create button menu", "permission", btn.Permission, "error", err)
				continue
			}
			slog.Info("seed: created menu", "name", btn.Name, "permission", btn.Permission)
		}
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

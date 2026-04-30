package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"metis/internal/model"
	"metis/internal/repository"
	"metis/internal/service"
)

type RoleHandler struct {
	roleSvc   *service.RoleService
	casbinSvc *service.CasbinService
	menuSvc   *service.MenuService
	roleRepo  *repository.RoleRepo
}

func (h *RoleHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	roles, total, err := h.roleSvc.List(page, pageSize)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	OK(c, gin.H{
		"items":    roles,
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
	})
}

type createRoleReq struct {
	Name        string `json:"name" binding:"required"`
	Code        string `json:"code" binding:"required"`
	Description string `json:"description"`
	Sort        int    `json:"sort"`
}

func (h *RoleHandler) Create(c *gin.Context) {
	var req createRoleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	role, err := h.roleSvc.Create(req.Name, req.Code, req.Description, req.Sort)
	if err != nil {
		if errors.Is(err, service.ErrRoleCodeExists) {
			Fail(c, http.StatusBadRequest, "role code already exists")
			return
		}
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_action", "role.create")
	c.Set("audit_resource", "role")
	c.Set("audit_summary", "创建角色")
	OK(c, role)
}

func (h *RoleHandler) Get(c *gin.Context) {
	id, err := parseIDParam(c)
	if err != nil {
		return
	}

	role, deptIDs, err := h.roleSvc.GetByIDWithDeptScope(id)
	if err != nil {
		if errors.Is(err, service.ErrRoleNotFound) {
			Fail(c, http.StatusNotFound, "role not found")
			return
		}
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	OK(c, gin.H{
		"id":          role.ID,
		"name":        role.Name,
		"code":        role.Code,
		"description": role.Description,
		"sort":        role.Sort,
		"isSystem":    role.IsSystem,
		"dataScope":   role.DataScope,
		"deptIds":     deptIDs,
		"createdAt":   role.CreatedAt,
		"updatedAt":   role.UpdatedAt,
	})
}

type updateRoleReq struct {
	Name        *string `json:"name"`
	Code        *string `json:"code"`
	Description *string `json:"description"`
	Sort        *int    `json:"sort"`
}

func (h *RoleHandler) Update(c *gin.Context) {
	id, err := parseIDParam(c)
	if err != nil {
		return
	}

	var req updateRoleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	role, err := h.roleSvc.Update(id, service.UpdateRoleParams{
		Name:        req.Name,
		Code:        req.Code,
		Description: req.Description,
		Sort:        req.Sort,
	})
	if err != nil {
		switch {
		case errors.Is(err, service.ErrRoleNotFound):
			Fail(c, http.StatusNotFound, "role not found")
		case errors.Is(err, service.ErrRoleCodeExists):
			Fail(c, http.StatusBadRequest, "role code already exists")
		case errors.Is(err, service.ErrSystemRole):
			Fail(c, http.StatusBadRequest, "cannot modify system role code")
		default:
			Fail(c, http.StatusInternalServerError, err.Error())
		}
		return
	}

	c.Set("audit_action", "role.update")
	c.Set("audit_resource", "role")
	c.Set("audit_resource_id", strconv.FormatUint(uint64(id), 10))
	c.Set("audit_summary", "更新角色")
	OK(c, role)
}

func (h *RoleHandler) Delete(c *gin.Context) {
	id, err := parseIDParam(c)
	if err != nil {
		return
	}

	if err := h.roleSvc.Delete(id); err != nil {
		switch {
		case errors.Is(err, service.ErrRoleNotFound):
			Fail(c, http.StatusNotFound, "role not found")
		case errors.Is(err, service.ErrSystemRoleDel):
			Fail(c, http.StatusBadRequest, "cannot delete system role")
		case errors.Is(err, service.ErrRoleHasUsers):
			Fail(c, http.StatusBadRequest, "cannot delete role with assigned users")
		default:
			Fail(c, http.StatusInternalServerError, err.Error())
		}
		return
	}

	c.Set("audit_action", "role.delete")
	c.Set("audit_resource", "role")
	c.Set("audit_resource_id", strconv.FormatUint(uint64(id), 10))
	c.Set("audit_summary", "删除角色")
	OK(c, nil)
}

// GetPermissions returns the current permission assignment for a role.
func (h *RoleHandler) GetPermissions(c *gin.Context) {
	id, err := parseIDParam(c)
	if err != nil {
		return
	}

	role, err := h.roleSvc.GetByID(id)
	if err != nil {
		if errors.Is(err, service.ErrRoleNotFound) {
			Fail(c, http.StatusNotFound, "role not found")
			return
		}
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	policies := h.casbinSvc.GetPoliciesForRole(role.Code)

	menuPerms := make([]string, 0)
	apiPolicies := make([]gin.H, 0)
	for _, p := range policies {
		obj, act := p[1], p[2]
		if len(obj) > 0 && obj[0] == '/' {
			apiPolicies = append(apiPolicies, gin.H{"path": obj, "method": act})
		} else {
			menuPerms = append(menuPerms, obj)
		}
	}

	OK(c, gin.H{
		"menuPermissions": menuPerms,
		"apiPolicies":     apiPolicies,
	})
}

type setPermissionsReq struct {
	MenuIDs     []uint `json:"menuIds"`
	APIPolicies []struct {
		Path   string `json:"path"`
		Method string `json:"method"`
	} `json:"apiPolicies"`
}

// SetPermissions replaces all permissions for a role.
func (h *RoleHandler) SetPermissions(c *gin.Context) {
	id, err := parseIDParam(c)
	if err != nil {
		return
	}

	role, err := h.roleSvc.GetByID(id)
	if err != nil {
		if errors.Is(err, service.ErrRoleNotFound) {
			Fail(c, http.StatusNotFound, "role not found")
			return
		}
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	if role.IsSystem && role.Code == "admin" {
		Fail(c, http.StatusBadRequest, "cannot modify system admin permissions")
		return
	}

	var req setPermissionsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	// Build Casbin policies
	var policies [][]string

	// Menu permissions: get the permission string from menu IDs
	if len(req.MenuIDs) > 0 {
		tree, _ := h.menuSvc.GetTree()
		menuMap := flattenMenus(tree)
		for _, mid := range req.MenuIDs {
			if m, ok := menuMap[mid]; ok && m.Permission != "" {
				policies = append(policies, []string{role.Code, m.Permission, "read"})
			}
		}
	}

	// API policies
	for _, ap := range req.APIPolicies {
		policies = append(policies, []string{role.Code, ap.Path, ap.Method})
	}

	if err := h.casbinSvc.SetPoliciesForRole(role.Code, policies); err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_action", "role.set_permissions")
	c.Set("audit_resource", "role")
	c.Set("audit_resource_id", strconv.FormatUint(uint64(id), 10))
	c.Set("audit_summary", "设置角色权限")
	OK(c, nil)
}

func flattenMenus(menus []model.Menu) map[uint]model.Menu {
	result := make(map[uint]model.Menu)
	var walk func([]model.Menu)
	walk = func(items []model.Menu) {
		for _, m := range items {
			result[m.ID] = m
			walk(m.Children)
		}
	}
	walk(menus)
	return result
}

type updateDataScopeReq struct {
	DataScope model.DataScope `json:"dataScope" binding:"required"`
	DeptIDs   []uint          `json:"deptIds"`
}

// UpdateDataScope sets the data visibility scope for a role.
func (h *RoleHandler) UpdateDataScope(c *gin.Context) {
	id, err := parseIDParam(c)
	if err != nil {
		return
	}

	var req updateDataScopeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	role, err := h.roleSvc.UpdateDataScope(id, req.DataScope, req.DeptIDs)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrRoleNotFound):
			Fail(c, http.StatusNotFound, "role not found")
		case errors.Is(err, service.ErrSystemRole):
			Fail(c, http.StatusBadRequest, "cannot modify system admin data scope")
		case errors.Is(err, service.ErrDataScopeInvalid):
			Fail(c, http.StatusBadRequest, "invalid data scope value")
		default:
			Fail(c, http.StatusInternalServerError, err.Error())
		}
		return
	}

	c.Set("audit_action", "role.update_data_scope")
	c.Set("audit_resource", "role")
	c.Set("audit_resource_id", strconv.FormatUint(uint64(id), 10))
	c.Set("audit_summary", "更新角色数据权限范围")
	OK(c, role)
}

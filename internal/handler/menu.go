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

type MenuHandler struct {
	menuSvc *service.MenuService
}

func (h *MenuHandler) GetTree(c *gin.Context) {
	tree, err := h.menuSvc.GetTree()
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, tree)
}

func (h *MenuHandler) GetUserTree(c *gin.Context) {
	roleCode, _ := c.Get("userRole")
	code, _ := roleCode.(string)

	tree, err := h.menuSvc.GetUserTree(code)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	permissions := h.menuSvc.GetUserPermissions(code)

	OK(c, gin.H{
		"menus":       tree,
		"permissions": permissions,
	})
}

type createMenuReq struct {
	ParentID   *uint  `json:"parentId"`
	Name       string `json:"name" binding:"required"`
	Type       string `json:"type" binding:"required"`
	Path       string `json:"path"`
	Icon       string `json:"icon"`
	Permission string `json:"permission"`
	Sort       int    `json:"sort"`
	IsHidden   bool   `json:"isHidden"`
}

func (h *MenuHandler) Create(c *gin.Context) {
	var req createMenuReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	menu := &model.Menu{
		ParentID:   req.ParentID,
		Name:       req.Name,
		Type:       model.MenuType(req.Type),
		Path:       req.Path,
		Icon:       req.Icon,
		Permission: req.Permission,
		Sort:       req.Sort,
		IsHidden:   req.IsHidden,
	}

	if err := h.menuSvc.Create(menu); err != nil {
		switch {
		case errors.Is(err, service.ErrMenuInvalidType), errors.Is(err, service.ErrMenuPermissionExists), errors.Is(err, service.ErrMenuParentNotFound), errors.Is(err, service.ErrMenuParentNotAllowed), errors.Is(err, service.ErrMenuCircularParent):
			Fail(c, http.StatusBadRequest, err.Error())
		default:
			Fail(c, http.StatusInternalServerError, err.Error())
		}
		return
	}

	c.Set("audit_action", "menu.create")
	c.Set("audit_resource", "menu")
	c.Set("audit_summary", "创建菜单")
	OK(c, menu)
}

func (h *MenuHandler) Update(c *gin.Context) {
	id, err := parseIDParam(c)
	if err != nil {
		return
	}

	var updates map[string]any
	if err := c.ShouldBindJSON(&updates); err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	menu, err := h.menuSvc.Update(id, updates)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrMenuNotFound):
			Fail(c, http.StatusNotFound, "menu not found")
			return
		case errors.Is(err, service.ErrMenuInvalidType), errors.Is(err, service.ErrMenuPermissionExists), errors.Is(err, service.ErrMenuParentNotFound), errors.Is(err, service.ErrMenuParentNotAllowed), errors.Is(err, service.ErrMenuCircularParent):
			Fail(c, http.StatusBadRequest, err.Error())
			return
		}
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_action", "menu.update")
	c.Set("audit_resource", "menu")
	c.Set("audit_resource_id", strconv.FormatUint(uint64(id), 10))
	c.Set("audit_summary", "更新菜单")
	OK(c, menu)
}

type reorderReq struct {
	Items []repository.SortItem `json:"items" binding:"required,dive"`
}

func (h *MenuHandler) Reorder(c *gin.Context) {
	var req reorderReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if len(req.Items) == 0 {
		Fail(c, http.StatusBadRequest, "items is required")
		return
	}
	if err := h.menuSvc.ReorderMenus(req.Items); err != nil {
		if errors.Is(err, service.ErrMenuNotFound) {
			Fail(c, http.StatusBadRequest, err.Error())
			return
		}
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.Set("audit_action", "menu.reorder")
	c.Set("audit_resource", "menu")
	c.Set("audit_summary", "调整菜单排序")
	OK(c, nil)
}

func (h *MenuHandler) Delete(c *gin.Context) {
	id, err := parseIDParam(c)
	if err != nil {
		return
	}

	if err := h.menuSvc.Delete(id); err != nil {
		switch {
		case errors.Is(err, service.ErrMenuNotFound):
			Fail(c, http.StatusNotFound, "menu not found")
		case errors.Is(err, service.ErrMenuHasChildren):
			Fail(c, http.StatusBadRequest, "cannot delete menu with children, delete children first")
		default:
			Fail(c, http.StatusInternalServerError, err.Error())
		}
		return
	}

	c.Set("audit_action", "menu.delete")
	c.Set("audit_resource", "menu")
	c.Set("audit_resource_id", strconv.FormatUint(uint64(id), 10))
	c.Set("audit_summary", "删除菜单")
	OK(c, nil)
}

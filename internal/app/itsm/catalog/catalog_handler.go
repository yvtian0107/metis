package catalog

import (
	"errors"
	. "metis/internal/app/itsm/domain"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/samber/do/v2"

	"metis/internal/handler"
)

type CatalogHandler struct {
	svc *CatalogService
}

func NewCatalogHandler(i do.Injector) (*CatalogHandler, error) {
	svc := do.MustInvoke[*CatalogService](i)
	return &CatalogHandler{svc: svc}, nil
}

type CreateCatalogRequest struct {
	Name        string `json:"name" binding:"required,max=128"`
	Code        string `json:"code" binding:"required,max=64"`
	Description string `json:"description" binding:"max=512"`
	Icon        string `json:"icon" binding:"max=64"`
	ParentID    *uint  `json:"parentId"`
	SortOrder   int    `json:"sortOrder"`
}

func (h *CatalogHandler) Create(c *gin.Context) {
	var req CreateCatalogRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	c.Set("audit_action", "itsm.catalog.create")
	c.Set("audit_resource", "service_catalog")

	catalog, err := h.svc.Create(req.Name, req.Code, req.Description, req.Icon, req.ParentID, req.SortOrder)
	if err != nil {
		if errors.Is(err, ErrCatalogCodeExists) {
			handler.Fail(c, http.StatusConflict, err.Error())
			return
		}
		if errors.Is(err, ErrCatalogNotFound) {
			handler.Fail(c, http.StatusBadRequest, "parent catalog not found")
			return
		}
		if errors.Is(err, ErrCatalogTooDeep) || errors.Is(err, ErrCatalogInvalidParent) {
			handler.Fail(c, http.StatusBadRequest, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_summary", "created service catalog: "+catalog.Name)
	handler.OK(c, catalog.ToResponse())
}

func (h *CatalogHandler) Tree(c *gin.Context) {
	tree, err := h.svc.Tree()
	if err != nil {
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	handler.OK(c, tree)
}

func (h *CatalogHandler) ServiceCounts(c *gin.Context) {
	counts, err := h.svc.ServiceCounts()
	if err != nil {
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	handler.OK(c, counts)
}

type UpdateCatalogRequest struct {
	Name        *string `json:"name" binding:"omitempty,max=128"`
	Code        *string `json:"code" binding:"omitempty,max=64"`
	Description *string `json:"description" binding:"omitempty,max=512"`
	Icon        *string `json:"icon" binding:"omitempty,max=64"`
	ParentID    *uint   `json:"parentId"`
	SortOrder   *int    `json:"sortOrder"`
	IsActive    *bool   `json:"isActive"`
}

func (h *CatalogHandler) Update(c *gin.Context) {
	id, err := ParseID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid id")
		return
	}

	var req UpdateCatalogRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	c.Set("audit_action", "itsm.catalog.update")
	c.Set("audit_resource", "service_catalog")
	c.Set("audit_resource_id", c.Param("id"))

	updates := map[string]any{}
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Code != nil {
		updates["code"] = *req.Code
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.Icon != nil {
		updates["icon"] = *req.Icon
	}
	if req.ParentID != nil {
		updates["parent_id"] = *req.ParentID
	}
	if req.SortOrder != nil {
		updates["sort_order"] = *req.SortOrder
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}

	catalog, err := h.svc.Update(id, updates)
	if err != nil {
		if errors.Is(err, ErrCatalogCodeExists) {
			handler.Fail(c, http.StatusConflict, err.Error())
			return
		}
		if errors.Is(err, ErrCatalogNotFound) {
			handler.Fail(c, http.StatusNotFound, err.Error())
			return
		}
		if errors.Is(err, ErrCatalogTooDeep) || errors.Is(err, ErrCatalogInvalidParent) {
			handler.Fail(c, http.StatusBadRequest, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_summary", "updated service catalog: "+catalog.Name)
	handler.OK(c, catalog.ToResponse())
}

func (h *CatalogHandler) Delete(c *gin.Context) {
	id, err := ParseID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid id")
		return
	}

	c.Set("audit_action", "itsm.catalog.delete")
	c.Set("audit_resource", "service_catalog")
	c.Set("audit_resource_id", c.Param("id"))

	if err := h.svc.Delete(id); err != nil {
		switch {
		case errors.Is(err, ErrCatalogNotFound):
			handler.Fail(c, http.StatusNotFound, err.Error())
		case errors.Is(err, ErrCatalogHasChildren), errors.Is(err, ErrCatalogHasServices):
			handler.Fail(c, http.StatusBadRequest, err.Error())
		default:
			handler.Fail(c, http.StatusInternalServerError, err.Error())
		}
		return
	}

	c.Set("audit_summary", "deleted service catalog")
	handler.OK(c, nil)
}

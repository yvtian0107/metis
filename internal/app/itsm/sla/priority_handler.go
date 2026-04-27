package sla

import (
	"errors"
	. "metis/internal/app/itsm/domain"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/samber/do/v2"

	"metis/internal/handler"
)

type PriorityHandler struct {
	svc *PriorityService
}

func NewPriorityHandler(i do.Injector) (*PriorityHandler, error) {
	svc := do.MustInvoke[*PriorityService](i)
	return &PriorityHandler{svc: svc}, nil
}

type CreatePriorityRequest struct {
	Name        string `json:"name" binding:"required,max=64"`
	Code        string `json:"code" binding:"required,max=16"`
	Value       int    `json:"value" binding:"required"`
	Color       string `json:"color" binding:"required,max=16"`
	Description string `json:"description" binding:"max=255"`
}

func (h *PriorityHandler) Create(c *gin.Context) {
	var req CreatePriorityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	c.Set("audit_action", "itsm.priority.create")
	c.Set("audit_resource", "priority")

	p := &Priority{
		Name:        req.Name,
		Code:        req.Code,
		Value:       req.Value,
		Color:       req.Color,
		Description: req.Description,
	}

	result, err := h.svc.Create(p)
	if err != nil {
		if errors.Is(err, ErrPriorityCodeExists) {
			handler.Fail(c, http.StatusConflict, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_summary", "created priority: "+result.Name)
	handler.OK(c, result.ToResponse())
}

func (h *PriorityHandler) List(c *gin.Context) {
	items, err := h.svc.ListAll()
	if err != nil {
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	result := make([]PriorityResponse, len(items))
	for i, p := range items {
		result[i] = p.ToResponse()
	}
	handler.OK(c, result)
}

type UpdatePriorityRequest struct {
	Name        *string `json:"name" binding:"omitempty,max=64"`
	Code        *string `json:"code" binding:"omitempty,max=16"`
	Value       *int    `json:"value"`
	Color       *string `json:"color" binding:"omitempty,max=16"`
	Description *string `json:"description" binding:"omitempty,max=255"`
	IsActive    *bool   `json:"isActive"`
}

func (h *PriorityHandler) Update(c *gin.Context) {
	id, err := ParseID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid id")
		return
	}

	var req UpdatePriorityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	c.Set("audit_action", "itsm.priority.update")
	c.Set("audit_resource", "priority")
	c.Set("audit_resource_id", c.Param("id"))

	updates := map[string]any{}
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Code != nil {
		updates["code"] = *req.Code
	}
	if req.Value != nil {
		updates["value"] = *req.Value
	}
	if req.Color != nil {
		updates["color"] = *req.Color
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}

	result, err := h.svc.Update(id, updates)
	if err != nil {
		switch {
		case errors.Is(err, ErrPriorityNotFound):
			handler.Fail(c, http.StatusNotFound, err.Error())
		case errors.Is(err, ErrPriorityCodeExists):
			handler.Fail(c, http.StatusConflict, err.Error())
		default:
			handler.Fail(c, http.StatusInternalServerError, err.Error())
		}
		return
	}

	c.Set("audit_summary", "updated priority: "+result.Name)
	handler.OK(c, result.ToResponse())
}

func (h *PriorityHandler) Delete(c *gin.Context) {
	id, err := ParseID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid id")
		return
	}

	c.Set("audit_action", "itsm.priority.delete")
	c.Set("audit_resource", "priority")
	c.Set("audit_resource_id", c.Param("id"))

	if err := h.svc.Delete(id); err != nil {
		if errors.Is(err, ErrPriorityNotFound) {
			handler.Fail(c, http.StatusNotFound, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_summary", "deleted priority")
	handler.OK(c, nil)
}

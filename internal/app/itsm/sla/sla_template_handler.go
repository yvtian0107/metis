package sla

import (
	"errors"
	. "metis/internal/app/itsm/domain"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/samber/do/v2"

	"metis/internal/handler"
)

type SLATemplateHandler struct {
	svc *SLATemplateService
}

func NewSLATemplateHandler(i do.Injector) (*SLATemplateHandler, error) {
	svc := do.MustInvoke[*SLATemplateService](i)
	return &SLATemplateHandler{svc: svc}, nil
}

type CreateSLARequest struct {
	Name              string `json:"name" binding:"required,max=128"`
	Code              string `json:"code" binding:"required,max=64"`
	Description       string `json:"description" binding:"max=512"`
	ResponseMinutes   int    `json:"responseMinutes" binding:"required"`
	ResolutionMinutes int    `json:"resolutionMinutes" binding:"required"`
}

func (h *SLATemplateHandler) Create(c *gin.Context) {
	var req CreateSLARequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	c.Set("audit_action", "itsm.sla.create")
	c.Set("audit_resource", "sla_template")

	sla := &SLATemplate{
		Name:              req.Name,
		Code:              req.Code,
		Description:       req.Description,
		ResponseMinutes:   req.ResponseMinutes,
		ResolutionMinutes: req.ResolutionMinutes,
	}

	result, err := h.svc.Create(sla)
	if err != nil {
		if errors.Is(err, ErrSLACodeExists) {
			handler.Fail(c, http.StatusConflict, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_summary", "created SLA template: "+result.Name)
	handler.OK(c, result.ToResponse())
}

func (h *SLATemplateHandler) List(c *gin.Context) {
	items, err := h.svc.ListAll()
	if err != nil {
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	result := make([]SLATemplateResponse, len(items))
	for i, s := range items {
		result[i] = s.ToResponse()
	}
	handler.OK(c, result)
}

type UpdateSLARequest struct {
	Name              *string `json:"name" binding:"omitempty,max=128"`
	Code              *string `json:"code" binding:"omitempty,max=64"`
	Description       *string `json:"description" binding:"omitempty,max=512"`
	ResponseMinutes   *int    `json:"responseMinutes"`
	ResolutionMinutes *int    `json:"resolutionMinutes"`
	IsActive          *bool   `json:"isActive"`
}

func (h *SLATemplateHandler) Update(c *gin.Context) {
	id, err := ParseID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid id")
		return
	}

	var req UpdateSLARequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	c.Set("audit_action", "itsm.sla.update")
	c.Set("audit_resource", "sla_template")
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
	if req.ResponseMinutes != nil {
		updates["response_minutes"] = *req.ResponseMinutes
	}
	if req.ResolutionMinutes != nil {
		updates["resolution_minutes"] = *req.ResolutionMinutes
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}

	result, err := h.svc.Update(id, updates)
	if err != nil {
		switch {
		case errors.Is(err, ErrSLATemplateNotFound):
			handler.Fail(c, http.StatusNotFound, err.Error())
		case errors.Is(err, ErrSLACodeExists):
			handler.Fail(c, http.StatusConflict, err.Error())
		default:
			handler.Fail(c, http.StatusInternalServerError, err.Error())
		}
		return
	}

	c.Set("audit_summary", "updated SLA template: "+result.Name)
	handler.OK(c, result.ToResponse())
}

func (h *SLATemplateHandler) Delete(c *gin.Context) {
	id, err := ParseID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid id")
		return
	}

	c.Set("audit_action", "itsm.sla.delete")
	c.Set("audit_resource", "sla_template")
	c.Set("audit_resource_id", c.Param("id"))

	if err := h.svc.Delete(id); err != nil {
		if errors.Is(err, ErrSLATemplateNotFound) {
			handler.Fail(c, http.StatusNotFound, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_summary", "deleted SLA template")
	handler.OK(c, nil)
}

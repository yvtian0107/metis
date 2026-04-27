package definition

import (
	"errors"
	. "metis/internal/app/itsm/catalog"
	. "metis/internal/app/itsm/domain"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/samber/do/v2"

	"metis/internal/handler"
)

type ServiceDefHandler struct {
	svc *ServiceDefService
}

func NewServiceDefHandler(i do.Injector) (*ServiceDefHandler, error) {
	svc := do.MustInvoke[*ServiceDefService](i)
	return &ServiceDefHandler{svc: svc}, nil
}

type CreateServiceDefRequest struct {
	Name              string    `json:"name" binding:"required,max=128"`
	Code              string    `json:"code" binding:"required,max=64"`
	Description       string    `json:"description" binding:"max=1024"`
	CatalogID         uint      `json:"catalogId" binding:"required"`
	EngineType        string    `json:"engineType" binding:"required,oneof=classic smart"`
	SLAID             *uint     `json:"slaId"`
	IntakeFormSchema  JSONField `json:"intakeFormSchema"`
	WorkflowJSON      JSONField `json:"workflowJson"`
	CollaborationSpec string    `json:"collaborationSpec"`
	AgentID           *uint     `json:"agentId"`
	AgentConfig       JSONField `json:"agentConfig"`
	KnowledgeBaseIDs  JSONField `json:"knowledgeBaseIds"`
	SortOrder         int       `json:"sortOrder"`
}

func (h *ServiceDefHandler) Create(c *gin.Context) {
	var req CreateServiceDefRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	c.Set("audit_action", "itsm.service.create")
	c.Set("audit_resource", "service_definition")

	svc := &ServiceDefinition{
		Name:              req.Name,
		Code:              req.Code,
		Description:       req.Description,
		CatalogID:         req.CatalogID,
		EngineType:        req.EngineType,
		SLAID:             req.SLAID,
		IntakeFormSchema:  req.IntakeFormSchema,
		WorkflowJSON:      req.WorkflowJSON,
		CollaborationSpec: req.CollaborationSpec,
		AgentID:           req.AgentID,
		AgentConfig:       req.AgentConfig,
		KnowledgeBaseIDs:  req.KnowledgeBaseIDs,
		SortOrder:         req.SortOrder,
	}

	result, err := h.svc.Create(svc)
	if err != nil {
		switch {
		case errors.Is(err, ErrServiceCodeExists):
			handler.Fail(c, http.StatusConflict, err.Error())
		case errors.Is(err, ErrCatalogNotFound), errors.Is(err, ErrServiceEngineMismatch), errors.Is(err, ErrAgentNotAvailable):
			handler.Fail(c, http.StatusBadRequest, err.Error())
		case errors.Is(err, ErrWorkflowValidation):
			handler.Fail(c, http.StatusBadRequest, err.Error())
		default:
			handler.Fail(c, http.StatusInternalServerError, err.Error())
		}
		return
	}

	c.Set("audit_resource_id", strconv.Itoa(int(result.ID)))
	c.Set("audit_summary", "created service: "+result.Name)
	handler.OK(c, result.ToResponse())
}

func (h *ServiceDefHandler) List(c *gin.Context) {
	var catalogID *uint
	if cidStr := c.Query("catalogId"); cidStr != "" {
		cid, err := strconv.ParseUint(cidStr, 10, 64)
		if err == nil {
			v := uint(cid)
			catalogID = &v
		}
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))

	var engineType *string
	if et := c.Query("engineType"); et != "" {
		engineType = &et
	}

	var isActive *bool
	if v := c.Query("isActive"); v != "" {
		b := v == "true"
		isActive = &b
	}

	items, total, err := h.svc.List(ServiceDefListParams{
		CatalogID:  catalogID,
		EngineType: engineType,
		Keyword:    c.Query("keyword"),
		IsActive:   isActive,
		Page:       page,
		PageSize:   pageSize,
	})
	if err != nil {
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	result := make([]ServiceDefinitionResponse, len(items))
	for i, s := range items {
		result[i] = s.ToResponse()
	}
	handler.OK(c, gin.H{"items": result, "total": total})
}

func (h *ServiceDefHandler) Get(c *gin.Context) {
	id, err := ParseID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid id")
		return
	}

	svc, err := h.svc.Get(id)
	if err != nil {
		if errors.Is(err, ErrServiceDefNotFound) {
			handler.Fail(c, http.StatusNotFound, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	handler.OK(c, svc.ToResponse())
}

func (h *ServiceDefHandler) HealthCheck(c *gin.Context) {
	id, err := ParseID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid id")
		return
	}

	result, err := h.svc.HealthCheck(id)
	if err != nil {
		if errors.Is(err, ErrServiceDefNotFound) {
			handler.Fail(c, http.StatusNotFound, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	handler.OK(c, result)
}

type UpdateServiceDefRequest struct {
	Name              *string    `json:"name" binding:"omitempty,max=128"`
	Code              *string    `json:"code" binding:"omitempty,max=64"`
	Description       *string    `json:"description" binding:"omitempty,max=1024"`
	CatalogID         *uint      `json:"catalogId"`
	EngineType        *string    `json:"engineType" binding:"omitempty,oneof=classic smart"`
	SLAID             *uint      `json:"slaId"`
	IntakeFormSchema  *JSONField `json:"intakeFormSchema"`
	WorkflowJSON      *JSONField `json:"workflowJson"`
	CollaborationSpec *string    `json:"collaborationSpec"`
	AgentID           *uint      `json:"agentId"`
	AgentConfig       *JSONField `json:"agentConfig"`
	KnowledgeBaseIDs  *JSONField `json:"knowledgeBaseIds"`
	IsActive          *bool      `json:"isActive"`
	SortOrder         *int       `json:"sortOrder"`
}

func (h *ServiceDefHandler) Update(c *gin.Context) {
	id, err := ParseID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid id")
		return
	}

	var req UpdateServiceDefRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	c.Set("audit_action", "itsm.service.update")
	c.Set("audit_resource", "service_definition")
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
	if req.CatalogID != nil {
		updates["catalog_id"] = *req.CatalogID
	}
	if req.EngineType != nil {
		updates["engine_type"] = *req.EngineType
	}
	if req.SLAID != nil {
		updates["sla_id"] = *req.SLAID
	}
	if req.IntakeFormSchema != nil {
		updates["intake_form_schema"] = *req.IntakeFormSchema
	}
	if req.WorkflowJSON != nil {
		updates["workflow_json"] = *req.WorkflowJSON
	}
	if req.CollaborationSpec != nil {
		updates["collaboration_spec"] = *req.CollaborationSpec
	}
	if req.AgentID != nil {
		updates["agent_id"] = *req.AgentID
	}
	if req.AgentConfig != nil {
		updates["agent_config"] = *req.AgentConfig
	}
	if req.KnowledgeBaseIDs != nil {
		updates["knowledge_base_ids"] = *req.KnowledgeBaseIDs
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}
	if req.SortOrder != nil {
		updates["sort_order"] = *req.SortOrder
	}

	result, err := h.svc.Update(id, updates)
	if err != nil {
		switch {
		case errors.Is(err, ErrServiceDefNotFound):
			handler.Fail(c, http.StatusNotFound, err.Error())
		case errors.Is(err, ErrServiceCodeExists):
			handler.Fail(c, http.StatusConflict, err.Error())
		case errors.Is(err, ErrCatalogNotFound), errors.Is(err, ErrServiceEngineMismatch), errors.Is(err, ErrAgentNotAvailable):
			handler.Fail(c, http.StatusBadRequest, err.Error())
		case errors.Is(err, ErrWorkflowValidation):
			handler.Fail(c, http.StatusBadRequest, err.Error())
		default:
			handler.Fail(c, http.StatusInternalServerError, err.Error())
		}
		return
	}

	c.Set("audit_summary", "updated service: "+result.Name)
	handler.OK(c, result.ToResponse())
}

func (h *ServiceDefHandler) Delete(c *gin.Context) {
	id, err := ParseID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid id")
		return
	}

	c.Set("audit_action", "itsm.service.delete")
	c.Set("audit_resource", "service_definition")
	c.Set("audit_resource_id", c.Param("id"))

	if err := h.svc.Delete(id); err != nil {
		if errors.Is(err, ErrServiceDefNotFound) {
			handler.Fail(c, http.StatusNotFound, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_summary", "deleted service definition")
	handler.OK(c, nil)
}

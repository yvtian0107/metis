package definition

import (
	"errors"
	. "metis/internal/app/itsm/domain"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/samber/do/v2"

	"metis/internal/handler"
)

type ServiceActionHandler struct {
	svc *ServiceActionService
}

func NewServiceActionHandler(i do.Injector) (*ServiceActionHandler, error) {
	svc := do.MustInvoke[*ServiceActionService](i)
	return &ServiceActionHandler{svc: svc}, nil
}

type CreateServiceActionRequest struct {
	Name        string    `json:"name" binding:"required,max=128"`
	Code        string    `json:"code" binding:"required,max=64"`
	Description string    `json:"description" binding:"max=512"`
	Prompt      string    `json:"prompt"`
	ActionType  string    `json:"actionType" binding:"required"`
	ConfigJSON  JSONField `json:"configJson"`
}

func (h *ServiceActionHandler) Create(c *gin.Context) {
	serviceID, err := ParseID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid service id")
		return
	}

	var req CreateServiceActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	c.Set("audit_action", "itsm.action.create")
	c.Set("audit_resource", "service_action")

	action := &ServiceAction{
		Name:        req.Name,
		Code:        req.Code,
		Description: req.Description,
		Prompt:      req.Prompt,
		ActionType:  req.ActionType,
		ConfigJSON:  req.ConfigJSON,
		ServiceID:   serviceID,
	}

	result, err := h.svc.Create(action)
	if err != nil {
		if errors.Is(err, ErrActionCodeExists) {
			handler.Fail(c, http.StatusConflict, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_summary", "created service action: "+result.Name)
	handler.OK(c, result.ToResponse())
}

func (h *ServiceActionHandler) List(c *gin.Context) {
	serviceID, err := ParseID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid service id")
		return
	}

	items, err := h.svc.ListByService(serviceID)
	if err != nil {
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	result := make([]ServiceActionResponse, len(items))
	for i, a := range items {
		result[i] = a.ToResponse()
	}
	handler.OK(c, result)
}

type UpdateServiceActionRequest struct {
	Name        *string    `json:"name" binding:"omitempty,max=128"`
	Code        *string    `json:"code" binding:"omitempty,max=64"`
	Description *string    `json:"description" binding:"omitempty,max=512"`
	Prompt      *string    `json:"prompt"`
	ActionType  *string    `json:"actionType"`
	ConfigJSON  *JSONField `json:"configJson"`
	IsActive    *bool      `json:"isActive"`
}

func (h *ServiceActionHandler) Update(c *gin.Context) {
	actionID, err := ParseParamID(c, "actionId")
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid action id")
		return
	}

	var req UpdateServiceActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	c.Set("audit_action", "itsm.action.update")
	c.Set("audit_resource", "service_action")

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
	if req.Prompt != nil {
		updates["prompt"] = *req.Prompt
	}
	if req.ActionType != nil {
		updates["action_type"] = *req.ActionType
	}
	if req.ConfigJSON != nil {
		updates["config_json"] = *req.ConfigJSON
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}

	result, err := h.svc.Update(actionID, updates)
	if err != nil {
		switch {
		case errors.Is(err, ErrServiceActionNotFound):
			handler.Fail(c, http.StatusNotFound, err.Error())
		case errors.Is(err, ErrActionCodeExists):
			handler.Fail(c, http.StatusConflict, err.Error())
		default:
			handler.Fail(c, http.StatusInternalServerError, err.Error())
		}
		return
	}

	c.Set("audit_summary", "updated service action: "+result.Name)
	handler.OK(c, result.ToResponse())
}

func (h *ServiceActionHandler) Delete(c *gin.Context) {
	actionID, err := ParseParamID(c, "actionId")
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid action id")
		return
	}

	c.Set("audit_action", "itsm.action.delete")
	c.Set("audit_resource", "service_action")

	if err := h.svc.Delete(actionID); err != nil {
		if errors.Is(err, ErrServiceActionNotFound) {
			handler.Fail(c, http.StatusNotFound, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_summary", "deleted service action")
	handler.OK(c, nil)
}

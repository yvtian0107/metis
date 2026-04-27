package sla

import (
	"errors"
	. "metis/internal/app/itsm/domain"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/samber/do/v2"

	"metis/internal/database"
	"metis/internal/handler"
	"metis/internal/model"
)

type EscalationRuleHandler struct {
	svc *EscalationRuleService
	db  *database.DB
}

func NewEscalationRuleHandler(i do.Injector) (*EscalationRuleHandler, error) {
	svc := do.MustInvoke[*EscalationRuleService](i)
	db := do.MustInvoke[*database.DB](i)
	return &EscalationRuleHandler{svc: svc, db: db}, nil
}

type CreateEscalationRuleRequest struct {
	TriggerType  string    `json:"triggerType" binding:"required,oneof=response_timeout resolution_timeout"`
	Level        int       `json:"level" binding:"required"`
	WaitMinutes  int       `json:"waitMinutes" binding:"required"`
	ActionType   string    `json:"actionType" binding:"required,oneof=notify reassign escalate_priority"`
	TargetConfig JSONField `json:"targetConfig"`
}

func (h *EscalationRuleHandler) Create(c *gin.Context) {
	slaID, err := ParseID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid SLA id")
		return
	}

	var req CreateEscalationRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	c.Set("audit_action", "itsm.escalation.create")
	c.Set("audit_resource", "escalation_rule")

	rule := &EscalationRule{
		SLAID:        slaID,
		TriggerType:  req.TriggerType,
		Level:        req.Level,
		WaitMinutes:  req.WaitMinutes,
		ActionType:   req.ActionType,
		TargetConfig: req.TargetConfig,
	}

	result, err := h.svc.Create(rule)
	if err != nil {
		if errors.Is(err, ErrEscalationLevelExists) {
			handler.Fail(c, http.StatusConflict, err.Error())
			return
		}
		if errors.Is(err, ErrEscalationTargetConfig) {
			handler.Fail(c, http.StatusBadRequest, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_summary", "created escalation rule")
	handler.OK(c, result.ToResponse())
}

func (h *EscalationRuleHandler) List(c *gin.Context) {
	slaID, err := ParseID(c)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid SLA id")
		return
	}

	items, err := h.svc.ListBySLA(slaID)
	if err != nil {
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	result := make([]EscalationRuleResponse, len(items))
	for i, e := range items {
		result[i] = e.ToResponse()
	}
	handler.OK(c, result)
}

type UpdateEscalationRuleRequest struct {
	TriggerType  *string    `json:"triggerType" binding:"omitempty,oneof=response_timeout resolution_timeout"`
	Level        *int       `json:"level"`
	WaitMinutes  *int       `json:"waitMinutes"`
	ActionType   *string    `json:"actionType" binding:"omitempty,oneof=notify reassign escalate_priority"`
	TargetConfig *JSONField `json:"targetConfig"`
	IsActive     *bool      `json:"isActive"`
}

func (h *EscalationRuleHandler) Update(c *gin.Context) {
	escalationID, err := ParseParamID(c, "escalationId")
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid escalation id")
		return
	}

	var req UpdateEscalationRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	c.Set("audit_action", "itsm.escalation.update")
	c.Set("audit_resource", "escalation_rule")

	updates := map[string]any{}
	if req.TriggerType != nil {
		updates["trigger_type"] = *req.TriggerType
	}
	if req.Level != nil {
		updates["level"] = *req.Level
	}
	if req.WaitMinutes != nil {
		updates["wait_minutes"] = *req.WaitMinutes
	}
	if req.ActionType != nil {
		updates["action_type"] = *req.ActionType
	}
	if req.TargetConfig != nil {
		updates["target_config"] = *req.TargetConfig
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}

	result, err := h.svc.Update(escalationID, updates)
	if err != nil {
		if errors.Is(err, ErrEscalationRuleNotFound) {
			handler.Fail(c, http.StatusNotFound, err.Error())
			return
		}
		if errors.Is(err, ErrEscalationLevelExists) {
			handler.Fail(c, http.StatusConflict, err.Error())
			return
		}
		if errors.Is(err, ErrEscalationTargetConfig) {
			handler.Fail(c, http.StatusBadRequest, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_summary", "updated escalation rule")
	handler.OK(c, result.ToResponse())
}

func (h *EscalationRuleHandler) Delete(c *gin.Context) {
	escalationID, err := ParseParamID(c, "escalationId")
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid escalation id")
		return
	}

	c.Set("audit_action", "itsm.escalation.delete")
	c.Set("audit_resource", "escalation_rule")

	if err := h.svc.Delete(escalationID); err != nil {
		if errors.Is(err, ErrEscalationRuleNotFound) {
			handler.Fail(c, http.StatusNotFound, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_summary", "deleted escalation rule")
	handler.OK(c, nil)
}

type NotificationChannelOption struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

func (h *EscalationRuleHandler) NotificationChannels(c *gin.Context) {
	var channels []model.MessageChannel
	if err := h.db.Where("enabled = ?", true).Order("created_at DESC").Find(&channels).Error; err != nil {
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	result := make([]NotificationChannelOption, len(channels))
	for i, channel := range channels {
		result[i] = NotificationChannelOption{
			ID:   channel.ID,
			Name: channel.Name,
			Type: channel.Type,
		}
	}
	handler.OK(c, result)
}

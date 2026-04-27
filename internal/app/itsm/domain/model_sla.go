package domain

import (
	"time"

	"metis/internal/model"
)

// Priority 工单优先级
type Priority struct {
	model.BaseModel
	Name        string `json:"name" gorm:"size:64;not null"`
	Code        string `json:"code" gorm:"size:16;uniqueIndex;not null"` // P0, P1, P2, P3, P4
	Value       int    `json:"value" gorm:"not null"`                    // lower = more urgent
	Color       string `json:"color" gorm:"size:16;not null"`            // hex color
	Description string `json:"description" gorm:"size:255"`
	IsActive    bool   `json:"isActive" gorm:"not null;default:true"`
}

func (Priority) TableName() string { return "itsm_priorities" }

type PriorityResponse struct {
	ID          uint      `json:"id"`
	Name        string    `json:"name"`
	Code        string    `json:"code"`
	Value       int       `json:"value"`
	Color       string    `json:"color"`
	Description string    `json:"description"`
	IsActive    bool      `json:"isActive"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

func (p *Priority) ToResponse() PriorityResponse {
	return PriorityResponse{
		ID:          p.ID,
		Name:        p.Name,
		Code:        p.Code,
		Value:       p.Value,
		Color:       p.Color,
		Description: p.Description,
		IsActive:    p.IsActive,
		CreatedAt:   p.CreatedAt,
		UpdatedAt:   p.UpdatedAt,
	}
}

// SLATemplate SLA 模板
type SLATemplate struct {
	model.BaseModel
	Name              string `json:"name" gorm:"size:128;not null"`
	Code              string `json:"code" gorm:"size:64;uniqueIndex;not null"`
	Description       string `json:"description" gorm:"size:512"`
	ResponseMinutes   int    `json:"responseMinutes" gorm:"not null"`
	ResolutionMinutes int    `json:"resolutionMinutes" gorm:"not null"`
	IsActive          bool   `json:"isActive" gorm:"not null;default:true"`
}

func (SLATemplate) TableName() string { return "itsm_sla_templates" }

type SLATemplateResponse struct {
	ID                uint      `json:"id"`
	Name              string    `json:"name"`
	Code              string    `json:"code"`
	Description       string    `json:"description"`
	ResponseMinutes   int       `json:"responseMinutes"`
	ResolutionMinutes int       `json:"resolutionMinutes"`
	IsActive          bool      `json:"isActive"`
	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt"`
}

func (s *SLATemplate) ToResponse() SLATemplateResponse {
	return SLATemplateResponse{
		ID:                s.ID,
		Name:              s.Name,
		Code:              s.Code,
		Description:       s.Description,
		ResponseMinutes:   s.ResponseMinutes,
		ResolutionMinutes: s.ResolutionMinutes,
		IsActive:          s.IsActive,
		CreatedAt:         s.CreatedAt,
		UpdatedAt:         s.UpdatedAt,
	}
}

// EscalationRule SLA 升级规则
type EscalationRule struct {
	model.BaseModel
	SLAID        uint      `json:"slaId" gorm:"not null;index:idx_esc_sla_trigger_level,unique"`
	TriggerType  string    `json:"triggerType" gorm:"size:32;not null;index:idx_esc_sla_trigger_level,unique"` // response_timeout | resolution_timeout
	Level        int       `json:"level" gorm:"not null;index:idx_esc_sla_trigger_level,unique"`               // 1, 2, 3
	WaitMinutes  int       `json:"waitMinutes" gorm:"not null"`
	ActionType   string    `json:"actionType" gorm:"size:32;not null"` // notify | reassign | escalate_priority
	TargetConfig JSONField `json:"targetConfig" gorm:"type:text"`
	IsActive     bool      `json:"isActive" gorm:"not null;default:true"`
}

func (EscalationRule) TableName() string { return "itsm_escalation_rules" }

type EscalationRuleResponse struct {
	ID           uint      `json:"id"`
	SLAID        uint      `json:"slaId"`
	TriggerType  string    `json:"triggerType"`
	Level        int       `json:"level"`
	WaitMinutes  int       `json:"waitMinutes"`
	ActionType   string    `json:"actionType"`
	TargetConfig JSONField `json:"targetConfig"`
	IsActive     bool      `json:"isActive"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

func (e *EscalationRule) ToResponse() EscalationRuleResponse {
	return EscalationRuleResponse{
		ID:           e.ID,
		SLAID:        e.SLAID,
		TriggerType:  e.TriggerType,
		Level:        e.Level,
		WaitMinutes:  e.WaitMinutes,
		ActionType:   e.ActionType,
		TargetConfig: e.TargetConfig,
		IsActive:     e.IsActive,
		CreatedAt:    e.CreatedAt,
		UpdatedAt:    e.UpdatedAt,
	}
}

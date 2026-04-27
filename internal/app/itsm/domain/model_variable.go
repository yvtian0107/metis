package domain

import (
	"encoding/json"
	"strconv"
	"time"

	"metis/internal/model"
)

// Value type constants for process variables.
const (
	ValueTypeString  = "string"
	ValueTypeNumber  = "number"
	ValueTypeBoolean = "boolean"
	ValueTypeJSON    = "json"
	ValueTypeDate    = "date"
)

// ProcessVariable stores a single key-value pair scoped to a ticket + scope.
type ProcessVariable struct {
	model.BaseModel
	TicketID  uint   `json:"ticketId" gorm:"not null;uniqueIndex:idx_ticket_scope_key"`
	ScopeID   string `json:"scopeId" gorm:"size:64;not null;uniqueIndex:idx_ticket_scope_key;default:root"`
	Key       string `json:"key" gorm:"size:128;not null;uniqueIndex:idx_ticket_scope_key"`
	Value     string `json:"value" gorm:"type:text"`
	ValueType string `json:"valueType" gorm:"size:16;not null;default:string"`
	Source    string `json:"source" gorm:"size:128"`
}

func (ProcessVariable) TableName() string { return "itsm_process_variables" }

// ProcessVariableResponse is the API response representation.
type ProcessVariableResponse struct {
	ID        uint      `json:"id"`
	TicketID  uint      `json:"ticketId"`
	ScopeID   string    `json:"scopeId"`
	Key       string    `json:"key"`
	Value     any       `json:"value"`
	ValueType string    `json:"valueType"`
	Source    string    `json:"source"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// ToResponse converts the model to an API response, deserializing value by value_type.
func (v *ProcessVariable) ToResponse() ProcessVariableResponse {
	return ProcessVariableResponse{
		ID:        v.ID,
		TicketID:  v.TicketID,
		ScopeID:   v.ScopeID,
		Key:       v.Key,
		Value:     deserializeValue(v.Value, v.ValueType),
		ValueType: v.ValueType,
		Source:    v.Source,
		CreatedAt: v.CreatedAt,
		UpdatedAt: v.UpdatedAt,
	}
}

// deserializeValue restores the typed value from the stored TEXT.
func deserializeValue(raw string, valueType string) any {
	if raw == "" {
		return nil
	}
	switch valueType {
	case ValueTypeNumber:
		if f, err := strconv.ParseFloat(raw, 64); err == nil {
			return f
		}
		return raw
	case ValueTypeBoolean:
		if b, err := strconv.ParseBool(raw); err == nil {
			return b
		}
		return raw
	case ValueTypeJSON:
		var v any
		if json.Unmarshal([]byte(raw), &v) == nil {
			return v
		}
		return raw
	default: // string, date
		return raw
	}
}

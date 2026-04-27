package runtime

import (
	"time"

	"metis/internal/model"
)

const (
	CapabilityTypeTool           = "tool"
	CapabilityTypeMCP            = "mcp"
	CapabilityTypeSkill          = "skill"
	CapabilityTypeKnowledgeBase  = "knowledge_base"
	CapabilityTypeKnowledgeGraph = "knowledge_graph"
)

type CapabilitySet struct {
	model.BaseModel
	Type        string `json:"type" gorm:"size:32;not null;index:idx_ai_capability_set_type_name,unique"`
	Name        string `json:"name" gorm:"size:128;not null;index:idx_ai_capability_set_type_name,unique"`
	Description string `json:"description" gorm:"type:text"`
	Icon        string `json:"icon" gorm:"size:64"`
	Sort        int    `json:"sort" gorm:"not null;default:0"`
	IsActive    bool   `json:"isActive" gorm:"not null;default:true"`
}

func (CapabilitySet) TableName() string { return "ai_capability_sets" }

type CapabilitySetItem struct {
	SetID  uint `json:"setId" gorm:"primaryKey;column:set_id"`
	ItemID uint `json:"itemId" gorm:"primaryKey;column:item_id"`
	Sort   int  `json:"sort" gorm:"not null;default:0"`
}

func (CapabilitySetItem) TableName() string { return "ai_capability_set_items" }

type AgentCapabilitySet struct {
	AgentID uint `json:"agentId" gorm:"primaryKey;column:agent_id"`
	SetID   uint `json:"setId" gorm:"primaryKey;column:set_id"`
}

func (AgentCapabilitySet) TableName() string { return "ai_agent_capability_sets" }

type AgentCapabilitySetItem struct {
	AgentID uint `json:"agentId" gorm:"primaryKey;column:agent_id"`
	SetID   uint `json:"setId" gorm:"primaryKey;column:set_id"`
	ItemID  uint `json:"itemId" gorm:"primaryKey;column:item_id"`
	Enabled bool `json:"enabled" gorm:"not null;default:true"`
}

func (AgentCapabilitySetItem) TableName() string { return "ai_agent_capability_set_items" }

type AgentCapabilitySetBinding struct {
	SetID   uint   `json:"setId"`
	ItemIDs []uint `json:"itemIds"`
}

type CapabilitySetItemResponse struct {
	ID                 uint   `json:"id"`
	Name               string `json:"name"`
	DisplayName        string `json:"displayName,omitempty"`
	Description        string `json:"description,omitempty"`
	IsActive           bool   `json:"isActive"`
	IsExecutable       bool   `json:"isExecutable,omitempty"`
	AvailabilityStatus string `json:"availabilityStatus,omitempty"`
	AvailabilityReason string `json:"availabilityReason,omitempty"`
}

type CapabilitySetResponse struct {
	ID          uint                        `json:"id"`
	Type        string                      `json:"type"`
	Name        string                      `json:"name"`
	Description string                      `json:"description"`
	Icon        string                      `json:"icon,omitempty"`
	Sort        int                         `json:"sort"`
	IsActive    bool                        `json:"isActive"`
	ItemCount   int                         `json:"itemCount"`
	Items       []CapabilitySetItemResponse `json:"items"`
	CreatedAt   time.Time                   `json:"createdAt"`
	UpdatedAt   time.Time                   `json:"updatedAt"`
}

func (s *CapabilitySet) ToResponse(items []CapabilitySetItemResponse) CapabilitySetResponse {
	return CapabilitySetResponse{
		ID:          s.ID,
		Type:        s.Type,
		Name:        s.Name,
		Description: s.Description,
		Icon:        s.Icon,
		Sort:        s.Sort,
		IsActive:    s.IsActive,
		ItemCount:   len(items),
		Items:       items,
		CreatedAt:   s.CreatedAt,
		UpdatedAt:   s.UpdatedAt,
	}
}

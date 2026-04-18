package itsm

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"metis/internal/model"
)

// JSONField is a JSON wrapper that handles SQLite TEXT columns for JSON objects.
type JSONField json.RawMessage

func (j JSONField) Value() (driver.Value, error) {
	if len(j) == 0 {
		return nil, nil
	}
	return string(j), nil
}

func (j *JSONField) Scan(src any) error {
	switch v := src.(type) {
	case string:
		*j = JSONField(v)
	case []byte:
		*j = append(JSONField(nil), v...)
	case nil:
		*j = nil
	default:
		return fmt.Errorf("JSONField.Scan: unsupported type %T", v)
	}
	return nil
}

func (j JSONField) MarshalJSON() ([]byte, error) {
	if len(j) == 0 {
		return []byte("null"), nil
	}
	return []byte(j), nil
}

func (j *JSONField) UnmarshalJSON(data []byte) error {
	*j = append(JSONField(nil), data...)
	return nil
}

// ServiceCatalog 服务目录（树形分类）
type ServiceCatalog struct {
	model.BaseModel
	Name        string `json:"name" gorm:"size:128;not null"`
	Code        string `json:"code" gorm:"size:64;uniqueIndex"`
	Description string `json:"description" gorm:"size:512"`
	Icon        string `json:"icon" gorm:"size:64"`
	ParentID    *uint  `json:"parentId" gorm:"index"`
	SortOrder   int    `json:"sortOrder" gorm:"default:0"`
	IsActive    bool   `json:"isActive" gorm:"not null;default:true"`
}

func (ServiceCatalog) TableName() string { return "itsm_service_catalogs" }

type ServiceCatalogResponse struct {
	ID          uint                     `json:"id"`
	Name        string                   `json:"name"`
	Code        string                   `json:"code"`
	Description string                   `json:"description"`
	Icon        string                   `json:"icon"`
	ParentID    *uint                    `json:"parentId"`
	SortOrder   int                      `json:"sortOrder"`
	IsActive    bool                     `json:"isActive"`
	Children    []ServiceCatalogResponse `json:"children,omitempty"`
	CreatedAt   time.Time                `json:"createdAt"`
	UpdatedAt   time.Time                `json:"updatedAt"`
}

func (c *ServiceCatalog) ToResponse() ServiceCatalogResponse {
	return ServiceCatalogResponse{
		ID:          c.ID,
		Name:        c.Name,
		Code:        c.Code,
		Description: c.Description,
		Icon:        c.Icon,
		ParentID:    c.ParentID,
		SortOrder:   c.SortOrder,
		IsActive:    c.IsActive,
		CreatedAt:   c.CreatedAt,
		UpdatedAt:   c.UpdatedAt,
	}
}

// ServiceDefinition 服务定义
type ServiceDefinition struct {
	model.BaseModel
	Name              string    `json:"name" gorm:"size:128;not null"`
	Code              string    `json:"code" gorm:"size:64;uniqueIndex;not null"`
	Description       string    `json:"description" gorm:"size:1024"`
	CatalogID         uint      `json:"catalogId" gorm:"not null;index"`
	EngineType        string    `json:"engineType" gorm:"size:16;not null;default:classic"` // classic | smart
	SLAID             *uint     `json:"slaId" gorm:"index"`
	IntakeFormSchema  JSONField `json:"intakeFormSchema" gorm:"type:text"`    // inline form schema
	WorkflowJSON      JSONField `json:"workflowJson" gorm:"type:text"`        // classic mode
	CollaborationSpec string    `json:"collaborationSpec" gorm:"type:text"`    // smart mode
	AgentID           *uint     `json:"agentId" gorm:"index"`                 // smart mode
	AgentConfig       JSONField `json:"agentConfig" gorm:"type:text"`         // smart mode
	KnowledgeBaseIDs  JSONField `json:"knowledgeBaseIds" gorm:"type:text"`    // smart mode: [1,2,3]
	IsActive          bool      `json:"isActive" gorm:"not null;default:true"`
	SortOrder         int       `json:"sortOrder" gorm:"default:0"`
}

func (ServiceDefinition) TableName() string { return "itsm_service_definitions" }

type ServiceDefinitionResponse struct {
	ID                uint      `json:"id"`
	Name              string    `json:"name"`
	Code              string    `json:"code"`
	Description       string    `json:"description"`
	CatalogID         uint      `json:"catalogId"`
	EngineType        string    `json:"engineType"`
	SLAID             *uint     `json:"slaId"`
	IntakeFormSchema  JSONField `json:"intakeFormSchema"`
	WorkflowJSON      JSONField `json:"workflowJson"`
	CollaborationSpec string    `json:"collaborationSpec"`
	AgentID           *uint     `json:"agentId"`
	AgentConfig       JSONField `json:"agentConfig"`
	KnowledgeBaseIDs  JSONField `json:"knowledgeBaseIds"`
	IsActive          bool      `json:"isActive"`
	SortOrder         int       `json:"sortOrder"`
	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt"`
}

func (s *ServiceDefinition) ToResponse() ServiceDefinitionResponse {
	return ServiceDefinitionResponse{
		ID:                s.ID,
		Name:              s.Name,
		Code:              s.Code,
		Description:       s.Description,
		CatalogID:         s.CatalogID,
		EngineType:        s.EngineType,
		SLAID:             s.SLAID,
		IntakeFormSchema:  s.IntakeFormSchema,
		WorkflowJSON:      s.WorkflowJSON,
		CollaborationSpec: s.CollaborationSpec,
		AgentID:           s.AgentID,
		AgentConfig:       s.AgentConfig,
		KnowledgeBaseIDs:  s.KnowledgeBaseIDs,
		IsActive:          s.IsActive,
		SortOrder:         s.SortOrder,
		CreatedAt:         s.CreatedAt,
		UpdatedAt:         s.UpdatedAt,
	}
}

// ServiceAction 服务动作（HTTP webhook 等）
type ServiceAction struct {
	model.BaseModel
	Name        string    `json:"name" gorm:"size:128;not null"`
	Code        string    `json:"code" gorm:"size:64;not null;index:idx_action_service_code,unique"`
	Description string    `json:"description" gorm:"size:512"`
	Prompt      string    `json:"prompt" gorm:"type:text"`
	ActionType  string    `json:"actionType" gorm:"size:16;not null;default:http"` // http
	ConfigJSON  JSONField `json:"configJson" gorm:"type:text"`
	ServiceID   uint      `json:"serviceId" gorm:"not null;index:idx_action_service_code,unique"`
	IsActive    bool      `json:"isActive" gorm:"not null;default:true"`
}

func (ServiceAction) TableName() string { return "itsm_service_actions" }

type ServiceActionResponse struct {
	ID          uint      `json:"id"`
	Name        string    `json:"name"`
	Code        string    `json:"code"`
	Description string    `json:"description"`
	Prompt      string    `json:"prompt"`
	ActionType  string    `json:"actionType"`
	ConfigJSON  JSONField `json:"configJson"`
	ServiceID   uint      `json:"serviceId"`
	IsActive    bool      `json:"isActive"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

func (a *ServiceAction) ToResponse() ServiceActionResponse {
	return ServiceActionResponse{
		ID:          a.ID,
		Name:        a.Name,
		Code:        a.Code,
		Description: a.Description,
		Prompt:      a.Prompt,
		ActionType:  a.ActionType,
		ConfigJSON:  a.ConfigJSON,
		ServiceID:   a.ServiceID,
		IsActive:    a.IsActive,
		CreatedAt:   a.CreatedAt,
		UpdatedAt:   a.UpdatedAt,
	}
}

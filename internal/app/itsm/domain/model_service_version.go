package domain

import "metis/internal/model"

// ServiceDefinitionVersion stores an immutable runtime snapshot of a service definition.
type ServiceDefinitionVersion struct {
	model.BaseModel
	ServiceID           uint      `json:"serviceId" gorm:"not null;index:idx_service_version_hash,unique;index:idx_service_version_number,unique"`
	Version             int       `json:"version" gorm:"not null;index:idx_service_version_number,unique"`
	ContentHash         string    `json:"contentHash" gorm:"size:64;not null;index:idx_service_version_hash,unique"`
	EngineType          string    `json:"engineType" gorm:"size:16;not null"`
	SLAID               *uint     `json:"slaId" gorm:"index"`
	IntakeFormSchema    JSONField `json:"intakeFormSchema" gorm:"type:text"`
	WorkflowJSON        JSONField `json:"workflowJson" gorm:"type:text"`
	CollaborationSpec   string    `json:"collaborationSpec" gorm:"type:text"`
	AgentID             *uint     `json:"agentId" gorm:"index"`
	AgentConfig         JSONField `json:"agentConfig" gorm:"type:text"`
	KnowledgeBaseIDs    JSONField `json:"knowledgeBaseIds" gorm:"type:text"`
	ActionsJSON         JSONField `json:"actionsJson" gorm:"type:text"`
	SLATemplateJSON     JSONField `json:"slaTemplateJson" gorm:"type:text"`
	EscalationRulesJSON JSONField `json:"escalationRulesJson" gorm:"type:text"`
}

func (ServiceDefinitionVersion) TableName() string { return "itsm_service_definition_versions" }

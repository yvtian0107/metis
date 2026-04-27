package domain

import (
	"metis/internal/model"
)

// ServiceKnowledgeDocument stores a knowledge document attached to a service definition.
// Documents are uploaded as files, then asynchronously parsed to extract plain text.
type ServiceKnowledgeDocument struct {
	model.BaseModel
	ServiceID   uint   `json:"serviceId" gorm:"not null;index"`
	FileName    string `json:"fileName" gorm:"size:256;not null"`
	FilePath    string `json:"filePath" gorm:"size:512;not null"`
	FileSize    int64  `json:"fileSize" gorm:"not null;default:0"`
	FileType    string `json:"fileType" gorm:"size:128"`
	ParseStatus string `json:"parseStatus" gorm:"size:16;not null;default:pending"` // pending | processing | completed | failed
	ParsedText  string `json:"parsedText,omitempty" gorm:"type:text"`
	ParseError  string `json:"parseError,omitempty" gorm:"size:1024"`
}

func (ServiceKnowledgeDocument) TableName() string { return "itsm_service_knowledge_documents" }

type ServiceKnowledgeDocumentResponse struct {
	ID          uint   `json:"id"`
	ServiceID   uint   `json:"serviceId"`
	FileName    string `json:"fileName"`
	FileSize    int64  `json:"fileSize"`
	FileType    string `json:"fileType"`
	ParseStatus string `json:"parseStatus"`
	ParseError  string `json:"parseError,omitempty"`
	CreatedAt   string `json:"createdAt"`
}

func (d *ServiceKnowledgeDocument) ToResponse() ServiceKnowledgeDocumentResponse {
	return ServiceKnowledgeDocumentResponse{
		ID:          d.ID,
		ServiceID:   d.ServiceID,
		FileName:    d.FileName,
		FileSize:    d.FileSize,
		FileType:    d.FileType,
		ParseStatus: d.ParseStatus,
		ParseError:  d.ParseError,
		CreatedAt:   d.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

package domain

import (
	"time"

	"metis/internal/model"
)

// ExecutionToken represents a single execution path within a ticket's workflow.
// Tokens form a tree: the root token (type=main) is created at workflow start;
// child tokens are created by parallel gateways (④) or subprocesses (⑤).
type ExecutionToken struct {
	model.BaseModel
	TicketID      uint   `json:"ticketId" gorm:"not null;index:idx_token_ticket_status"`
	ParentTokenID *uint  `json:"parentTokenId" gorm:"index"`
	NodeID        string `json:"nodeId" gorm:"size:64"`
	Status        string `json:"status" gorm:"size:16;not null;index:idx_token_ticket_status;default:active"`
	TokenType     string `json:"tokenType" gorm:"size:16;not null;default:main"`
	ScopeID       string `json:"scopeId" gorm:"size:64;not null;default:root"`
}

func (ExecutionToken) TableName() string { return "itsm_execution_tokens" }

// ExecutionTokenResponse is the API response representation.
type ExecutionTokenResponse struct {
	ID            uint      `json:"id"`
	TicketID      uint      `json:"ticketId"`
	ParentTokenID *uint     `json:"parentTokenId"`
	NodeID        string    `json:"nodeId"`
	Status        string    `json:"status"`
	TokenType     string    `json:"tokenType"`
	ScopeID       string    `json:"scopeId"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

// ToResponse converts the model to an API response.
func (t *ExecutionToken) ToResponse() ExecutionTokenResponse {
	return ExecutionTokenResponse{
		ID:            t.ID,
		TicketID:      t.TicketID,
		ParentTokenID: t.ParentTokenID,
		NodeID:        t.NodeID,
		Status:        t.Status,
		TokenType:     t.TokenType,
		ScopeID:       t.ScopeID,
		CreatedAt:     t.CreatedAt,
		UpdatedAt:     t.UpdatedAt,
	}
}

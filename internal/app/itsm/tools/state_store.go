package tools

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"gorm.io/gorm"
	"metis/internal/model"
)

// SessionStateStore implements StateStore by reading/writing the
// ai_agent_sessions.state JSON column.
type SessionStateStore struct {
	db *gorm.DB
}

// NewSessionStateStore creates a new SessionStateStore.
func NewSessionStateStore(db *gorm.DB) *SessionStateStore {
	return &SessionStateStore{db: db}
}

// GetState reads the session state from the database.
func (s *SessionStateStore) GetState(sessionID uint) (*ServiceDeskState, error) {
	var row struct {
		State string
	}
	if err := s.db.Table("ai_agent_sessions").
		Where("id = ?", sessionID).
		Select("state").First(&row).Error; err != nil {
		return nil, fmt.Errorf("session %d not found: %w", sessionID, err)
	}

	if row.State == "" || row.State == "null" {
		return defaultState(), nil
	}

	var state ServiceDeskState
	if err := json.Unmarshal([]byte(row.State), &state); err != nil {
		_ = s.auditInvalidState(sessionID, err)
		return nil, fmt.Errorf("invalid service desk state for session %d: %w", sessionID, err)
	}
	return &state, nil
}

func (s *SessionStateStore) auditInvalidState(sessionID uint, parseErr error) error {
	detail := parseErr.Error()
	return s.db.Create(&model.AuditLog{
		CreatedAt:  time.Now(),
		Category:   model.AuditCategoryApplication,
		Action:     "itsm.service_desk.state_invalid",
		Resource:   "ai_agent_session",
		ResourceID: strconv.FormatUint(uint64(sessionID), 10),
		Summary:    "Invalid ITSM service desk state JSON",
		Level:      model.AuditLevelError,
		Detail:     &detail,
	}).Error
}

// SaveState writes the session state to the database.
func (s *SessionStateStore) SaveState(sessionID uint, state *ServiceDeskState) error {
	b, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}
	if err := s.db.Table("ai_agent_sessions").
		Where("id = ?", sessionID).
		Update("state", string(b)).Error; err != nil {
		return fmt.Errorf("save state: %w", err)
	}
	return nil
}

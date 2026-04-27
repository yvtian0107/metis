package runtime

import (
	"github.com/samber/do/v2"

	"metis/internal/database"
)

type SessionRepo struct {
	db *database.DB
}

func NewSessionRepo(i do.Injector) (*SessionRepo, error) {
	return &SessionRepo{db: do.MustInvoke[*database.DB](i)}, nil
}

func (r *SessionRepo) Create(s *AgentSession) error {
	return r.db.Create(s).Error
}

func (r *SessionRepo) FindByID(id uint) (*AgentSession, error) {
	var s AgentSession
	if err := r.db.First(&s, id).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *SessionRepo) FindOwnedByID(id, userID uint) (*AgentSession, error) {
	var s AgentSession
	if err := r.db.Where("id = ? AND user_id = ?", id, userID).First(&s).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

type SessionListParams struct {
	AgentID  uint
	UserID   uint
	Page     int
	PageSize int
}

func (r *SessionRepo) List(params SessionListParams) ([]AgentSession, int64, error) {
	if params.Page < 1 {
		params.Page = 1
	}
	if params.PageSize < 1 {
		params.PageSize = 20
	}

	query := r.db.Model(&AgentSession{})
	if params.AgentID > 0 {
		query = query.Where("agent_id = ?", params.AgentID)
	}
	if params.UserID > 0 {
		query = query.Where("user_id = ?", params.UserID)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []AgentSession
	offset := (params.Page - 1) * params.PageSize
	if err := query.Offset(offset).Limit(params.PageSize).
		Order("updated_at DESC").
		Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *SessionRepo) UpdateStatus(id uint, status string) error {
	return r.db.Model(&AgentSession{}).Where("id = ?", id).Update("status", status).Error
}

func (r *SessionRepo) UpdateTitle(id uint, title string) error {
	return r.db.Model(&AgentSession{}).Where("id = ?", id).Update("title", title).Error
}

func (r *SessionRepo) Update(id uint, updates map[string]interface{}) error {
	return r.db.Model(&AgentSession{}).Where("id = ?", id).Updates(updates).Error
}

func (r *SessionRepo) Delete(id uint) error {
	// Delete messages first
	if err := r.db.Where("session_id = ?", id).Delete(&SessionMessage{}).Error; err != nil {
		return err
	}
	return r.db.Delete(&AgentSession{}, id).Error
}

// --- Messages ---

func (r *SessionRepo) CreateMessage(m *SessionMessage) error {
	return r.db.Create(m).Error
}

func (r *SessionRepo) GetMessages(sessionID uint) ([]SessionMessage, error) {
	var items []SessionMessage
	if err := r.db.Where("session_id = ?", sessionID).
		Order("sequence ASC").
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *SessionRepo) NextSequence(sessionID uint) (int, error) {
	var maxSeq *int
	if err := r.db.Model(&SessionMessage{}).
		Where("session_id = ?", sessionID).
		Select("MAX(sequence)").
		Scan(&maxSeq).Error; err != nil {
		return 1, err
	}
	if maxSeq == nil {
		return 1, nil
	}
	return *maxSeq + 1, nil
}

func (r *SessionRepo) FindMessageByID(id, sessionID uint) (*SessionMessage, error) {
	var m SessionMessage
	if err := r.db.Where("id = ? AND session_id = ?", id, sessionID).First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *SessionRepo) UpdateMessageContent(id uint, content string) error {
	return r.db.Model(&SessionMessage{}).Where("id = ?", id).Update("content", content).Error
}

func (r *SessionRepo) DeleteMessagesAfterSequence(sessionID uint, sequence int) error {
	return r.db.Where("session_id = ? AND sequence > ?", sessionID, sequence).Delete(&SessionMessage{}).Error
}

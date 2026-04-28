package runtime

import (
	"context"
	"errors"
	"log/slog"

	"github.com/samber/do/v2"
	"gorm.io/gorm"

	"metis/internal/app"
	"metis/internal/model"
)

var (
	ErrSessionNotFound = errors.New("session not found")
	ErrSessionBusy     = errors.New("session has a running execution")
)

type SessionService struct {
	repo           *SessionRepo
	agentSvc       *AgentService
	titleProviders []app.SessionTitleProvider
}

func NewSessionService(i do.Injector) (*SessionService, error) {
	return &SessionService{
		repo:           do.MustInvoke[*SessionRepo](i),
		agentSvc:       do.MustInvoke[*AgentService](i),
		titleProviders: collectSessionTitleProviders(),
	}, nil
}

func (s *SessionService) Create(agentID, userID uint) (*AgentSession, error) {
	// Validate current user can see the target agent.
	if _, err := s.agentSvc.GetAccessible(agentID, userID); err != nil {
		return nil, err
	}

	session := &AgentSession{
		AgentID: agentID,
		UserID:  userID,
		Status:  SessionStatusRunning,
	}
	if err := s.repo.Create(session); err != nil {
		return nil, err
	}
	return session, nil
}

func (s *SessionService) Get(id uint) (*AgentSession, error) {
	session, err := s.repo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSessionNotFound
		}
		return nil, err
	}
	return session, nil
}

func (s *SessionService) GetOwned(id, userID uint) (*AgentSession, error) {
	session, err := s.repo.FindOwnedByID(id, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSessionNotFound
		}
		return nil, err
	}
	return session, nil
}

func (s *SessionService) List(params SessionListParams) ([]AgentSession, int64, error) {
	return s.repo.List(params)
}

func (s *SessionService) Delete(id uint) error {
	return s.repo.Delete(id)
}

func (s *SessionService) GetMessages(sessionID uint) ([]SessionMessage, error) {
	return s.repo.GetMessages(sessionID)
}

func (s *SessionService) StoreMessage(sessionID uint, role, content string, metadata []byte, tokenCount int) (*SessionMessage, error) {
	return s.StoreMessageContext(context.Background(), sessionID, role, content, metadata, tokenCount)
}

func (s *SessionService) StoreMessageContext(ctx context.Context, sessionID uint, role, content string, metadata []byte, tokenCount int) (*SessionMessage, error) {
	seq, err := s.nextSequenceContext(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	msg := &SessionMessage{
		SessionID:  sessionID,
		Role:       role,
		Content:    content,
		Metadata:   model.JSONText(metadata),
		TokenCount: tokenCount,
		Sequence:   seq,
	}
	if err := s.repo.db.WithContext(ctx).Create(msg).Error; err != nil {
		return nil, err
	}

	// Auto-generate session title from first user message
	if seq == 1 && role == MessageRoleUser {
		title := s.fallbackTitle(content)
		session, findErr := s.repo.FindByID(sessionID)
		if findErr == nil {
			for _, provider := range s.titleProviders {
				providerTitle, handled, providerErr := provider.GenerateSessionTitle(ctx, sessionID, session.UserID, session.AgentID, content)
				if providerErr != nil {
					slog.Warn("session title provider failed", "sessionID", sessionID, "agentID", session.AgentID, "error", providerErr)
					if handled {
						break
					}
					continue
				}
				if handled {
					if providerTitle != "" {
						title = providerTitle
					}
					break
				}
			}
		}
		_ = s.repo.db.WithContext(ctx).Model(&AgentSession{}).Where("id = ?", sessionID).Update("title", title).Error
	}

	return msg, nil
}

func (s *SessionService) fallbackTitle(content string) string {
	title := content
	if len(title) > 100 {
		title = title[:100] + "..."
	}
	return title
}

func (s *SessionService) nextSequenceContext(ctx context.Context, sessionID uint) (int, error) {
	var maxSeq *int
	if err := s.repo.db.WithContext(ctx).Model(&SessionMessage{}).
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

func (s *SessionService) UpdateStatus(id uint, status string) error {
	return s.repo.UpdateStatus(id, status)
}

func (s *SessionService) Update(id uint, updates map[string]interface{}) error {
	return s.repo.Update(id, updates)
}

func (s *SessionService) EditMessage(sessionID, messageID uint, content string) (*SessionMessage, error) {
	msg, err := s.repo.FindMessageByID(messageID, sessionID)
	if err != nil {
		return nil, err
	}

	// Update message content
	if err := s.repo.UpdateMessageContent(messageID, content); err != nil {
		return nil, err
	}

	// Delete all messages after this one
	if err := s.repo.DeleteMessagesAfterSequence(sessionID, msg.Sequence); err != nil {
		return nil, err
	}

	msg.Content = content
	return msg, nil
}

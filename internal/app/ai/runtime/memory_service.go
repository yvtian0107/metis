package runtime

import (
	"errors"
	"fmt"
	"strings"

	"github.com/samber/do/v2"
	"gorm.io/gorm"
)

var (
	ErrMemoryNotFound = errors.New("memory not found")
)

const MaxMemoriesPerAgentUser = 100

type MemoryService struct {
	repo *MemoryRepo
}

func NewMemoryService(i do.Injector) (*MemoryService, error) {
	return &MemoryService{
		repo: do.MustInvoke[*MemoryRepo](i),
	}, nil
}

func (s *MemoryService) Upsert(m *AgentMemory) error {
	// Check limit for agent_generated entries
	if m.Source == MemorySourceAgentGenerated {
		total, err := s.repo.Count(m.AgentID, m.UserID)
		if err != nil {
			return err
		}
		if total >= MaxMemoriesPerAgentUser {
			// Evict oldest agent_generated entry
			agCount, err := s.repo.CountBySource(m.AgentID, m.UserID, MemorySourceAgentGenerated)
			if err != nil {
				return err
			}
			if agCount == 0 {
				// All entries are user_set, silently drop
				return nil
			}
			if err := s.repo.DeleteOldestBySource(m.AgentID, m.UserID, MemorySourceAgentGenerated); err != nil {
				return err
			}
		}
	}
	return s.repo.Upsert(m)
}

func (s *MemoryService) List(agentID, userID uint) ([]AgentMemory, error) {
	return s.repo.List(agentID, userID)
}

func (s *MemoryService) Delete(id uint) error {
	if _, err := s.repo.FindByID(id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrMemoryNotFound
		}
		return err
	}
	return s.repo.Delete(id)
}

func (s *MemoryService) DeleteForAgentUser(id, agentID, userID uint) error {
	if _, err := s.repo.FindByIDForAgentUser(id, agentID, userID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrMemoryNotFound
		}
		return err
	}
	return s.repo.Delete(id)
}

// FormatForPrompt formats all memories for injection into system prompt
func (s *MemoryService) FormatForPrompt(agentID, userID uint) (string, error) {
	memories, err := s.repo.List(agentID, userID)
	if err != nil {
		return "", err
	}
	if len(memories) == 0 {
		return "", nil
	}

	var sb strings.Builder
	sb.WriteString("## User Memories\n")
	sb.WriteString("The following are things you remember about this user:\n")
	for _, m := range memories {
		sb.WriteString(fmt.Sprintf("- %s: %s\n", m.Key, m.Content))
	}
	return sb.String(), nil
}

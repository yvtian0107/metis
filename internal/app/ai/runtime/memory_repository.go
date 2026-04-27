package runtime

import (
	"github.com/samber/do/v2"

	"metis/internal/database"
)

type MemoryRepo struct {
	db *database.DB
}

func NewMemoryRepo(i do.Injector) (*MemoryRepo, error) {
	return &MemoryRepo{db: do.MustInvoke[*database.DB](i)}, nil
}

// Upsert creates or updates a memory entry by agent+user+key.
func (r *MemoryRepo) Upsert(m *AgentMemory) error {
	var existing AgentMemory
	err := r.db.Where("agent_id = ? AND user_id = ? AND key = ?", m.AgentID, m.UserID, m.Key).First(&existing).Error
	if err == nil {
		// Update existing
		existing.Content = m.Content
		existing.Source = m.Source
		return r.db.Save(&existing).Error
	}
	return r.db.Create(m).Error
}

func (r *MemoryRepo) List(agentID, userID uint) ([]AgentMemory, error) {
	var items []AgentMemory
	if err := r.db.Where("agent_id = ? AND user_id = ?", agentID, userID).
		Order("updated_at DESC").
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *MemoryRepo) FindByID(id uint) (*AgentMemory, error) {
	var m AgentMemory
	if err := r.db.First(&m, id).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *MemoryRepo) FindByIDForAgentUser(id, agentID, userID uint) (*AgentMemory, error) {
	var m AgentMemory
	if err := r.db.Where("id = ? AND agent_id = ? AND user_id = ?", id, agentID, userID).First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *MemoryRepo) Delete(id uint) error {
	return r.db.Delete(&AgentMemory{}, id).Error
}

func (r *MemoryRepo) CountBySource(agentID, userID uint, source string) (int64, error) {
	var count int64
	if err := r.db.Model(&AgentMemory{}).
		Where("agent_id = ? AND user_id = ? AND source = ?", agentID, userID, source).
		Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (r *MemoryRepo) DeleteOldestBySource(agentID, userID uint, source string) error {
	var oldest AgentMemory
	if err := r.db.Where("agent_id = ? AND user_id = ? AND source = ?", agentID, userID, source).
		Order("updated_at ASC").
		First(&oldest).Error; err != nil {
		return err
	}
	return r.db.Delete(&AgentMemory{}, oldest.ID).Error
}

func (r *MemoryRepo) Count(agentID, userID uint) (int64, error) {
	var count int64
	if err := r.db.Model(&AgentMemory{}).
		Where("agent_id = ? AND user_id = ?", agentID, userID).
		Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

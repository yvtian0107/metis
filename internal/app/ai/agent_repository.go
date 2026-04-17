package ai

import (
	"github.com/samber/do/v2"
	"gorm.io/gorm"

	"metis/internal/database"
)

type AgentRepo struct {
	db *database.DB
}

func NewAgentRepo(i do.Injector) (*AgentRepo, error) {
	return &AgentRepo{db: do.MustInvoke[*database.DB](i)}, nil
}

func (r *AgentRepo) Create(a *Agent) error {
	return r.db.Create(a).Error
}

func (r *AgentRepo) FindByID(id uint) (*Agent, error) {
	var a Agent
	if err := r.db.First(&a, id).Error; err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *AgentRepo) FindByName(name string) (*Agent, error) {
	var a Agent
	if err := r.db.Where("name = ?", name).First(&a).Error; err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *AgentRepo) FindByCode(code string) (*Agent, error) {
	var a Agent
	if err := r.db.Where("code = ?", code).First(&a).Error; err != nil {
		return nil, err
	}
	return &a, nil
}

type AgentListParams struct {
	Keyword    string
	Type       string
	Visibility string
	UserID     uint
	Page       int
	PageSize   int
}

func (r *AgentRepo) List(params AgentListParams) ([]Agent, int64, error) {
	if params.Page < 1 {
		params.Page = 1
	}
	if params.PageSize < 1 {
		params.PageSize = 20
	}

	query := r.db.Model(&Agent{})

	if params.Keyword != "" {
		like := "%" + params.Keyword + "%"
		query = query.Where("name LIKE ? OR description LIKE ?", like, like)
	}
	if params.Type != "" {
		query = query.Where("type = ?", params.Type)
	} else {
		// By default, exclude internal agents from listings
		query = query.Where("type != ?", AgentTypeInternal)
	}

	// Visibility filter: user sees team + public + own private
	if params.UserID > 0 {
		query = query.Where(
			"visibility IN (?, ?) OR (visibility = ? AND created_by = ?)",
			AgentVisibilityTeam, AgentVisibilityPublic, AgentVisibilityPrivate, params.UserID,
		)
	}
	if params.Visibility != "" {
		query = query.Where("visibility = ?", params.Visibility)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []Agent
	offset := (params.Page - 1) * params.PageSize
	if err := query.Offset(offset).Limit(params.PageSize).
		Order("created_at DESC").
		Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *AgentRepo) Update(a *Agent) error {
	return r.db.Save(a).Error
}

func (r *AgentRepo) Delete(id uint) error {
	return r.db.Delete(&Agent{}, id).Error
}

// --- Binding helpers ---

func (r *AgentRepo) ReplaceToolBindings(agentID uint, toolIDs []uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("agent_id = ?", agentID).Delete(&AgentTool{}).Error; err != nil {
			return err
		}
		for _, tid := range toolIDs {
			if err := tx.Create(&AgentTool{AgentID: agentID, ToolID: tid}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *AgentRepo) ReplaceSkillBindings(agentID uint, skillIDs []uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("agent_id = ?", agentID).Delete(&AgentSkill{}).Error; err != nil {
			return err
		}
		for _, sid := range skillIDs {
			if err := tx.Create(&AgentSkill{AgentID: agentID, SkillID: sid}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *AgentRepo) ReplaceMCPServerBindings(agentID uint, mcpIDs []uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("agent_id = ?", agentID).Delete(&AgentMCPServer{}).Error; err != nil {
			return err
		}
		for _, mid := range mcpIDs {
			if err := tx.Create(&AgentMCPServer{AgentID: agentID, MCPServerID: mid}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *AgentRepo) ReplaceKnowledgeBaseBindings(agentID uint, kbIDs []uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("agent_id = ?", agentID).Delete(&AgentKnowledgeBase{}).Error; err != nil {
			return err
		}
		for _, kid := range kbIDs {
			if err := tx.Create(&AgentKnowledgeBase{AgentID: agentID, KnowledgeBaseID: kid}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *AgentRepo) GetToolIDs(agentID uint) ([]uint, error) {
	var bindings []AgentTool
	if err := r.db.Where("agent_id = ?", agentID).Find(&bindings).Error; err != nil {
		return nil, err
	}
	ids := make([]uint, len(bindings))
	for i, b := range bindings {
		ids[i] = b.ToolID
	}
	return ids, nil
}

func (r *AgentRepo) GetSkillIDs(agentID uint) ([]uint, error) {
	var bindings []AgentSkill
	if err := r.db.Where("agent_id = ?", agentID).Find(&bindings).Error; err != nil {
		return nil, err
	}
	ids := make([]uint, len(bindings))
	for i, b := range bindings {
		ids[i] = b.SkillID
	}
	return ids, nil
}

func (r *AgentRepo) GetMCPServerIDs(agentID uint) ([]uint, error) {
	var bindings []AgentMCPServer
	if err := r.db.Where("agent_id = ?", agentID).Find(&bindings).Error; err != nil {
		return nil, err
	}
	ids := make([]uint, len(bindings))
	for i, b := range bindings {
		ids[i] = b.MCPServerID
	}
	return ids, nil
}

func (r *AgentRepo) GetKnowledgeBaseIDs(agentID uint) ([]uint, error) {
	var bindings []AgentKnowledgeBase
	if err := r.db.Where("agent_id = ?", agentID).Find(&bindings).Error; err != nil {
		return nil, err
	}
	ids := make([]uint, len(bindings))
	for i, b := range bindings {
		ids[i] = b.KnowledgeBaseID
	}
	return ids, nil
}

// --- Template helpers ---

func (r *AgentRepo) ListTemplates() ([]AgentTemplate, error) {
	var items []AgentTemplate
	if err := r.db.Order("id ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *AgentRepo) FindTemplateByID(id uint) (*AgentTemplate, error) {
	var t AgentTemplate
	if err := r.db.First(&t, id).Error; err != nil {
		return nil, err
	}
	return &t, nil
}

// HasRunningSessions checks if agent has any sessions with status "running"
func (r *AgentRepo) HasRunningSessions(agentID uint) (bool, error) {
	var count int64
	if err := r.db.Model(&AgentSession{}).
		Where("agent_id = ? AND status = ?", agentID, SessionStatusRunning).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

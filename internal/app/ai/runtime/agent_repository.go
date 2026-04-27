package runtime

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

func (r *AgentRepo) FindAccessibleByID(id, userID uint) (*Agent, error) {
	var a Agent
	query := r.db.Where("id = ?", id)
	if userID > 0 {
		query = query.Where(
			"visibility IN (?, ?) OR (visibility = ? AND created_by = ?)",
			AgentVisibilityTeam, AgentVisibilityPublic, AgentVisibilityPrivate, userID,
		)
	} else {
		query = query.Where("visibility IN (?, ?)", AgentVisibilityTeam, AgentVisibilityPublic)
	}
	if err := query.First(&a).Error; err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *AgentRepo) FindOwnedByID(id, userID uint) (*Agent, error) {
	var a Agent
	if err := r.db.Where("id = ? AND created_by = ?", id, userID).First(&a).Error; err != nil {
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
		return r.replaceToolBindingsInTx(tx, agentID, toolIDs)
	})
}

func (r *AgentRepo) ReplaceSkillBindings(agentID uint, skillIDs []uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		return r.replaceSkillBindingsInTx(tx, agentID, skillIDs)
	})
}

func (r *AgentRepo) ReplaceMCPServerBindings(agentID uint, mcpIDs []uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		return r.replaceMCPServerBindingsInTx(tx, agentID, mcpIDs)
	})
}

func (r *AgentRepo) ReplaceKnowledgeBaseBindings(agentID uint, kbIDs []uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		return r.replaceKnowledgeBaseBindingsInTx(tx, agentID, kbIDs)
	})
}

func (r *AgentRepo) GetToolIDs(agentID uint) ([]uint, error) {
	return legacyFallbackOrSelectedIDs(r.db.DB, agentID, CapabilityTypeTool, func() ([]uint, error) {
		return r.getLegacyToolIDs(agentID)
	})
}

func (r *AgentRepo) getLegacyToolIDs(agentID uint) ([]uint, error) {
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
	return legacyFallbackOrSelectedIDs(r.db.DB, agentID, CapabilityTypeSkill, func() ([]uint, error) {
		return r.getLegacySkillIDs(agentID)
	})
}

func (r *AgentRepo) getLegacySkillIDs(agentID uint) ([]uint, error) {
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
	return legacyFallbackOrSelectedIDs(r.db.DB, agentID, CapabilityTypeMCP, func() ([]uint, error) {
		return r.getLegacyMCPServerIDs(agentID)
	})
}

func (r *AgentRepo) getLegacyMCPServerIDs(agentID uint) ([]uint, error) {
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
	return legacyFallbackOrSelectedIDs(r.db.DB, agentID, CapabilityTypeKnowledgeBase, func() ([]uint, error) {
		return r.getLegacyKnowledgeBaseIDs(agentID)
	})
}

func (r *AgentRepo) getLegacyKnowledgeBaseIDs(agentID uint) ([]uint, error) {
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

func (r *AgentRepo) ReplaceKnowledgeGraphBindings(agentID uint, kgIDs []uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		return r.replaceKnowledgeGraphBindingsInTx(tx, agentID, kgIDs)
	})
}

func (r *AgentRepo) GetKnowledgeGraphIDs(agentID uint) ([]uint, error) {
	return legacyFallbackOrSelectedIDs(r.db.DB, agentID, CapabilityTypeKnowledgeGraph, func() ([]uint, error) {
		return r.getLegacyKnowledgeGraphIDs(agentID)
	})
}

func (r *AgentRepo) getLegacyKnowledgeGraphIDs(agentID uint) ([]uint, error) {
	var bindings []AgentKnowledgeGraph
	if err := r.db.Where("agent_id = ?", agentID).Find(&bindings).Error; err != nil {
		return nil, err
	}
	ids := make([]uint, len(bindings))
	for i, b := range bindings {
		ids[i] = b.KnowledgeGraphID
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

func (r *AgentRepo) ListTemplatesByType(agentType string) ([]AgentTemplate, error) {
	var items []AgentTemplate
	if err := r.db.Where("type = ?", agentType).Order("id ASC").Find(&items).Error; err != nil {
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

func (r *AgentRepo) DB() *gorm.DB {
	return r.db.DB
}

func (r *AgentRepo) replaceBindingsInTx(tx *gorm.DB, agentID uint, bindings AgentBindings) error {
	if err := r.replaceToolBindingsInTx(tx, agentID, bindings.ToolIDs); err != nil {
		return err
	}
	if err := r.replaceSkillBindingsInTx(tx, agentID, bindings.SkillIDs); err != nil {
		return err
	}
	if err := r.replaceMCPServerBindingsInTx(tx, agentID, bindings.MCPServerIDs); err != nil {
		return err
	}
	if err := r.replaceKnowledgeBaseBindingsInTx(tx, agentID, bindings.KnowledgeBaseIDs); err != nil {
		return err
	}
	if err := r.replaceKnowledgeGraphBindingsInTx(tx, agentID, bindings.KnowledgeGraphIDs); err != nil {
		return err
	}
	return replaceCapabilityBindingsInTx(tx, agentID, bindings.CapabilitySets)
}

func (r *AgentRepo) GetCapabilitySetBindings(agentID uint) ([]AgentCapabilitySetBinding, error) {
	hasSetBindings, err := agentHasCapabilitySetBindings(r.db.DB, agentID)
	if err != nil {
		return nil, err
	}
	if hasSetBindings {
		return getAgentCapabilitySetBindings(r.db.DB, agentID)
	}
	flat, err := r.getLegacyBindings(agentID)
	if err != nil {
		return nil, err
	}
	if !flat.hasAnyFlatBinding() {
		return nil, nil
	}
	return capabilitySetBindingsFromFlat(r.db.DB, flat)
}

func (r *AgentRepo) getLegacyBindings(agentID uint) (AgentBindings, error) {
	var result AgentBindings
	var err error
	if result.ToolIDs, err = r.getLegacyToolIDs(agentID); err != nil {
		return AgentBindings{}, err
	}
	if result.SkillIDs, err = r.getLegacySkillIDs(agentID); err != nil {
		return AgentBindings{}, err
	}
	if result.MCPServerIDs, err = r.getLegacyMCPServerIDs(agentID); err != nil {
		return AgentBindings{}, err
	}
	if result.KnowledgeBaseIDs, err = r.getLegacyKnowledgeBaseIDs(agentID); err != nil {
		return AgentBindings{}, err
	}
	if result.KnowledgeGraphIDs, err = r.getLegacyKnowledgeGraphIDs(agentID); err != nil {
		return AgentBindings{}, err
	}
	return result, nil
}

func (r *AgentRepo) replaceToolBindingsInTx(tx *gorm.DB, agentID uint, toolIDs []uint) error {
	if err := tx.Where("agent_id = ?", agentID).Delete(&AgentTool{}).Error; err != nil {
		return err
	}
	for _, tid := range toolIDs {
		if err := tx.Create(&AgentTool{AgentID: agentID, ToolID: tid}).Error; err != nil {
			return err
		}
	}
	return nil
}

func (r *AgentRepo) replaceSkillBindingsInTx(tx *gorm.DB, agentID uint, skillIDs []uint) error {
	if err := tx.Where("agent_id = ?", agentID).Delete(&AgentSkill{}).Error; err != nil {
		return err
	}
	for _, sid := range skillIDs {
		if err := tx.Create(&AgentSkill{AgentID: agentID, SkillID: sid}).Error; err != nil {
			return err
		}
	}
	return nil
}

func (r *AgentRepo) replaceMCPServerBindingsInTx(tx *gorm.DB, agentID uint, mcpIDs []uint) error {
	if err := tx.Where("agent_id = ?", agentID).Delete(&AgentMCPServer{}).Error; err != nil {
		return err
	}
	for _, mid := range mcpIDs {
		if err := tx.Create(&AgentMCPServer{AgentID: agentID, MCPServerID: mid}).Error; err != nil {
			return err
		}
	}
	return nil
}

func (r *AgentRepo) replaceKnowledgeBaseBindingsInTx(tx *gorm.DB, agentID uint, kbIDs []uint) error {
	if err := tx.Where("agent_id = ?", agentID).Delete(&AgentKnowledgeBase{}).Error; err != nil {
		return err
	}
	for _, kid := range kbIDs {
		if err := tx.Create(&AgentKnowledgeBase{AgentID: agentID, KnowledgeBaseID: kid}).Error; err != nil {
			return err
		}
	}
	return nil
}

func (r *AgentRepo) replaceKnowledgeGraphBindingsInTx(tx *gorm.DB, agentID uint, kgIDs []uint) error {
	if err := tx.Where("agent_id = ?", agentID).Delete(&AgentKnowledgeGraph{}).Error; err != nil {
		return err
	}
	for _, kid := range kgIDs {
		if err := tx.Create(&AgentKnowledgeGraph{AgentID: agentID, KnowledgeGraphID: kid}).Error; err != nil {
			return err
		}
	}
	return nil
}

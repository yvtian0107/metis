package ai

import (
	"github.com/samber/do/v2"

	"metis/internal/database"
)

type ToolRepo struct {
	db *database.DB
}

func NewToolRepo(i do.Injector) (*ToolRepo, error) {
	return &ToolRepo{db: do.MustInvoke[*database.DB](i)}, nil
}

func (r *ToolRepo) List() ([]Tool, error) {
	var tools []Tool
	if err := r.db.Order("name ASC").Find(&tools).Error; err != nil {
		return nil, err
	}
	return tools, nil
}

func (r *ToolRepo) FindByID(id uint) (*Tool, error) {
	var t Tool
	if err := r.db.First(&t, id).Error; err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *ToolRepo) FindByName(name string) (*Tool, error) {
	var t Tool
	if err := r.db.Where("name = ?", name).First(&t).Error; err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *ToolRepo) Create(t *Tool) error {
	return r.db.Create(t).Error
}

func (r *ToolRepo) Update(t *Tool) error {
	return r.db.Save(t).Error
}

func (r *ToolRepo) CountBoundAgents(toolID uint) (int64, error) {
	var count int64
	err := r.db.Raw(`
		SELECT COUNT(DISTINCT agent_id) FROM (
			SELECT agent_id FROM ai_agent_tools WHERE tool_id = ?
			UNION
			SELECT ai_agent_capability_set_items.agent_id
			FROM ai_agent_capability_set_items
			JOIN ai_capability_sets ON ai_capability_sets.id = ai_agent_capability_set_items.set_id
			WHERE ai_capability_sets.type = ? AND ai_agent_capability_set_items.item_id = ? AND ai_agent_capability_set_items.enabled = ?
		) AS bound_agents
	`, toolID, CapabilityTypeTool, toolID, true).Scan(&count).Error
	return count, err
}

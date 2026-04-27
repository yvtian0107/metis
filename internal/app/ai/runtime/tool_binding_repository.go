package runtime

import (
	"github.com/samber/do/v2"

	"metis/internal/database"
)

// AgentToolRepo manages tool bindings for agents.
type AgentToolRepo struct {
	db *database.DB
}

func NewAgentToolRepo(i do.Injector) (*AgentToolRepo, error) {
	return &AgentToolRepo{db: do.MustInvoke[*database.DB](i)}, nil
}

func (r *AgentToolRepo) Bind(agentID, toolID uint) error {
	return r.db.FirstOrCreate(&AgentTool{AgentID: agentID, ToolID: toolID}).Error
}

func (r *AgentToolRepo) Unbind(agentID, toolID uint) error {
	return r.db.Where("agent_id = ? AND tool_id = ?", agentID, toolID).Delete(&AgentTool{}).Error
}

func (r *AgentToolRepo) ListByAgent(agentID uint) ([]Tool, error) {
	var tools []Tool
	hasSetBindings, err := agentHasCapabilitySetBindings(r.db.DB, agentID)
	if err != nil {
		return nil, err
	}
	if hasSetBindings {
		ids, err := selectedCapabilityItemIDsByType(r.db.DB, agentID, CapabilityTypeTool)
		if err != nil {
			return nil, err
		}
		if len(ids) == 0 {
			return tools, nil
		}
		err = r.db.Where("id IN ? AND is_active = ?", ids, true).Find(&tools).Error
		return tools, err
	}
	err = r.db.
		Joins("JOIN ai_agent_tools ON ai_agent_tools.tool_id = ai_tools.id").
		Where("ai_agent_tools.agent_id = ? AND ai_tools.is_active = ?", agentID, true).
		Find(&tools).Error
	return tools, err
}

func (r *AgentToolRepo) ReplaceForAgent(agentID uint, toolIDs []uint) error {
	if err := r.db.Where("agent_id = ?", agentID).Delete(&AgentTool{}).Error; err != nil {
		return err
	}
	for _, tid := range toolIDs {
		if err := r.db.Create(&AgentTool{AgentID: agentID, ToolID: tid}).Error; err != nil {
			return err
		}
	}
	return nil
}

// AgentMCPServerRepo manages MCP server bindings for agents.
type AgentMCPServerRepo struct {
	db *database.DB
}

func NewAgentMCPServerRepo(i do.Injector) (*AgentMCPServerRepo, error) {
	return &AgentMCPServerRepo{db: do.MustInvoke[*database.DB](i)}, nil
}

func (r *AgentMCPServerRepo) Bind(agentID, mcpServerID uint) error {
	return r.db.FirstOrCreate(&AgentMCPServer{AgentID: agentID, MCPServerID: mcpServerID}).Error
}

func (r *AgentMCPServerRepo) Unbind(agentID, mcpServerID uint) error {
	return r.db.Where("agent_id = ? AND mcp_server_id = ?", agentID, mcpServerID).Delete(&AgentMCPServer{}).Error
}

func (r *AgentMCPServerRepo) ListByAgent(agentID uint) ([]MCPServer, error) {
	var servers []MCPServer
	hasSetBindings, err := agentHasCapabilitySetBindings(r.db.DB, agentID)
	if err != nil {
		return nil, err
	}
	if hasSetBindings {
		ids, err := selectedCapabilityItemIDsByType(r.db.DB, agentID, CapabilityTypeMCP)
		if err != nil {
			return nil, err
		}
		if len(ids) == 0 {
			return servers, nil
		}
		err = r.db.Where("id IN ? AND is_active = ?", ids, true).Find(&servers).Error
		return servers, err
	}
	err = r.db.
		Joins("JOIN ai_agent_mcp_servers ON ai_agent_mcp_servers.mcp_server_id = ai_mcp_servers.id").
		Where("ai_agent_mcp_servers.agent_id = ? AND ai_mcp_servers.is_active = ?", agentID, true).
		Find(&servers).Error
	return servers, err
}

func (r *AgentMCPServerRepo) ReplaceForAgent(agentID uint, mcpServerIDs []uint) error {
	if err := r.db.Where("agent_id = ?", agentID).Delete(&AgentMCPServer{}).Error; err != nil {
		return err
	}
	for _, mid := range mcpServerIDs {
		if err := r.db.Create(&AgentMCPServer{AgentID: agentID, MCPServerID: mid}).Error; err != nil {
			return err
		}
	}
	return nil
}

// AgentSkillRepo manages skill bindings for agents.
type AgentSkillRepo struct {
	db *database.DB
}

func NewAgentSkillRepo(i do.Injector) (*AgentSkillRepo, error) {
	return &AgentSkillRepo{db: do.MustInvoke[*database.DB](i)}, nil
}

func (r *AgentSkillRepo) Bind(agentID, skillID uint) error {
	return r.db.FirstOrCreate(&AgentSkill{AgentID: agentID, SkillID: skillID}).Error
}

func (r *AgentSkillRepo) Unbind(agentID, skillID uint) error {
	return r.db.Where("agent_id = ? AND skill_id = ?", agentID, skillID).Delete(&AgentSkill{}).Error
}

func (r *AgentSkillRepo) ListByAgent(agentID uint) ([]Skill, error) {
	var skills []Skill
	hasSetBindings, err := agentHasCapabilitySetBindings(r.db.DB, agentID)
	if err != nil {
		return nil, err
	}
	if hasSetBindings {
		ids, err := selectedCapabilityItemIDsByType(r.db.DB, agentID, CapabilityTypeSkill)
		if err != nil {
			return nil, err
		}
		if len(ids) == 0 {
			return skills, nil
		}
		err = r.db.Where("id IN ? AND is_active = ?", ids, true).Find(&skills).Error
		return skills, err
	}
	err = r.db.
		Joins("JOIN ai_agent_skills ON ai_agent_skills.skill_id = ai_skills.id").
		Where("ai_agent_skills.agent_id = ? AND ai_skills.is_active = ?", agentID, true).
		Find(&skills).Error
	return skills, err
}

func (r *AgentSkillRepo) ReplaceForAgent(agentID uint, skillIDs []uint) error {
	if err := r.db.Where("agent_id = ?", agentID).Delete(&AgentSkill{}).Error; err != nil {
		return err
	}
	for _, sid := range skillIDs {
		if err := r.db.Create(&AgentSkill{AgentID: agentID, SkillID: sid}).Error; err != nil {
			return err
		}
	}
	return nil
}

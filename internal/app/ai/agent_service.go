package ai

import (
	"errors"

	"github.com/samber/do/v2"
	"gorm.io/gorm"
)

var (
	ErrAgentNotFound           = errors.New("agent not found")
	ErrAgentNameConflict       = errors.New("agent name already exists")
	ErrAgentCodeConflict       = errors.New("agent code already exists")
	ErrAgentHasRunningSessions = errors.New("agent has running sessions")
	ErrInvalidAgentType        = errors.New("invalid agent type")
	ErrNodeRequired            = errors.New("node_id is required for remote exec mode")
	ErrModelRequired           = errors.New("model_id is required for assistant agent")
	ErrRuntimeRequired         = errors.New("runtime is required for coding agent")
	ErrCodeRequired            = errors.New("code is required for internal agent")
)

var ValidAgentTypes = map[string]bool{
	AgentTypeAssistant: true,
	AgentTypeCoding:    true,
	AgentTypeInternal:  true,
}

var ValidStrategies = map[string]bool{
	AgentStrategyReact:          true,
	AgentStrategyPlanAndExecute: true,
}

var ValidRuntimes = map[string]bool{
	AgentRuntimeClaudeCode: true,
	AgentRuntimeCodex:      true,
	AgentRuntimeOpenCode:   true,
	AgentRuntimeAider:      true,
}

type AgentService struct {
	repo *AgentRepo
}

func NewAgentService(i do.Injector) (*AgentService, error) {
	return &AgentService{
		repo: do.MustInvoke[*AgentRepo](i),
	}, nil
}

func (s *AgentService) Create(a *Agent) error {
	if !ValidAgentTypes[a.Type] {
		return ErrInvalidAgentType
	}
	if err := s.validateByType(a); err != nil {
		return err
	}

	// Check name uniqueness
	if _, err := s.repo.FindByName(a.Name); err == nil {
		return ErrAgentNameConflict
	}

	// Check code uniqueness for internal agents
	if a.Code != nil && *a.Code != "" {
		if _, err := s.repo.FindByCode(*a.Code); err == nil {
			return ErrAgentCodeConflict
		}
	}

	return s.repo.Create(a)
}

func (s *AgentService) Get(id uint) (*Agent, error) {
	a, err := s.repo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAgentNotFound
		}
		return nil, err
	}
	return a, nil
}

func (s *AgentService) GetAccessible(id, userID uint) (*Agent, error) {
	a, err := s.repo.FindAccessibleByID(id, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAgentNotFound
		}
		return nil, err
	}
	return a, nil
}

func (s *AgentService) GetOwned(id, userID uint) (*Agent, error) {
	a, err := s.repo.FindOwnedByID(id, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAgentNotFound
		}
		return nil, err
	}
	return a, nil
}

func (s *AgentService) GetByCode(code string) (*Agent, error) {
	a, err := s.repo.FindByCode(code)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAgentNotFound
		}
		return nil, err
	}
	return a, nil
}

func (s *AgentService) Update(a *Agent) error {
	if err := s.validateByType(a); err != nil {
		return err
	}
	return s.repo.Update(a)
}

func (s *AgentService) Delete(id uint) error {
	hasRunning, err := s.repo.HasRunningSessions(id)
	if err != nil {
		return err
	}
	if hasRunning {
		return ErrAgentHasRunningSessions
	}
	return s.repo.Delete(id)
}

func (s *AgentService) List(params AgentListParams) ([]Agent, int64, error) {
	return s.repo.List(params)
}

func (s *AgentService) validateByType(a *Agent) error {
	switch a.Type {
	case AgentTypeAssistant:
		if a.ModelID == nil {
			return ErrModelRequired
		}
		if a.Strategy == "" {
			a.Strategy = AgentStrategyReact
		}
		if !ValidStrategies[a.Strategy] {
			return errors.New("invalid strategy: " + a.Strategy)
		}
	case AgentTypeCoding:
		if a.Runtime == "" {
			return ErrRuntimeRequired
		}
		if !ValidRuntimes[a.Runtime] {
			return errors.New("invalid runtime: " + a.Runtime)
		}
		if a.ExecMode == "" {
			a.ExecMode = AgentExecModeLocal
		}
		if a.ExecMode == AgentExecModeRemote && a.NodeID == nil {
			return ErrNodeRequired
		}
	case AgentTypeInternal:
		if a.Code == nil || *a.Code == "" {
			return ErrCodeRequired
		}
	}
	return nil
}

// UpdateBindings replaces all bindings for the given agent
func (s *AgentService) UpdateBindings(agentID uint, toolIDs, skillIDs, mcpIDs, kbIDs []uint) error {
	if err := s.repo.ReplaceToolBindings(agentID, toolIDs); err != nil {
		return err
	}
	if err := s.repo.ReplaceSkillBindings(agentID, skillIDs); err != nil {
		return err
	}
	if err := s.repo.ReplaceMCPServerBindings(agentID, mcpIDs); err != nil {
		return err
	}
	return s.repo.ReplaceKnowledgeBaseBindings(agentID, kbIDs)
}

// GetBindings returns all binding IDs for an agent
func (s *AgentService) GetBindings(agentID uint) (toolIDs, skillIDs, mcpIDs, kbIDs []uint, err error) {
	toolIDs, err = s.repo.GetToolIDs(agentID)
	if err != nil {
		return
	}
	skillIDs, err = s.repo.GetSkillIDs(agentID)
	if err != nil {
		return
	}
	mcpIDs, err = s.repo.GetMCPServerIDs(agentID)
	if err != nil {
		return
	}
	kbIDs, err = s.repo.GetKnowledgeBaseIDs(agentID)
	return
}

// ListTemplates returns all agent templates
func (s *AgentService) ListTemplates() ([]AgentTemplate, error) {
	return s.repo.ListTemplates()
}

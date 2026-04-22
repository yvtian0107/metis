package itsm

import (
	"errors"
	"strconv"

	"github.com/samber/do/v2"
	"gorm.io/gorm"

	"metis/internal/app/ai"
	"metis/internal/database"
	"metis/internal/model"
	"metis/internal/repository"
)

var (
	ErrEngineNotConfigured  = errors.New("工作流解析引擎未配置，请前往引擎配置页面设置")
	ErrModelNotFound        = errors.New("模型不存在或已停用")
	ErrAgentNotFound        = errors.New("智能体不存在或已停用")
	ErrFallbackUserNotFound = errors.New("兜底处理人不存在或已停用")
)

// EngineConfigService manages ITSM engine configuration.
// It aggregates AI Agent records (for LLM config) and SystemConfig (for ITSM-specific settings).
type EngineConfigService struct {
	agentSvc      *ai.AgentService
	modelRepo     *ai.ModelRepo
	sysConfigRepo *repository.SysConfigRepo
	db            *gorm.DB
}

func NewEngineConfigService(i do.Injector) (*EngineConfigService, error) {
	db := do.MustInvoke[*database.DB](i)
	return &EngineConfigService{
		agentSvc:      do.MustInvoke[*ai.AgentService](i),
		modelRepo:     do.MustInvoke[*ai.ModelRepo](i),
		sysConfigRepo: do.MustInvoke[*repository.SysConfigRepo](i),
		db:            db.DB,
	}, nil
}

// EngineConfig is the aggregated engine configuration response.
type EngineConfig struct {
	Generator   EngineAgentConfig    `json:"generator"`
	Servicedesk EngineAgentSelector  `json:"servicedesk"`
	Decision    EngineDecisionConfig `json:"decision"`
	General     EngineGeneralConfig  `json:"general"`
}

type EngineAgentConfig struct {
	ModelID      uint    `json:"modelId"`
	ProviderID   uint    `json:"providerId"`
	ProviderName string  `json:"providerName"`
	ModelName    string  `json:"modelName"`
	Temperature  float64 `json:"temperature"`
}

type EngineAgentSelector struct {
	AgentID   uint   `json:"agentId"`
	AgentName string `json:"agentName"`
}

type EngineDecisionConfig struct {
	EngineAgentSelector
	DecisionMode string `json:"decisionMode"`
}

type EngineGeneralConfig struct {
	MaxRetries       int    `json:"maxRetries"`
	TimeoutSeconds   int    `json:"timeoutSeconds"`
	ReasoningLog     string `json:"reasoningLog"`
	FallbackAssignee uint   `json:"fallbackAssignee"`
}

// GetConfig returns the aggregated engine configuration.
func (s *EngineConfigService) GetConfig() (*EngineConfig, error) {
	cfg := &EngineConfig{}

	// Read generator agent config (still Provider → Model)
	cfg.Generator = s.readAgentConfig("itsm.generator")

	// Read servicedesk agent selector
	cfg.Servicedesk = s.readAgentSelector("itsm.engine.servicedesk.agent_id")

	// Read decision agent selector + decision mode
	cfg.Decision = EngineDecisionConfig{
		EngineAgentSelector: s.readAgentSelector("itsm.engine.decision.agent_id"),
		DecisionMode:        s.getConfigValue("itsm.engine.decision.decision_mode", "direct_first"),
	}

	// Read general settings
	cfg.General = EngineGeneralConfig{
		MaxRetries:       s.getConfigInt("itsm.engine.general.max_retries", 3),
		TimeoutSeconds:   s.getConfigInt("itsm.engine.general.timeout_seconds", 30),
		ReasoningLog:     s.getConfigValue("itsm.engine.general.reasoning_log", "full"),
		FallbackAssignee: uint(s.getConfigInt("itsm.engine.general.fallback_assignee", 0)),
	}

	return cfg, nil
}

// UpdateConfigRequest is the request body for updating engine config.
type UpdateConfigRequest struct {
	Generator struct {
		ModelID     uint    `json:"modelId"`
		Temperature float64 `json:"temperature"`
	} `json:"generator"`
	Servicedesk struct {
		AgentID uint `json:"agentId"`
	} `json:"servicedesk"`
	Decision struct {
		AgentID      uint   `json:"agentId"`
		DecisionMode string `json:"decisionMode"`
	} `json:"decision"`
	General struct {
		MaxRetries       int    `json:"maxRetries"`
		TimeoutSeconds   int    `json:"timeoutSeconds"`
		ReasoningLog     string `json:"reasoningLog"`
		FallbackAssignee uint   `json:"fallbackAssignee"`
	} `json:"general"`
}

// UpdateConfig updates the aggregated engine configuration.
func (s *EngineConfigService) UpdateConfig(req *UpdateConfigRequest) error {
	// Validate generator model ID if provided
	if req.Generator.ModelID > 0 {
		if _, err := s.modelRepo.FindByID(req.Generator.ModelID); err != nil {
			return ErrModelNotFound
		}
	}

	// Validate servicedesk agent ID
	if req.Servicedesk.AgentID > 0 {
		if err := s.validateActiveAgent(req.Servicedesk.AgentID); err != nil {
			return err
		}
	}

	// Validate decision agent ID
	if req.Decision.AgentID > 0 {
		if err := s.validateActiveAgent(req.Decision.AgentID); err != nil {
			return err
		}
	}

	// Update generator agent (still Provider → Model)
	if err := s.updateAgentConfig("itsm.generator", req.Generator.ModelID, req.Generator.Temperature); err != nil {
		return err
	}

	// Update servicedesk/decision agent_id in SystemConfig
	s.setConfigValue("itsm.engine.servicedesk.agent_id", strconv.FormatUint(uint64(req.Servicedesk.AgentID), 10))
	s.setConfigValue("itsm.engine.decision.agent_id", strconv.FormatUint(uint64(req.Decision.AgentID), 10))

	// Validate fallback assignee if provided
	if req.General.FallbackAssignee > 0 {
		var user struct{ IsActive bool }
		if err := s.db.Table("users").Where("id = ? AND deleted_at IS NULL", req.General.FallbackAssignee).
			Select("is_active").First(&user).Error; err != nil {
			return ErrFallbackUserNotFound
		}
		if !user.IsActive {
			return ErrFallbackUserNotFound
		}
	}

	// Update SystemConfig values
	s.setConfigValue("itsm.engine.decision.decision_mode", req.Decision.DecisionMode)
	s.setConfigValue("itsm.engine.general.max_retries", strconv.Itoa(req.General.MaxRetries))
	s.setConfigValue("itsm.engine.general.timeout_seconds", strconv.Itoa(req.General.TimeoutSeconds))
	s.setConfigValue("itsm.engine.general.reasoning_log", req.General.ReasoningLog)
	s.setConfigValue("itsm.engine.general.fallback_assignee", strconv.FormatUint(uint64(req.General.FallbackAssignee), 10))

	return nil
}

// readAgentConfig reads an internal agent's LLM config by code, enriching with provider/model names.
func (s *EngineConfigService) readAgentConfig(code string) EngineAgentConfig {
	cfg := EngineAgentConfig{}

	agent, err := s.agentSvc.GetByCode(code)
	if err != nil {
		return cfg
	}

	cfg.Temperature = agent.Temperature

	if agent.ModelID == nil || *agent.ModelID == 0 {
		return cfg
	}
	cfg.ModelID = *agent.ModelID

	// Enrich with model + provider info
	m, err := s.modelRepo.FindByID(*agent.ModelID)
	if err != nil {
		return cfg
	}
	cfg.ModelName = m.DisplayName
	cfg.ProviderID = m.ProviderID
	if m.Provider != nil {
		cfg.ProviderName = m.Provider.Name
	}

	return cfg
}

// readAgentSelector reads an agent_id from SystemConfig and returns id + name.
func (s *EngineConfigService) readAgentSelector(configKey string) EngineAgentSelector {
	agentID := uint(s.getConfigInt(configKey, 0))
	if agentID == 0 {
		return EngineAgentSelector{}
	}
	agent, err := s.agentSvc.Get(agentID)
	if err != nil {
		return EngineAgentSelector{}
	}
	return EngineAgentSelector{AgentID: agent.ID, AgentName: agent.Name}
}

// validateActiveAgent checks that an agent exists and is active.
func (s *EngineConfigService) validateActiveAgent(agentID uint) error {
	agent, err := s.agentSvc.Get(agentID)
	if err != nil {
		return ErrAgentNotFound
	}
	if !agent.IsActive {
		return ErrAgentNotFound
	}
	return nil
}

// updateAgentConfig updates an internal agent's model_id and temperature.
func (s *EngineConfigService) updateAgentConfig(code string, modelID uint, temperature float64) error {
	agent, err := s.agentSvc.GetByCode(code)
	if err != nil {
		return err
	}

	if modelID > 0 {
		agent.ModelID = &modelID
	} else {
		agent.ModelID = nil
	}
	agent.Temperature = temperature

	return s.agentSvc.Update(agent)
}

func (s *EngineConfigService) getConfigValue(key, defaultVal string) string {
	cfg, err := s.sysConfigRepo.Get(key)
	if err != nil {
		return defaultVal
	}
	if cfg.Value == "" {
		return defaultVal
	}
	return cfg.Value
}

func (s *EngineConfigService) getConfigInt(key string, defaultVal int) int {
	v := s.getConfigValue(key, "")
	if v == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return defaultVal
	}
	return n
}

func (s *EngineConfigService) setConfigValue(key, value string) {
	cfg, err := s.sysConfigRepo.Get(key)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			_ = s.sysConfigRepo.Set(&model.SystemConfig{Key: key, Value: value})
			return
		}
		return
	}
	cfg.Value = value
	_ = s.sysConfigRepo.Set(cfg)
}

// FallbackAssigneeID returns the configured fallback assignee user ID (0 = not configured).
// Implements engine.EngineConfigProvider.
func (s *EngineConfigService) FallbackAssigneeID() uint {
	return uint(s.getConfigInt("itsm.engine.general.fallback_assignee", 0))
}

// DecisionMode returns the decision mode ("direct_first" or "ai_only").
// Implements engine.EngineConfigProvider.
func (s *EngineConfigService) DecisionMode() string {
	return s.getConfigValue("itsm.engine.decision.decision_mode", "direct_first")
}

// DecisionAgentID returns the configured decision agent ID (0 = not configured).
// Implements engine.EngineConfigProvider.
func (s *EngineConfigService) DecisionAgentID() uint {
	return uint(s.getConfigInt("itsm.engine.decision.agent_id", 0))
}

func (s *EngineConfigService) ServicedeskAgentID() uint {
	return uint(s.getConfigInt("itsm.engine.servicedesk.agent_id", 0))
}

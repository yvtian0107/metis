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
	ErrEngineNotConfigured   = errors.New("工作流解析引擎未配置，请前往引擎配置页面设置")
	ErrModelNotFound         = errors.New("模型不存在或已停用")
	ErrFallbackUserNotFound  = errors.New("兜底处理人不存在或已停用")
)

// EngineConfigService manages ITSM engine configuration.
// It aggregates AI Agent records (for LLM config) and SystemConfig (for ITSM-specific settings).
type EngineConfigService struct {
	agentSvc      *ai.AgentService
	modelRepo     *ai.ModelRepo
	providerRepo  *ai.ProviderRepo
	sysConfigRepo *repository.SysConfigRepo
	db            *gorm.DB
}

func NewEngineConfigService(i do.Injector) (*EngineConfigService, error) {
	db := do.MustInvoke[*database.DB](i)
	return &EngineConfigService{
		agentSvc:      do.MustInvoke[*ai.AgentService](i),
		modelRepo:     do.MustInvoke[*ai.ModelRepo](i),
		providerRepo:  do.MustInvoke[*ai.ProviderRepo](i),
		sysConfigRepo: do.MustInvoke[*repository.SysConfigRepo](i),
		db:            db.DB,
	}, nil
}

// EngineConfig is the aggregated engine configuration response.
type EngineConfig struct {
	Generator   EngineAgentConfig    `json:"generator"`
	Servicedesk EngineAgentConfig    `json:"servicedesk"`
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

type EngineDecisionConfig struct {
	EngineAgentConfig
	DecisionMode string `json:"decisionMode"`
}

type EngineGeneralConfig struct {
	MaxRetries        int    `json:"maxRetries"`
	TimeoutSeconds    int    `json:"timeoutSeconds"`
	ReasoningLog      string `json:"reasoningLog"`
	FallbackAssignee  uint   `json:"fallbackAssignee"`
}

// GetConfig returns the aggregated engine configuration.
func (s *EngineConfigService) GetConfig() (*EngineConfig, error) {
	cfg := &EngineConfig{}

	// Read generator agent config
	cfg.Generator = s.readAgentConfig("itsm.generator")

	// Read servicedesk agent config
	cfg.Servicedesk = s.readAgentConfig("itsm.servicedesk")

	// Read decision agent config
	decisionAgent := s.readAgentConfig("itsm.decision")
	cfg.Decision = EngineDecisionConfig{
		EngineAgentConfig: decisionAgent,
		DecisionMode:      s.getConfigValue("itsm.engine.decision.decision_mode", "direct_first"),
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
		ModelID     uint    `json:"modelId"`
		Temperature float64 `json:"temperature"`
	} `json:"servicedesk"`
	Decision struct {
		ModelID      uint    `json:"modelId"`
		Temperature  float64 `json:"temperature"`
		DecisionMode string  `json:"decisionMode"`
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
	// Validate model IDs if provided
	if req.Generator.ModelID > 0 {
		if _, err := s.modelRepo.FindByID(req.Generator.ModelID); err != nil {
			return ErrModelNotFound
		}
	}
	if req.Servicedesk.ModelID > 0 {
		if _, err := s.modelRepo.FindByID(req.Servicedesk.ModelID); err != nil {
			return ErrModelNotFound
		}
	}
	if req.Decision.ModelID > 0 {
		if _, err := s.modelRepo.FindByID(req.Decision.ModelID); err != nil {
			return ErrModelNotFound
		}
	}

	// Update generator agent
	if err := s.updateAgentConfig("itsm.generator", req.Generator.ModelID, req.Generator.Temperature); err != nil {
		return err
	}

	// Update servicedesk agent
	if err := s.updateAgentConfig("itsm.servicedesk", req.Servicedesk.ModelID, req.Servicedesk.Temperature); err != nil {
		return err
	}

	// Update decision agent
	if err := s.updateAgentConfig("itsm.decision", req.Decision.ModelID, req.Decision.Temperature); err != nil {
		return err
	}

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

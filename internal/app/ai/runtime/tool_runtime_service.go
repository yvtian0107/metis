package runtime

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/samber/do/v2"

	"metis/internal/model"
)

const (
	ToolRuntimeKindLLM          = "llm"
	serviceMatchRuntimeToolName = "itsm.service_match"
)

var (
	ErrToolRuntimeNotConfigured = errors.New("tool runtime is not configured")
	ErrToolRuntimeInvalid       = errors.New("tool runtime config is invalid")
)

type ToolRuntimeService struct {
	toolRepo    *ToolRepo
	modelRepo   *ModelRepo
	providerSvc *ProviderService
}

func NewToolRuntimeService(i do.Injector) (*ToolRuntimeService, error) {
	return &ToolRuntimeService{
		toolRepo:    do.MustInvoke[*ToolRepo](i),
		modelRepo:   do.MustInvoke[*ModelRepo](i),
		providerSvc: do.MustInvoke[*ProviderService](i),
	}, nil
}

type UpdateToolRuntimeRequest struct {
	RuntimeConfig json.RawMessage `json:"runtimeConfig"`
}

type LLMToolRuntimeConfig struct {
	Model          string
	Protocol       string
	BaseURL        string
	APIKey         string
	Temperature    float64
	MaxTokens      int
	TimeoutSeconds int
}

type serviceMatchRuntimeConfig struct {
	ModelID        uint    `json:"modelId"`
	Temperature    float64 `json:"temperature"`
	MaxTokens      int     `json:"maxTokens"`
	TimeoutSeconds int     `json:"timeoutSeconds"`
}

func (s *ToolRuntimeService) UpdateRuntimeConfig(toolID uint, raw json.RawMessage) (*ToolResponse, error) {
	t, err := s.toolRepo.FindByID(toolID)
	if err != nil {
		return nil, ErrToolNotFound
	}
	if len(t.RuntimeConfigSchema) == 0 {
		return nil, ErrToolRuntimeNotConfigured
	}
	if err := s.validateRuntimeConfig(*t, raw); err != nil {
		return nil, err
	}
	t.RuntimeConfig = model.JSONText(raw)
	if err := s.toolRepo.Update(t); err != nil {
		return nil, err
	}
	resp := t.ToResponse()
	return &resp, nil
}

func (s *ToolRuntimeService) LLMRuntimeConfig(toolName string) (LLMToolRuntimeConfig, error) {
	t, err := s.toolRepo.FindByName(toolName)
	if err != nil {
		return LLMToolRuntimeConfig{}, ErrToolNotFound
	}
	if !t.IsActive {
		return LLMToolRuntimeConfig{}, ErrToolNotExecutable
	}
	if len(t.RuntimeConfigSchema) == 0 {
		return LLMToolRuntimeConfig{}, ErrToolRuntimeNotConfigured
	}
	if err := s.validateRuntimeConfig(*t, json.RawMessage(t.RuntimeConfig)); err != nil {
		return LLMToolRuntimeConfig{}, err
	}
	switch t.Name {
	case serviceMatchRuntimeToolName:
		cfg, err := decodeServiceMatchRuntimeConfig(json.RawMessage(t.RuntimeConfig))
		if err != nil {
			return LLMToolRuntimeConfig{}, err
		}
		model, err := s.activeLLMModel(cfg.ModelID)
		if err != nil {
			return LLMToolRuntimeConfig{}, err
		}
		apiKey, err := s.providerSvc.DecryptAPIKey(model.Provider)
		if err != nil {
			return LLMToolRuntimeConfig{}, fmt.Errorf("API Key 解密失败: %w", err)
		}
		return LLMToolRuntimeConfig{
			Model:          model.ModelID,
			Protocol:       ProtocolForType(model.Provider.Type),
			BaseURL:        model.Provider.BaseURL,
			APIKey:         apiKey,
			Temperature:    cfg.Temperature,
			MaxTokens:      cfg.MaxTokens,
			TimeoutSeconds: cfg.TimeoutSeconds,
		}, nil
	default:
		return LLMToolRuntimeConfig{}, fmt.Errorf("%w: unsupported LLM runtime tool %s", ErrToolRuntimeInvalid, t.Name)
	}
}

func (s *ToolRuntimeService) ValidateRuntimeConfig(t Tool) error {
	if len(t.RuntimeConfigSchema) == 0 {
		return nil
	}
	return s.validateRuntimeConfig(t, json.RawMessage(t.RuntimeConfig))
}

func (s *ToolRuntimeService) validateRuntimeConfig(t Tool, raw json.RawMessage) error {
	switch t.Name {
	case serviceMatchRuntimeToolName:
		cfg, err := decodeServiceMatchRuntimeConfig(raw)
		if err != nil {
			return err
		}
		if _, err := s.activeLLMModel(cfg.ModelID); err != nil {
			return err
		}
		return nil
	default:
		if len(raw) == 0 {
			return ErrToolRuntimeNotConfigured
		}
		return nil
	}
}

func decodeServiceMatchRuntimeConfig(raw json.RawMessage) (serviceMatchRuntimeConfig, error) {
	if len(raw) == 0 {
		return serviceMatchRuntimeConfig{}, fmt.Errorf("%w: 服务匹配运行时未配置", ErrToolRuntimeNotConfigured)
	}
	var cfg serviceMatchRuntimeConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return serviceMatchRuntimeConfig{}, fmt.Errorf("%w: %v", ErrToolRuntimeInvalid, err)
	}
	if cfg.ModelID == 0 {
		return serviceMatchRuntimeConfig{}, fmt.Errorf("%w: 服务匹配运行时未选择模型", ErrToolRuntimeNotConfigured)
	}
	if cfg.Temperature < 0 || cfg.Temperature > 1 {
		return serviceMatchRuntimeConfig{}, fmt.Errorf("%w: 温度必须在 0 到 1 之间", ErrToolRuntimeInvalid)
	}
	if cfg.MaxTokens < 256 || cfg.MaxTokens > 8192 {
		return serviceMatchRuntimeConfig{}, fmt.Errorf("%w: 最大输出 Token 必须在 256 到 8192 之间", ErrToolRuntimeInvalid)
	}
	if cfg.TimeoutSeconds < 5 || cfg.TimeoutSeconds > 300 {
		return serviceMatchRuntimeConfig{}, fmt.Errorf("%w: 超时时间必须在 5 到 300 秒之间", ErrToolRuntimeInvalid)
	}
	return cfg, nil
}

func (s *ToolRuntimeService) activeLLMModel(modelID uint) (*AIModel, error) {
	m, err := s.modelRepo.FindByID(modelID)
	if err != nil || m.Status != ModelStatusActive || m.Type != ModelTypeLLM || m.Provider == nil || m.Provider.Status != ProviderStatusActive {
		return nil, ErrModelNotFound
	}
	return m, nil
}

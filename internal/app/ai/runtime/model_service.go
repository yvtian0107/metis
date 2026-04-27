package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/samber/do/v2"
	openai "github.com/sashabaranov/go-openai"
	"gorm.io/gorm"

	"metis/internal/model"
	"metis/internal/pkg/crypto"
)

var (
	ErrModelNotFound = errors.New("model not found")
	ErrInvalidType   = errors.New("invalid model type")
	ErrInvalidStatus = errors.New("invalid model status")
)

type ModelService struct {
	repo         *ModelRepo
	providerRepo *ProviderRepo
	encKey       crypto.EncryptionKey
}

func NewModelService(i do.Injector) (*ModelService, error) {
	return &ModelService{
		repo:         do.MustInvoke[*ModelRepo](i),
		providerRepo: do.MustInvoke[*ProviderRepo](i),
		encKey:       do.MustInvoke[crypto.EncryptionKey](i),
	}, nil
}

func (s *ModelService) Create(m *AIModel) error {
	if err := normalizeModel(m); err != nil {
		return err
	}
	return s.repo.Create(m)
}

func (s *ModelService) Get(id uint) (*AIModel, error) {
	m, err := s.repo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrModelNotFound
		}
		return nil, err
	}
	return m, nil
}

func (s *ModelService) Update(m *AIModel) error {
	if err := normalizeModel(m); err != nil {
		return err
	}
	return s.repo.Update(m)
}

func normalizeModel(m *AIModel) error {
	if m.Type != "" && !ValidModelTypes[m.Type] {
		return ErrInvalidType
	}
	if m.Status != "" && !ValidModelStatuses[m.Status] {
		return ErrInvalidStatus
	}
	if m.Type != ModelTypeLLM {
		m.Capabilities = model.JSONText("[]")
	}
	return nil
}

func (s *ModelService) Delete(id uint) error {
	return s.repo.Delete(id)
}

func (s *ModelService) SetDefault(id uint) error {
	m, err := s.repo.FindByID(id)
	if err != nil {
		return ErrModelNotFound
	}
	return s.repo.DB().Transaction(func(tx *gorm.DB) error {
		if err := s.repo.ClearDefaultByProviderAndType(tx, m.ProviderID, m.Type); err != nil {
			return err
		}
		return s.repo.SetDefaultInTx(tx, m.ID)
	})
}

func (s *ModelService) SyncModels(ctx context.Context, providerID uint) (int, error) {
	p, err := s.providerRepo.FindByID(providerID)
	if err != nil {
		return 0, ErrProviderNotFound
	}

	apiKey, err := decryptAPIKey(p.APIKeyEncrypted, s.encKey)
	if err != nil {
		return 0, fmt.Errorf("decrypt api key: %w", err)
	}

	switch p.Type {
	case ProviderTypeAnthropic:
		return s.syncAnthropicModels(p.ID)
	default:
		return s.syncOpenAIModels(ctx, p, apiKey)
	}
}

func (s *ModelService) syncOpenAIModels(ctx context.Context, p *Provider, apiKey string) (int, error) {
	cfg := openai.DefaultConfig(apiKey)
	if p.BaseURL != "" {
		cfg.BaseURL = p.BaseURL
	}
	client := openai.NewClientWithConfig(cfg)

	resp, err := client.ListModels(ctx)
	if err != nil {
		return 0, fmt.Errorf("list models: %w", err)
	}

	added := 0
	for _, m := range resp.Models {
		_, err := s.repo.FindByModelIDAndProvider(m.ID, p.ID)
		if err == nil {
			continue // already exists
		}
		newModel := &AIModel{
			ModelID:      m.ID,
			DisplayName:  m.ID,
			ProviderID:   p.ID,
			Type:         guessModelType(m.ID),
			Capabilities: model.JSONText("[]"),
			Status:       ModelStatusActive,
		}
		if err := s.repo.Create(newModel); err != nil {
			continue
		}
		added++
	}
	return added, nil
}

func (s *ModelService) syncAnthropicModels(providerID uint) (int, error) {
	added := 0
	for _, preset := range AnthropicPresetModels {
		_, err := s.repo.FindByModelIDAndProvider(preset.ModelID, providerID)
		if err == nil {
			continue
		}
		caps, _ := json.Marshal(preset.Capabilities)
		m := &AIModel{
			ModelID:         preset.ModelID,
			DisplayName:     preset.DisplayName,
			ProviderID:      providerID,
			Type:            preset.Type,
			Capabilities:    caps,
			ContextWindow:   preset.ContextWindow,
			MaxOutputTokens: preset.MaxOutputTokens,
			Status:          ModelStatusActive,
		}
		if err := s.repo.Create(m); err != nil {
			continue
		}
		added++
	}
	return added, nil
}

func guessModelType(modelID string) string {
	id := strings.ToLower(modelID)
	switch {
	case strings.Contains(id, "embed"):
		return ModelTypeEmbed
	case strings.Contains(id, "rerank"):
		return ModelTypeRerank
	case strings.Contains(id, "tts"):
		return ModelTypeTTS
	case strings.Contains(id, "whisper"), strings.Contains(id, "stt"):
		return ModelTypeSTT
	case strings.Contains(id, "dall-e"), strings.Contains(id, "gpt-image"):
		return ModelTypeImage
	}
	// Known LLM prefixes/keywords
	llmPatterns := []string{
		"gpt", "o1", "o3", "o4",
		"claude", "llama", "qwen", "mistral", "gemma", "phi-",
		"deepseek", "glm", "yi-", "baichuan", "internlm",
		"codex", "chatglm", "command",
	}
	for _, p := range llmPatterns {
		if strings.Contains(id, p) {
			return ModelTypeLLM
		}
	}
	return ModelTypeOther
}

func decryptAPIKey(encrypted []byte, key crypto.EncryptionKey) (string, error) {
	if len(encrypted) == 0 {
		return "", nil
	}
	plain, err := crypto.Decrypt(encrypted, key)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

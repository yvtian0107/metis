package ai

import (
	"encoding/json"
	"time"

	"metis/internal/model"
)

// Provider types (brand identity)
const (
	ProviderTypeOpenAI    = "openai"
	ProviderTypeAnthropic = "anthropic"
	ProviderTypeOllama    = "ollama"
)

// Provider statuses
const (
	ProviderStatusActive   = "active"
	ProviderStatusInactive = "inactive"
	ProviderStatusError    = "error"
)

// Model types
const (
	ModelTypeLLM    = "llm"
	ModelTypeEmbed  = "embed"
	ModelTypeRerank = "rerank"
	ModelTypeTTS    = "tts"
	ModelTypeSTT    = "stt"
	ModelTypeImage  = "image"
)

// Model statuses
const (
	ModelStatusActive     = "active"
	ModelStatusDeprecated = "deprecated"
)

// AILog statuses
const (
	LogStatusSuccess = "success"
	LogStatusError   = "error"
	LogStatusTimeout = "timeout"
)

// ValidModelTypes is the set of valid model type strings.
var ValidModelTypes = map[string]bool{
	ModelTypeLLM: true, ModelTypeEmbed: true, ModelTypeRerank: true,
	ModelTypeTTS: true, ModelTypeSTT: true, ModelTypeImage: true,
}

// ValidCapabilities for LLM models.
var ValidCapabilities = map[string]bool{
	"vision": true, "tool_use": true, "reasoning": true,
	"coding": true, "long_context": true,
}

// ProtocolForType returns the LLM protocol for a provider type.
func ProtocolForType(providerType string) string {
	switch providerType {
	case ProviderTypeAnthropic:
		return "anthropic"
	default:
		return "openai"
	}
}

// --- Provider ---

type Provider struct {
	model.BaseModel
	Name            string     `json:"name" gorm:"size:128;not null"`
	Type            string     `json:"type" gorm:"size:32;not null"`
	Protocol        string     `json:"protocol" gorm:"size:32;not null"`
	BaseURL         string     `json:"baseUrl" gorm:"size:512;not null"`
	APIKeyEncrypted []byte     `json:"-" gorm:"column:api_key_encrypted;type:bytes"`
	Status          string     `json:"status" gorm:"size:16;not null;default:inactive"`
	HealthCheckedAt *time.Time `json:"healthCheckedAt"`
}

func (Provider) TableName() string { return "ai_providers" }

type ProviderResponse struct {
	ID              uint           `json:"id"`
	Name            string         `json:"name"`
	Type            string         `json:"type"`
	Protocol        string         `json:"protocol"`
	BaseURL         string         `json:"baseUrl"`
	APIKeyMasked    string         `json:"apiKeyMasked"`
	Status          string         `json:"status"`
	HealthCheckedAt *time.Time     `json:"healthCheckedAt"`
	ModelCount      int            `json:"modelCount"`
	ModelTypeCounts map[string]int `json:"modelTypeCounts"`
	CreatedAt       time.Time      `json:"createdAt"`
	UpdatedAt       time.Time      `json:"updatedAt"`
}

func (p *Provider) ToResponse(apiKeyMasked string, modelCount int, modelTypeCounts map[string]int) ProviderResponse {
	if modelTypeCounts == nil {
		modelTypeCounts = map[string]int{}
	}

	return ProviderResponse{
		ID:              p.ID,
		Name:            p.Name,
		Type:            p.Type,
		Protocol:        p.Protocol,
		BaseURL:         p.BaseURL,
		APIKeyMasked:    apiKeyMasked,
		Status:          p.Status,
		HealthCheckedAt: p.HealthCheckedAt,
		ModelCount:      modelCount,
		ModelTypeCounts: modelTypeCounts,
		CreatedAt:       p.CreatedAt,
		UpdatedAt:       p.UpdatedAt,
	}
}

// --- Model ---

type AIModel struct {
	model.BaseModel
	ModelID         string         `json:"modelId" gorm:"size:128;not null"`
	DisplayName     string         `json:"displayName" gorm:"size:128;not null"`
	ProviderID      uint           `json:"providerId" gorm:"not null;index"`
	Type            string         `json:"type" gorm:"size:16;not null;index"`
	Capabilities    model.JSONText `json:"capabilities" gorm:"type:text"`
	ContextWindow   int            `json:"contextWindow"`
	MaxOutputTokens int            `json:"maxOutputTokens"`
	InputPrice      float64        `json:"inputPrice"`
	OutputPrice     float64        `json:"outputPrice"`
	IsDefault       bool           `json:"isDefault" gorm:"not null;default:false"`
	Status          string         `json:"status" gorm:"size:16;not null;default:active"`
	Provider        *Provider      `json:"provider,omitempty" gorm:"foreignKey:ProviderID"`
}

func (AIModel) TableName() string { return "ai_models" }

type AIModelResponse struct {
	ID              uint            `json:"id"`
	ModelID         string          `json:"modelId"`
	DisplayName     string          `json:"displayName"`
	ProviderID      uint            `json:"providerId"`
	ProviderName    string          `json:"providerName,omitempty"`
	Type            string          `json:"type"`
	Capabilities    json.RawMessage `json:"capabilities"`
	ContextWindow   int             `json:"contextWindow"`
	MaxOutputTokens int             `json:"maxOutputTokens"`
	InputPrice      float64         `json:"inputPrice"`
	OutputPrice     float64         `json:"outputPrice"`
	IsDefault       bool            `json:"isDefault"`
	Status          string          `json:"status"`
	CreatedAt       time.Time       `json:"createdAt"`
	UpdatedAt       time.Time       `json:"updatedAt"`
}

func (m *AIModel) ToResponse() AIModelResponse {
	caps := json.RawMessage(m.Capabilities)
	if len(caps) == 0 {
		caps = json.RawMessage("[]")
	}
	resp := AIModelResponse{
		ID:              m.ID,
		ModelID:         m.ModelID,
		DisplayName:     m.DisplayName,
		ProviderID:      m.ProviderID,
		Type:            m.Type,
		Capabilities:    caps,
		ContextWindow:   m.ContextWindow,
		MaxOutputTokens: m.MaxOutputTokens,
		InputPrice:      m.InputPrice,
		OutputPrice:     m.OutputPrice,
		IsDefault:       m.IsDefault,
		Status:          m.Status,
		CreatedAt:       m.CreatedAt,
		UpdatedAt:       m.UpdatedAt,
	}
	if m.Provider != nil {
		resp.ProviderName = m.Provider.Name
	}
	return resp
}

// --- AILog ---

type AILog struct {
	ID           uint      `json:"id" gorm:"primaryKey"`
	ModelID      string    `json:"modelId" gorm:"size:128;index"`
	ProviderID   uint      `json:"providerId" gorm:"index"`
	UserID       uint      `json:"userId" gorm:"index"`
	AppSource    string    `json:"appSource" gorm:"size:64"`
	InputTokens  int       `json:"inputTokens"`
	OutputTokens int       `json:"outputTokens"`
	TotalCost    float64   `json:"totalCost"`
	LatencyMs    int       `json:"latencyMs"`
	Status       string    `json:"status" gorm:"size:16;not null"`
	ErrorMessage string    `json:"errorMessage" gorm:"type:text"`
	CreatedAt    time.Time `json:"createdAt" gorm:"index"`
}

func (AILog) TableName() string { return "ai_logs" }

// --- Anthropic preset models ---

type PresetModel struct {
	ModelID         string
	DisplayName     string
	Type            string
	Capabilities    []string
	ContextWindow   int
	MaxOutputTokens int
}

var AnthropicPresetModels = []PresetModel{
	{ModelID: "claude-opus-4-20250514", DisplayName: "Claude Opus 4", Type: ModelTypeLLM, Capabilities: []string{"vision", "tool_use", "reasoning", "coding", "long_context"}, ContextWindow: 200000, MaxOutputTokens: 32000},
	{ModelID: "claude-sonnet-4-20250514", DisplayName: "Claude Sonnet 4", Type: ModelTypeLLM, Capabilities: []string{"vision", "tool_use", "reasoning", "coding", "long_context"}, ContextWindow: 200000, MaxOutputTokens: 64000},
	{ModelID: "claude-haiku-3-5-20241022", DisplayName: "Claude 3.5 Haiku", Type: ModelTypeLLM, Capabilities: []string{"vision", "tool_use", "coding"}, ContextWindow: 200000, MaxOutputTokens: 8192},
}

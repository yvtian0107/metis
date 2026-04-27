package definition

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	aiapp "metis/internal/app/ai/runtime"
	"metis/internal/app/itsm/tools"
	"metis/internal/llm"
)

type ServiceMatchConfigProvider interface {
	LLMRuntimeConfig(toolName string) (aiapp.LLMToolRuntimeConfig, error)
}

type LLMClientFactory func(protocol, baseURL, apiKey string) (llm.Client, error)

type LLMServiceMatcher struct {
	db            *gorm.DB
	config        ServiceMatchConfigProvider
	clientFactory LLMClientFactory
}

func NewLLMServiceMatcher(db *gorm.DB, config ServiceMatchConfigProvider, clientFactory LLMClientFactory) *LLMServiceMatcher {
	if clientFactory == nil {
		clientFactory = llm.NewClient
	}
	return &LLMServiceMatcher{
		db:            db,
		config:        config,
		clientFactory: clientFactory,
	}
}

type serviceMatchCandidate struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	CatalogPath string `json:"catalog_path"`
	Description string `json:"description"`
}

const serviceMatchSystemPrompt = `你是 ITSM 服务匹配判定器。服务目录是唯一事实集合，你只能从候选服务中选择，不能创造服务，不能用泛词凑匹配。

你的任务是根据用户原始诉求做结构化 Function Call：
1. 用户明确指向某个服务时，调用 select_service。
2. 只有多个服务在业务语义上都真实可能成立时，调用 need_clarification。
3. 没有可办理服务时，调用 no_match。

不要输出自然语言。不要因为“申请”“开通”“权限”等泛词匹配多个服务。`

func (m *LLMServiceMatcher) MatchServices(ctx context.Context, query string) ([]tools.ServiceMatch, tools.MatchDecision, error) {
	if strings.TrimSpace(query) == "" {
		return nil, tools.MatchDecision{}, fmt.Errorf("query is required")
	}
	if m.db == nil {
		return nil, tools.MatchDecision{}, fmt.Errorf("service matcher database is not configured")
	}
	if m.config == nil {
		return nil, tools.MatchDecision{}, fmt.Errorf("service matcher config provider is not configured")
	}
	engineCfg, err := m.config.LLMRuntimeConfig("itsm.service_match")
	if err != nil {
		return nil, tools.MatchDecision{}, fmt.Errorf("load service match tool runtime: %w", err)
	}

	candidates, err := m.loadCandidates()
	if err != nil {
		return nil, tools.MatchDecision{}, err
	}
	if len(candidates) == 0 {
		return nil, tools.MatchDecision{Kind: tools.MatchDecisionNoMatch}, nil
	}

	client, err := m.clientFactory(engineCfg.Protocol, engineCfg.BaseURL, engineCfg.APIKey)
	if err != nil {
		return nil, tools.MatchDecision{}, fmt.Errorf("create service match llm client: %w", err)
	}

	userPayload, _ := json.Marshal(map[string]any{
		"query":    query,
		"services": candidates,
	})

	var tempPtr *float32
	if engineCfg.Temperature != 0 {
		temp := float32(engineCfg.Temperature)
		tempPtr = &temp
	}
	maxTokens := engineCfg.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 1024
	}
	timeoutSeconds := engineCfg.TimeoutSeconds
	if timeoutSeconds <= 0 {
		timeoutSeconds = 30
	}
	callCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSeconds)*time.Second)
	defer cancel()

	resp, err := client.Chat(callCtx, llm.ChatRequest{
		Model: engineCfg.Model,
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: serviceMatchSystemPrompt},
			{Role: llm.RoleUser, Content: string(userPayload)},
		},
		Tools:       serviceMatchToolDefs(),
		MaxTokens:   maxTokens,
		Temperature: tempPtr,
	})
	if err != nil {
		return nil, tools.MatchDecision{}, fmt.Errorf("service match llm chat: %w", err)
	}

	return m.parseDecision(resp, candidates)
}

func (m *LLMServiceMatcher) loadCandidates() ([]serviceMatchCandidate, error) {
	type row struct {
		ID          uint
		Name        string
		Description string
		CatalogID   uint
	}
	var rows []row
	if err := m.db.Table("itsm_service_definitions").
		Where("is_active = ? AND deleted_at IS NULL", true).
		Select("id, name, description, catalog_id").
		Order("sort_order ASC, id ASC").
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("query services: %w", err)
	}

	candidates := make([]serviceMatchCandidate, 0, len(rows))
	for _, r := range rows {
		candidates = append(candidates, serviceMatchCandidate{
			ID:          r.ID,
			Name:        r.Name,
			CatalogPath: m.buildCatalogPath(r.CatalogID),
			Description: truncateText(r.Description, 240),
		})
	}
	return candidates, nil
}

func (m *LLMServiceMatcher) parseDecision(resp *llm.ChatResponse, candidates []serviceMatchCandidate) ([]tools.ServiceMatch, tools.MatchDecision, error) {
	if resp == nil || len(resp.ToolCalls) != 1 {
		return nil, tools.MatchDecision{}, fmt.Errorf("service match llm must return exactly one tool call")
	}

	byID := make(map[uint]serviceMatchCandidate, len(candidates))
	for _, c := range candidates {
		byID[c.ID] = c
	}

	tc := resp.ToolCalls[0]
	switch tc.Name {
	case "select_service":
		var args struct {
			ServiceID  uint    `json:"service_id"`
			Confidence float64 `json:"confidence"`
			Reason     string  `json:"reason"`
		}
		if err := json.Unmarshal([]byte(tc.Arguments), &args); err != nil {
			return nil, tools.MatchDecision{}, fmt.Errorf("parse select_service args: %w", err)
		}
		c, ok := byID[args.ServiceID]
		if !ok {
			return nil, tools.MatchDecision{}, fmt.Errorf("select_service returned unknown service_id: %d", args.ServiceID)
		}
		return []tools.ServiceMatch{candidateToMatch(c, args.Confidence, args.Reason)},
			tools.MatchDecision{Kind: tools.MatchDecisionSelectService, SelectedServiceID: args.ServiceID}, nil
	case "need_clarification":
		var args struct {
			ServiceIDs []uint `json:"service_ids"`
			Question   string `json:"question"`
		}
		if err := json.Unmarshal([]byte(tc.Arguments), &args); err != nil {
			return nil, tools.MatchDecision{}, fmt.Errorf("parse need_clarification args: %w", err)
		}
		if len(args.ServiceIDs) == 0 {
			return nil, tools.MatchDecision{}, fmt.Errorf("need_clarification requires at least one service_id")
		}
		matches := make([]tools.ServiceMatch, 0, len(args.ServiceIDs))
		for _, id := range args.ServiceIDs {
			c, ok := byID[id]
			if !ok {
				return nil, tools.MatchDecision{}, fmt.Errorf("need_clarification returned unknown service_id: %d", id)
			}
			matches = append(matches, candidateToMatch(c, 0, "需要用户澄清"))
		}
		return matches,
			tools.MatchDecision{Kind: tools.MatchDecisionNeedClarification, ClarificationQuestion: args.Question}, nil
	case "no_match":
		return nil, tools.MatchDecision{Kind: tools.MatchDecisionNoMatch}, nil
	default:
		return nil, tools.MatchDecision{}, fmt.Errorf("unknown service match tool call: %s", tc.Name)
	}
}

func candidateToMatch(c serviceMatchCandidate, confidence float64, reason string) tools.ServiceMatch {
	if reason == "" {
		reason = "LLM 结构化语义匹配"
	}
	return tools.ServiceMatch{
		ID:          c.ID,
		Name:        c.Name,
		CatalogPath: c.CatalogPath,
		Description: c.Description,
		Score:       confidence,
		Reason:      reason,
	}
}

func serviceMatchToolDefs() []llm.ToolDef {
	return []llm.ToolDef{
		{
			Name:        "select_service",
			Description: "用户诉求明确命中一个 ITSM 服务时调用。",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"service_id": map[string]any{"type": "integer", "description": "候选服务中的真实 id"},
					"confidence": map[string]any{"type": "number", "minimum": 0, "maximum": 1},
					"reason":     map[string]any{"type": "string"},
				},
				"required": []string{"service_id", "confidence", "reason"},
			},
		},
		{
			Name:        "need_clarification",
			Description: "多个服务在业务语义上都真实可能匹配，需要用户选择时调用。",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"service_ids": map[string]any{"type": "array", "items": map[string]any{"type": "integer"}},
					"question":    map[string]any{"type": "string"},
				},
				"required": []string{"service_ids", "question"},
			},
		},
		{
			Name:        "no_match",
			Description: "服务目录中没有可办理服务时调用。",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"reason": map[string]any{"type": "string"},
				},
				"required": []string{"reason"},
			},
		},
	}
}

func (m *LLMServiceMatcher) buildCatalogPath(catalogID uint) string {
	var parts []string
	currentID := catalogID
	for i := 0; i < 5; i++ {
		type cat struct {
			Name     string
			ParentID *uint
		}
		var c cat
		if err := m.db.Table("itsm_service_catalogs").
			Where("id = ?", currentID).
			Select("name, parent_id").First(&c).Error; err != nil {
			break
		}
		parts = append([]string{c.Name}, parts...)
		if c.ParentID == nil || *c.ParentID == 0 {
			break
		}
		currentID = *c.ParentID
	}
	return strings.Join(parts, "/")
}

func truncateText(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "..."
}

package desk

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/samber/do/v2"

	"metis/internal/app/itsm/config"
	"metis/internal/llm"
)

type titleConfigProvider interface {
	IntakeAgentID() uint
	SessionTitleRuntimeConfig() (config.LLMEngineRuntimeConfig, error)
}

type titleLLMClientFactory func(protocol, baseURL, apiKey string) (llm.Client, error)

type SessionTitleService struct {
	configSvc        titleConfigProvider
	llmClientFactory titleLLMClientFactory
}

func NewSessionTitleService(i do.Injector) (*SessionTitleService, error) {
	return &SessionTitleService{
		configSvc:        do.MustInvoke[*config.EngineConfigService](i),
		llmClientFactory: llm.NewClient,
	}, nil
}

func (s *SessionTitleService) Generate(ctx context.Context, _ uint, _ uint, agentID uint, firstUserMessage string) (string, bool, error) {
	if s.configSvc == nil {
		return "", false, nil
	}
	if agentID == 0 || agentID != s.configSvc.IntakeAgentID() {
		return "", false, nil
	}

	runtimeCfg, err := s.configSvc.SessionTitleRuntimeConfig()
	if err != nil {
		return "", true, err
	}
	clientFactory := s.llmClientFactory
	if clientFactory == nil {
		clientFactory = llm.NewClient
	}
	client, err := clientFactory(runtimeCfg.Protocol, runtimeCfg.BaseURL, runtimeCfg.APIKey)
	if err != nil {
		return "", true, err
	}

	timeoutSec := runtimeCfg.TimeoutSeconds
	if timeoutSec <= 0 {
		timeoutSec = 30
	}
	callCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
	defer cancel()

	temp := float32(runtimeCfg.Temperature)
	resp, err := client.Chat(callCtx, llm.ChatRequest{
		Model: runtimeCfg.Model,
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: runtimeCfg.SystemPrompt},
			{Role: llm.RoleUser, Content: fmt.Sprintf("用户首条问题：%s\n\n请输出会话标题。", firstUserMessage)},
		},
		Temperature: &temp,
		MaxTokens:   runtimeCfg.MaxTokens,
	})
	if err != nil {
		return "", true, err
	}
	title := normalizeSessionTitle(resp.Content)
	if title == "" {
		return "", true, fmt.Errorf("标题生成结果为空")
	}
	if len(title) > 100 {
		title = title[:100] + "..."
	}
	return title, true, nil
}

func normalizeSessionTitle(raw string) string {
	title := strings.TrimSpace(raw)
	title = strings.Trim(title, "`\"'“”‘’")
	title = strings.ReplaceAll(title, "\n", " ")
	title = strings.Join(strings.Fields(title), " ")
	return strings.TrimSpace(title)
}

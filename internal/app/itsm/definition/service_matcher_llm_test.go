package definition

import (
	"context"
	. "metis/internal/app/itsm/config"
	. "metis/internal/app/itsm/domain"
	"strconv"
	"strings"
	"testing"

	"gorm.io/gorm"

	aiapp "metis/internal/app/ai/runtime"
	"metis/internal/app/itsm/tools"
	"metis/internal/llm"
)

type fakeServiceMatchConfigProvider struct {
	cfg aiapp.LLMToolRuntimeConfig
	err error
}

func (f fakeServiceMatchConfigProvider) LLMRuntimeConfig(toolName string) (aiapp.LLMToolRuntimeConfig, error) {
	if f.err != nil {
		return aiapp.LLMToolRuntimeConfig{}, f.err
	}
	return f.cfg, nil
}

type fakeServiceMatchLLMClient struct {
	resp *llm.ChatResponse
	err  error
	req  llm.ChatRequest
}

func (f *fakeServiceMatchLLMClient) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	f.req = req
	return f.resp, f.err
}

func (f *fakeServiceMatchLLMClient) ChatStream(ctx context.Context, req llm.ChatRequest) (<-chan llm.StreamEvent, error) {
	return nil, llm.ErrNotSupported
}

func (f *fakeServiceMatchLLMClient) Embedding(ctx context.Context, req llm.EmbeddingRequest) (*llm.EmbeddingResponse, error) {
	return nil, llm.ErrNotSupported
}

func seedLLMMatcherCatalogAndServices(t *testing.T, db *gorm.DB) (vpn ServiceDefinition, copilot ServiceDefinition) {
	t.Helper()
	root := ServiceCatalog{Name: "基础设施与网络", Code: "infra-network", IsActive: true}
	if err := db.Create(&root).Error; err != nil {
		t.Fatalf("create root catalog: %v", err)
	}
	child := ServiceCatalog{Name: "网络与 VPN", Code: "infra-network:network", ParentID: &root.ID, IsActive: true}
	if err := db.Create(&child).Error; err != nil {
		t.Fatalf("create child catalog: %v", err)
	}
	account := ServiceCatalog{Name: "账号与权限", Code: "account-access", IsActive: true}
	if err := db.Create(&account).Error; err != nil {
		t.Fatalf("create account catalog: %v", err)
	}
	vpn = ServiceDefinition{Name: "VPN 开通申请", Code: "vpn-access-request", CatalogID: child.ID, Description: "用于办理 VPN 开通。", IsActive: true}
	if err := db.Create(&vpn).Error; err != nil {
		t.Fatalf("create vpn service: %v", err)
	}
	copilot = ServiceDefinition{Name: "Copilot 账号申请", Code: "copilot-account-request", CatalogID: account.ID, Description: "用于办理 Copilot 账号。", IsActive: true}
	if err := db.Create(&copilot).Error; err != nil {
		t.Fatalf("create copilot service: %v", err)
	}
	return vpn, copilot
}

func newTestLLMServiceMatcher(t *testing.T, client *fakeServiceMatchLLMClient, db *gorm.DB) *LLMServiceMatcher {
	t.Helper()
	return NewLLMServiceMatcher(
		db,
		fakeServiceMatchConfigProvider{cfg: aiapp.LLMToolRuntimeConfig{Model: "test-model", Protocol: llm.ProtocolOpenAI, APIKey: "key", Temperature: 0.2, MaxTokens: 128, TimeoutSeconds: 30}},
		func(protocol, baseURL, apiKey string) (llm.Client, error) {
			return client, nil
		},
	)
}

func TestLLMServiceMatcher_SelectServiceUsesOnlyChosenService(t *testing.T) {
	db := newTestDB(t)
	vpn, _ := seedLLMMatcherCatalogAndServices(t, db)
	client := &fakeServiceMatchLLMClient{resp: &llm.ChatResponse{
		ToolCalls: []llm.ToolCall{{Name: "select_service", Arguments: `{"service_id":` + strconv.FormatUint(uint64(vpn.ID), 10) + `,"confidence":0.98,"reason":"用户明确要申请 VPN"}`}},
	}}
	matcher := newTestLLMServiceMatcher(t, client, db)

	matches, decision, err := matcher.MatchServices(context.Background(), "我要申请VPN")
	if err != nil {
		t.Fatalf("match services: %v", err)
	}
	if decision.Kind != tools.MatchDecisionSelectService || decision.SelectedServiceID != vpn.ID {
		t.Fatalf("unexpected decision: %+v", decision)
	}
	if len(matches) != 1 || matches[0].ID != vpn.ID || matches[0].Name != "VPN 开通申请" {
		t.Fatalf("expected only VPN service, got %+v", matches)
	}
	if len(client.req.Tools) != 3 {
		t.Fatalf("expected exactly three function-call tools, got %+v", client.req.Tools)
	}
}

func TestLLMServiceMatcher_NeedClarificationReturnsValidatedCandidates(t *testing.T) {
	db := newTestDB(t)
	vpn, copilot := seedLLMMatcherCatalogAndServices(t, db)
	client := &fakeServiceMatchLLMClient{resp: &llm.ChatResponse{
		ToolCalls: []llm.ToolCall{{Name: "need_clarification", Arguments: `{"service_ids":[` + strconv.FormatUint(uint64(vpn.ID), 10) + `,` + strconv.FormatUint(uint64(copilot.ID), 10) + `],"question":"请选择要办理 VPN 还是 Copilot 账号"}`}},
	}}
	matcher := newTestLLMServiceMatcher(t, client, db)

	matches, decision, err := matcher.MatchServices(context.Background(), "我要申请账号权限")
	if err != nil {
		t.Fatalf("match services: %v", err)
	}
	if decision.Kind != tools.MatchDecisionNeedClarification || decision.ClarificationQuestion == "" {
		t.Fatalf("unexpected decision: %+v", decision)
	}
	if len(matches) != 2 || matches[0].ID != vpn.ID || matches[1].ID != copilot.ID {
		t.Fatalf("expected validated candidates in model order, got %+v", matches)
	}
}

func TestLLMServiceMatcher_NoMatchReturnsEmptyMatches(t *testing.T) {
	db := newTestDB(t)
	seedLLMMatcherCatalogAndServices(t, db)
	client := &fakeServiceMatchLLMClient{resp: &llm.ChatResponse{
		ToolCalls: []llm.ToolCall{{Name: "no_match", Arguments: `{"reason":"服务目录中没有咖啡申请"}`}},
	}}
	matcher := newTestLLMServiceMatcher(t, client, db)

	matches, decision, err := matcher.MatchServices(context.Background(), "我要领一杯咖啡")
	if err != nil {
		t.Fatalf("match services: %v", err)
	}
	if decision.Kind != tools.MatchDecisionNoMatch || len(matches) != 0 {
		t.Fatalf("expected no match, got decision=%+v matches=%+v", decision, matches)
	}
}

func TestLLMServiceMatcher_ExcludesPublishHealthFailuresFromCandidates(t *testing.T) {
	db := newTestDB(t)
	vpn, copilot := seedLLMMatcherCatalogAndServices(t, db)
	if err := db.Model(&ServiceDefinition{}).Where("id = ?", vpn.ID).Update("publish_health_status", "fail").Error; err != nil {
		t.Fatalf("mark vpn unhealthy: %v", err)
	}
	client := &fakeServiceMatchLLMClient{resp: &llm.ChatResponse{
		ToolCalls: []llm.ToolCall{{Name: "select_service", Arguments: `{"service_id":` + strconv.FormatUint(uint64(copilot.ID), 10) + `,"confidence":0.92,"reason":"only healthy candidate remains"}`}},
	}}
	matcher := newTestLLMServiceMatcher(t, client, db)

	matches, decision, err := matcher.MatchServices(context.Background(), "我要申请 Copilot 账号")
	if err != nil {
		t.Fatalf("match services: %v", err)
	}
	if decision.SelectedServiceID != copilot.ID {
		t.Fatalf("expected healthy service to be selected, got %+v", decision)
	}
	if len(matches) != 1 || matches[0].ID != copilot.ID {
		t.Fatalf("expected only healthy service match, got %+v", matches)
	}
	if strings.Contains(client.req.Messages[1].Content, "VPN 开通申请") {
		t.Fatalf("expected unhealthy service to be excluded from model candidates, got %q", client.req.Messages[1].Content)
	}
}

func TestLLMServiceMatcher_RejectsInvalidModelOutput(t *testing.T) {
	tests := []struct {
		name string
		resp *llm.ChatResponse
	}{
		{name: "no tool call", resp: &llm.ChatResponse{}},
		{name: "unknown tool", resp: &llm.ChatResponse{ToolCalls: []llm.ToolCall{{Name: "maybe_service", Arguments: `{}`}}}},
		{name: "unknown service id", resp: &llm.ChatResponse{ToolCalls: []llm.ToolCall{{Name: "select_service", Arguments: `{"service_id":999,"confidence":0.8,"reason":"bad id"}`}}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := newTestDB(t)
			seedLLMMatcherCatalogAndServices(t, db)
			matcher := newTestLLMServiceMatcher(t, &fakeServiceMatchLLMClient{resp: tt.resp}, db)

			if _, _, err := matcher.MatchServices(context.Background(), "我要申请VPN"); err == nil {
				t.Fatal("expected invalid model output to fail")
			}
		})
	}
}

func TestLLMServiceMatcher_RequiresConfiguredServiceMatcherEngine(t *testing.T) {
	db := newTestDB(t)
	seedLLMMatcherCatalogAndServices(t, db)
	matcher := NewLLMServiceMatcher(
		db,
		fakeServiceMatchConfigProvider{err: ErrModelNotFound},
		func(protocol, baseURL, apiKey string) (llm.Client, error) {
			t.Fatal("client factory should not be called without configured engine")
			return nil, nil
		},
	)

	if _, _, err := matcher.MatchServices(context.Background(), "我要申请VPN"); err == nil {
		t.Fatal("expected missing service matcher engine configuration to fail")
	}
}

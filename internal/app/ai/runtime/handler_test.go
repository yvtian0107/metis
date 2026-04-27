package runtime

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"metis/internal/llm"
)

func setupAIHandlerTestRouter(agentH *AgentHandler, sessionH *SessionHandler, memoryH *MemoryHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		userID := uint(1)
		if c.GetHeader("X-Test-User") == "2" {
			userID = 2
		}
		c.Set("userId", userID)
		c.Next()
	})
	r.GET("/api/v1/ai/agents/:id", agentH.Get)
	r.GET("/api/v1/ai/sessions/:sid", sessionH.Get)
	r.POST("/api/v1/ai/sessions/:sid/chat", sessionH.Chat)
	r.GET("/api/v1/ai/sessions/:sid/stream", sessionH.Stream)
	r.PUT("/api/v1/ai/sessions/:sid/messages/:mid", sessionH.EditMessage)
	r.DELETE("/api/v1/ai/agents/:id/memories/:mid", memoryH.Delete)
	return r
}

func TestAgentHandler_Get_HiddenPrivateAgentReturnsNotFound(t *testing.T) {
	db := setupTestDB(t)
	agentSvc := newAgentServiceForTest(t, db)
	modelID := uint(1)
	agent := &Agent{Name: "private-agent", Type: AgentTypeAssistant, ModelID: &modelID, Visibility: AgentVisibilityPrivate, CreatedBy: 1}
	if err := agentSvc.Create(agent); err != nil {
		t.Fatalf("create agent: %v", err)
	}

	r := setupAIHandlerTestRouter(
		&AgentHandler{svc: agentSvc, repo: agentSvc.repo},
		&SessionHandler{svc: newSessionServiceForTest(t, db), gateway: newGatewayForTest(t, db, nil)},
		&MemoryHandler{svc: &MemoryService{repo: &MemoryRepo{db: agentSvc.repo.db}}},
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/ai/agents/"+strconv.FormatUint(uint64(agent.ID), 10), nil)
	req.Header.Set("X-Test-User", "2")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSessionHandler_CrossUserAccessReturnsNotFound(t *testing.T) {
	db := setupTestDB(t)
	agentSvc := newAgentServiceForTest(t, db)
	sessionSvc := newSessionServiceForTest(t, db)
	modelID := uint(1)
	agent := &Agent{Name: "agent", Type: AgentTypeAssistant, ModelID: &modelID, Visibility: AgentVisibilityTeam, CreatedBy: 1}
	if err := agentSvc.Create(agent); err != nil {
		t.Fatalf("create agent: %v", err)
	}
	session, err := sessionSvc.Create(agent.ID, 1)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	msg, err := sessionSvc.StoreMessage(session.ID, MessageRoleUser, "hello", nil, 0)
	if err != nil {
		t.Fatalf("store message: %v", err)
	}

	r := setupAIHandlerTestRouter(
		&AgentHandler{svc: agentSvc, repo: agentSvc.repo},
		&SessionHandler{svc: sessionSvc, gateway: newGatewayForTest(t, db, nil)},
		&MemoryHandler{svc: &MemoryService{repo: &MemoryRepo{db: agentSvc.repo.db}}},
	)

	tests := []struct {
		name   string
		method string
		path   string
		body   []byte
	}{
		{name: "detail", method: http.MethodGet, path: "/api/v1/ai/sessions/" + strconv.FormatUint(uint64(session.ID), 10)},
		{name: "chat", method: http.MethodPost, path: "/api/v1/ai/sessions/" + strconv.FormatUint(uint64(session.ID), 10) + "/chat", body: []byte(`{"messages":[{"role":"user","parts":[{"type":"text","text":"hello"}]}]}`)},
		{name: "stream", method: http.MethodGet, path: "/api/v1/ai/sessions/" + strconv.FormatUint(uint64(session.ID), 10) + "/stream"},
		{name: "edit", method: http.MethodPut, path: "/api/v1/ai/sessions/" + strconv.FormatUint(uint64(session.ID), 10) + "/messages/" + strconv.FormatUint(uint64(msg.ID), 10), body: []byte(`{"content":"updated"}`)},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, bytes.NewReader(tc.body))
			req.Header.Set("X-Test-User", "2")
			if tc.body != nil {
				req.Header.Set("Content-Type", "application/json")
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != http.StatusNotFound {
				t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestSessionHandler_ChatStoresMessageAndStreams(t *testing.T) {
	db := setupTestDB(t)
	agentSvc := newAgentServiceForTest(t, db)
	sessionSvc := newSessionServiceForTest(t, db)
	mockLLM := newMockLLMClient([]llm.StreamEvent{
		{Type: "content_delta", Content: "你好"},
		{Type: "content_delta", Content: "！"},
		{Type: "done", Usage: &llm.Usage{InputTokens: 3, OutputTokens: 2}},
	}, nil)
	modelID := uint(1)
	agent := &Agent{Name: "agent", Type: AgentTypeAssistant, ModelID: &modelID, Strategy: AgentStrategyReact, Visibility: AgentVisibilityTeam, CreatedBy: 1}
	if err := agentSvc.Create(agent); err != nil {
		t.Fatalf("create agent: %v", err)
	}
	session, err := sessionSvc.Create(agent.ID, 1)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	r := setupAIHandlerTestRouter(
		&AgentHandler{svc: agentSvc, repo: agentSvc.repo},
		&SessionHandler{svc: sessionSvc, gateway: newGatewayForTest(t, db, mockLLM)},
		&MemoryHandler{svc: &MemoryService{repo: &MemoryRepo{db: agentSvc.repo.db}}},
	)

	body := []byte(`{"id":"service-desk","messages":[{"id":"u1","role":"user","parts":[{"type":"text","text":"我想申请 VPN"},{"type":"file","mediaType":"image/png","url":"data:image/png;base64,abc"}]}],"trigger":"submit-message"}`)
	server := httptest.NewServer(r)
	defer server.Close()

	req, err := http.NewRequest(http.MethodPost, server.URL+"/api/v1/ai/sessions/"+strconv.FormatUint(uint64(session.ID), 10)+"/chat", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := server.Client().Do(req)
	if err != nil {
		t.Fatalf("send chat request: %v", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read stream body: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(respBody))
	}
	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "text/event-stream") {
		t.Fatalf("expected event-stream content type, got %q", contentType)
	}
	streamBody := string(respBody)
	for _, expected := range []string{`"type":"start"`, `"type":"text-delta"`, `"delta":"你好"`, "data: [DONE]"} {
		if !strings.Contains(streamBody, expected) {
			t.Fatalf("stream missing %s: %s", expected, streamBody)
		}
	}

	messages, err := sessionSvc.GetMessages(session.ID)
	if err != nil {
		t.Fatalf("get messages: %v", err)
	}
	if len(messages) < 2 {
		t.Fatalf("expected stored user and assistant messages, got %+v", messages)
	}
	if messages[0].Role != MessageRoleUser || messages[0].Content != "我想申请 VPN" {
		t.Fatalf("unexpected user message: %+v", messages[0])
	}
	if !strings.Contains(string(messages[0].Metadata), "data:image/png;base64,abc") {
		t.Fatalf("expected uploaded image metadata, got %s", messages[0].Metadata)
	}
	if messages[len(messages)-1].Role != MessageRoleAssistant || messages[len(messages)-1].Content != "你好！" {
		t.Fatalf("unexpected assistant message: %+v", messages[len(messages)-1])
	}
}

func TestSessionHandler_ChatRegenerateDoesNotDuplicateUserMessage(t *testing.T) {
	db := setupTestDB(t)
	agentSvc := newAgentServiceForTest(t, db)
	sessionSvc := newSessionServiceForTest(t, db)
	mockLLM := newMockLLMClient([]llm.StreamEvent{
		{Type: "content_delta", Content: "重新生成"},
		{Type: "done", Usage: &llm.Usage{InputTokens: 4, OutputTokens: 2}},
	}, nil)
	modelID := uint(1)
	agent := &Agent{Name: "agent", Type: AgentTypeAssistant, ModelID: &modelID, Strategy: AgentStrategyReact, Visibility: AgentVisibilityTeam, CreatedBy: 1}
	if err := agentSvc.Create(agent); err != nil {
		t.Fatalf("create agent: %v", err)
	}
	session, err := sessionSvc.Create(agent.ID, 1)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if _, err := sessionSvc.StoreMessage(session.ID, MessageRoleUser, "我想申请 VPN", nil, 0); err != nil {
		t.Fatalf("store original user message: %v", err)
	}

	r := setupAIHandlerTestRouter(
		&AgentHandler{svc: agentSvc, repo: agentSvc.repo},
		&SessionHandler{svc: sessionSvc, gateway: newGatewayForTest(t, db, mockLLM)},
		&MemoryHandler{svc: &MemoryService{repo: &MemoryRepo{db: agentSvc.repo.db}}},
	)

	body := []byte(`{"id":"service-desk","messages":[{"id":"u1","role":"user","parts":[{"type":"text","text":"我想申请 VPN"}]}],"trigger":"regenerate-message"}`)
	server := httptest.NewServer(r)
	defer server.Close()

	req, err := http.NewRequest(http.MethodPost, server.URL+"/api/v1/ai/sessions/"+strconv.FormatUint(uint64(session.ID), 10)+"/chat", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := server.Client().Do(req)
	if err != nil {
		t.Fatalf("send chat request: %v", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read stream body: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(respBody))
	}
	if !strings.Contains(string(respBody), `"delta":"重新生成"`) {
		t.Fatalf("expected regenerated stream content, got %s", string(respBody))
	}

	messages, err := sessionSvc.GetMessages(session.ID)
	if err != nil {
		t.Fatalf("get messages: %v", err)
	}
	userCount := 0
	for _, message := range messages {
		if message.Role == MessageRoleUser {
			userCount++
		}
	}
	if userCount != 1 {
		t.Fatalf("expected one persisted user message after regenerate, got %d: %+v", userCount, messages)
	}
	if messages[len(messages)-1].Role != MessageRoleAssistant || messages[len(messages)-1].Content != "重新生成" {
		t.Fatalf("unexpected regenerated assistant message: %+v", messages[len(messages)-1])
	}
}

func TestMemoryHandler_CrossUserDeleteReturnsNotFound(t *testing.T) {
	db := setupTestDB(t)
	agentSvc := newAgentServiceForTest(t, db)
	modelID := uint(1)
	agent := &Agent{Name: "agent", Type: AgentTypeAssistant, ModelID: &modelID, Visibility: AgentVisibilityTeam, CreatedBy: 1}
	if err := agentSvc.Create(agent); err != nil {
		t.Fatalf("create agent: %v", err)
	}
	memorySvc := &MemoryService{repo: &MemoryRepo{db: agentSvc.repo.db}}
	memory := &AgentMemory{AgentID: agent.ID, UserID: 1, Key: "pref", Content: "json", Source: MemorySourceUserSet}
	if err := memorySvc.Upsert(memory); err != nil {
		t.Fatalf("upsert memory: %v", err)
	}

	r := setupAIHandlerTestRouter(
		&AgentHandler{svc: agentSvc, repo: agentSvc.repo},
		&SessionHandler{svc: newSessionServiceForTest(t, db), gateway: newGatewayForTest(t, db, nil)},
		&MemoryHandler{svc: memorySvc},
	)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/ai/agents/"+strconv.FormatUint(uint64(agent.ID), 10)+"/memories/"+strconv.FormatUint(uint64(memory.ID), 10), nil)
	req.Header.Set("X-Test-User", "2")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

package ai

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
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

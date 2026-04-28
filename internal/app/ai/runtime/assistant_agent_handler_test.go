package runtime

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestAssistantAgentHandlerUpdate_PreservesModelWhenModelIDMissing(t *testing.T) {
	db := setupTestDB(t)
	svc := newAgentServiceForTest(t, db)
	modelID := uint(42)
	agent := &Agent{
		Name:         "assistant",
		Type:         AgentTypeAssistant,
		ModelID:      &modelID,
		Strategy:     AgentStrategyReact,
		Visibility:   AgentVisibilityTeam,
		CreatedBy:    1,
		SystemPrompt: "before",
		Temperature:  0.7,
		MaxTokens:    4096,
		MaxTurns:     10,
	}
	if err := svc.Create(agent); err != nil {
		t.Fatalf("create agent: %v", err)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("userId", uint(1))
		c.Next()
	})
	h := &AssistantAgentHandler{svc: svc, repo: svc.repo}
	r.PUT("/api/v1/ai/assistant-agents/:id", h.Update)

	body := []byte(`{
		"name":"assistant updated",
		"description":"updated",
		"visibility":"team",
		"strategy":"react",
		"systemPrompt":"after",
		"temperature":0.4,
		"maxTokens":2048,
		"maxTurns":6,
		"toolIds":[],
		"skillIds":[],
		"mcpServerIds":[],
		"knowledgeBaseIds":[],
		"knowledgeGraphIds":[],
		"capabilitySetBindings":[]
	}`)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/ai/assistant-agents/"+strconv.FormatUint(uint64(agent.ID), 10), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	loaded, err := svc.Get(agent.ID)
	if err != nil {
		t.Fatalf("reload agent: %v", err)
	}
	if loaded.ModelID == nil || *loaded.ModelID != modelID {
		t.Fatalf("expected modelID to remain %d, got %v", modelID, loaded.ModelID)
	}
	if loaded.SystemPrompt != "after" {
		t.Fatalf("expected prompt to update, got %q", loaded.SystemPrompt)
	}
}

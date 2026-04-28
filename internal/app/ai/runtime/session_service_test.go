package runtime

import (
	"context"
	"errors"
	"testing"

	"metis/internal/app"
	"metis/internal/model"
)

func TestSessionService_Create(t *testing.T) {
	db := setupTestDB(t)
	agentSvc := newAgentServiceForTest(t, db)
	svc := newSessionServiceForTest(t, db)

	modelID := uint(1)
	agent := &Agent{Name: "Agent", Type: AgentTypeAssistant, ModelID: &modelID, CreatedBy: 1}
	_ = agentSvc.Create(agent)

	session, err := svc.Create(agent.ID, 1)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if session.AgentID != agent.ID {
		t.Errorf("agentID: expected %d, got %d", agent.ID, session.AgentID)
	}
	if session.Status != SessionStatusRunning {
		t.Errorf("status: expected %q, got %q", SessionStatusRunning, session.Status)
	}
}

func TestSessionService_StoreMessage_And_GetMessages(t *testing.T) {
	db := setupTestDB(t)
	agentSvc := newAgentServiceForTest(t, db)
	svc := newSessionServiceForTest(t, db)

	modelID := uint(1)
	agent := &Agent{Name: "Agent", Type: AgentTypeAssistant, ModelID: &modelID, CreatedBy: 1}
	_ = agentSvc.Create(agent)
	session, _ := svc.Create(agent.ID, 1)

	msg1, err := svc.StoreMessage(session.ID, MessageRoleUser, "Hello", nil, 5)
	if err != nil {
		t.Fatalf("store message: %v", err)
	}
	if msg1.Sequence != 1 {
		t.Errorf("sequence: expected 1, got %d", msg1.Sequence)
	}

	// Auto title generation
	sessionReloaded, _ := svc.Get(session.ID)
	if sessionReloaded.Title != "Hello" {
		t.Errorf("title: expected %q, got %q", "Hello", sessionReloaded.Title)
	}

	msg2, _ := svc.StoreMessage(session.ID, MessageRoleAssistant, "Hi there", nil, 3)
	if msg2.Sequence != 2 {
		t.Errorf("sequence: expected 2, got %d", msg2.Sequence)
	}

	messages, err := svc.GetMessages(session.ID)
	if err != nil {
		t.Fatalf("get messages: %v", err)
	}
	if len(messages) != 2 {
		t.Errorf("expected 2 messages, got %d", len(messages))
	}
}

func TestSessionService_EditMessage(t *testing.T) {
	db := setupTestDB(t)
	agentSvc := newAgentServiceForTest(t, db)
	svc := newSessionServiceForTest(t, db)

	modelID := uint(1)
	agent := &Agent{Name: "Agent", Type: AgentTypeAssistant, ModelID: &modelID, CreatedBy: 1}
	_ = agentSvc.Create(agent)
	session, _ := svc.Create(agent.ID, 1)

	_, _ = svc.StoreMessage(session.ID, MessageRoleUser, "Hello", nil, 0)
	msg2, _ := svc.StoreMessage(session.ID, MessageRoleAssistant, "Hi", nil, 0)
	_, _ = svc.StoreMessage(session.ID, MessageRoleUser, "How are you", model.JSONText([]byte(`{}`)), 0)

	edited, err := svc.EditMessage(session.ID, msg2.ID, "Hi there")
	if err != nil {
		t.Fatalf("edit message: %v", err)
	}
	if edited.Content != "Hi there" {
		t.Errorf("content: expected %q, got %q", "Hi there", edited.Content)
	}

	// msg3 should be deleted
	messages, _ := svc.GetMessages(session.ID)
	if len(messages) != 2 {
		t.Errorf("expected 2 messages after edit, got %d", len(messages))
	}
}

func TestSessionService_UpdateStatus(t *testing.T) {
	db := setupTestDB(t)
	agentSvc := newAgentServiceForTest(t, db)
	svc := newSessionServiceForTest(t, db)

	modelID := uint(1)
	agent := &Agent{Name: "Agent", Type: AgentTypeAssistant, ModelID: &modelID, CreatedBy: 1}
	_ = agentSvc.Create(agent)
	session, _ := svc.Create(agent.ID, 1)

	if err := svc.UpdateStatus(session.ID, SessionStatusCompleted); err != nil {
		t.Fatalf("update status: %v", err)
	}

	loaded, _ := svc.Get(session.ID)
	if loaded.Status != SessionStatusCompleted {
		t.Errorf("status: expected %q, got %q", SessionStatusCompleted, loaded.Status)
	}
}

func TestSessionService_Create_HiddenPrivateAgentReturnsNotFound(t *testing.T) {
	db := setupTestDB(t)
	agentSvc := newAgentServiceForTest(t, db)
	svc := newSessionServiceForTest(t, db)

	modelID := uint(1)
	agent := &Agent{Name: "Agent", Type: AgentTypeAssistant, ModelID: &modelID, CreatedBy: 1, Visibility: AgentVisibilityPrivate}
	_ = agentSvc.Create(agent)

	if _, err := svc.Create(agent.ID, 2); err != ErrAgentNotFound {
		t.Fatalf("expected ErrAgentNotFound, got %v", err)
	}
}

func TestSessionService_GetOwned_HidesCrossUserSession(t *testing.T) {
	db := setupTestDB(t)
	agentSvc := newAgentServiceForTest(t, db)
	svc := newSessionServiceForTest(t, db)

	modelID := uint(1)
	agent := &Agent{Name: "Agent", Type: AgentTypeAssistant, ModelID: &modelID, CreatedBy: 1, Visibility: AgentVisibilityTeam}
	_ = agentSvc.Create(agent)
	session, _ := svc.Create(agent.ID, 1)

	if _, err := svc.GetOwned(session.ID, 2); err != ErrSessionNotFound {
		t.Fatalf("expected ErrSessionNotFound, got %v", err)
	}
}

func TestSessionService_StoreMessage_UsesSessionTitleProvider(t *testing.T) {
	db := setupTestDB(t)
	agentSvc := newAgentServiceForTest(t, db)
	svc := newSessionServiceForTest(t, db)

	modelID := uint(1)
	agent := &Agent{Name: "Agent", Type: AgentTypeAssistant, ModelID: &modelID, CreatedBy: 1}
	_ = agentSvc.Create(agent)
	session, _ := svc.Create(agent.ID, 1)

	svc.titleProviders = []app.SessionTitleProvider{
		fakeSessionTitleProvider{
			title:   "VPN 线上支持申请",
			handled: true,
		},
	}
	if _, err := svc.StoreMessage(session.ID, MessageRoleUser, "我想申请 VPN，线上支持用", nil, 0); err != nil {
		t.Fatalf("store message: %v", err)
	}
	reloaded, _ := svc.Get(session.ID)
	if reloaded.Title != "VPN 线上支持申请" {
		t.Fatalf("expected provider title, got %q", reloaded.Title)
	}
}

func TestSessionService_StoreMessage_FallbackWhenSessionTitleProviderFails(t *testing.T) {
	db := setupTestDB(t)
	agentSvc := newAgentServiceForTest(t, db)
	svc := newSessionServiceForTest(t, db)

	modelID := uint(1)
	agent := &Agent{Name: "Agent", Type: AgentTypeAssistant, ModelID: &modelID, CreatedBy: 1}
	_ = agentSvc.Create(agent)
	session, _ := svc.Create(agent.ID, 1)

	svc.titleProviders = []app.SessionTitleProvider{
		fakeSessionTitleProvider{
			handled: true,
			err:     errors.New("upstream failed"),
		},
	}
	content := "我想申请 VPN，线上支持用"
	if _, err := svc.StoreMessage(session.ID, MessageRoleUser, content, nil, 0); err != nil {
		t.Fatalf("store message: %v", err)
	}
	reloaded, _ := svc.Get(session.ID)
	if reloaded.Title != content {
		t.Fatalf("expected fallback title %q, got %q", content, reloaded.Title)
	}
}

type fakeSessionTitleProvider struct {
	title   string
	handled bool
	err     error
}

func (f fakeSessionTitleProvider) GenerateSessionTitle(context.Context, uint, uint, uint, string) (string, bool, error) {
	return f.title, f.handled, f.err
}

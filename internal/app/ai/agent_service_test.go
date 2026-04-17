package ai

import (
	"testing"
)

func TestAgentService_Create_InvalidType(t *testing.T) {
	db := setupTestDB(t)
	svc := newAgentServiceForTest(t, db)

	a := &Agent{Name: "Test", Type: "unknown", CreatedBy: 1}
	err := svc.Create(a)
	if err != ErrInvalidAgentType {
		t.Errorf("expected %v, got %v", ErrInvalidAgentType, err)
	}
}

func TestAgentService_Create_AssistantWithoutModel(t *testing.T) {
	db := setupTestDB(t)
	svc := newAgentServiceForTest(t, db)

	a := &Agent{Name: "Assistant", Type: AgentTypeAssistant, CreatedBy: 1}
	err := svc.Create(a)
	if err != ErrModelRequired {
		t.Errorf("expected %v, got %v", ErrModelRequired, err)
	}
}

func TestAgentService_Create_CodingWithoutRuntime(t *testing.T) {
	db := setupTestDB(t)
	svc := newAgentServiceForTest(t, db)

	a := &Agent{Name: "Coder", Type: AgentTypeCoding, CreatedBy: 1}
	err := svc.Create(a)
	if err != ErrRuntimeRequired {
		t.Errorf("expected %v, got %v", ErrRuntimeRequired, err)
	}
}

func TestAgentService_Create_RemoteWithoutNode(t *testing.T) {
	db := setupTestDB(t)
	svc := newAgentServiceForTest(t, db)

	a := &Agent{Name: "Coder", Type: AgentTypeCoding, Runtime: AgentRuntimeClaudeCode, ExecMode: AgentExecModeRemote, CreatedBy: 1}
	err := svc.Create(a)
	if err != ErrNodeRequired {
		t.Errorf("expected %v, got %v", ErrNodeRequired, err)
	}
}

func TestAgentService_Create_InternalWithoutCode(t *testing.T) {
	db := setupTestDB(t)
	svc := newAgentServiceForTest(t, db)

	a := &Agent{Name: "Internal", Type: AgentTypeInternal, CreatedBy: 1}
	err := svc.Create(a)
	if err != ErrCodeRequired {
		t.Errorf("expected %v, got %v", ErrCodeRequired, err)
	}
}

func TestAgentService_Create_NameConflict(t *testing.T) {
	db := setupTestDB(t)
	svc := newAgentServiceForTest(t, db)

	modelID := uint(1)
	a1 := &Agent{Name: "Agent1", Type: AgentTypeAssistant, ModelID: &modelID, CreatedBy: 1}
	_ = svc.Create(a1)

	a2 := &Agent{Name: "Agent1", Type: AgentTypeAssistant, ModelID: &modelID, CreatedBy: 1}
	err := svc.Create(a2)
	if err != ErrAgentNameConflict {
		t.Errorf("expected %v, got %v", ErrAgentNameConflict, err)
	}
}

func TestAgentService_Create_CodeConflict(t *testing.T) {
	db := setupTestDB(t)
	svc := newAgentServiceForTest(t, db)

	code := "agent-a"
	a1 := &Agent{Name: "AgentA", Type: AgentTypeInternal, Code: &code, CreatedBy: 1}
	_ = svc.Create(a1)

	a2 := &Agent{Name: "AgentB", Type: AgentTypeInternal, Code: &code, CreatedBy: 1}
	err := svc.Create(a2)
	if err != ErrAgentCodeConflict {
		t.Errorf("expected %v, got %v", ErrAgentCodeConflict, err)
	}
}

func TestAgentService_Update(t *testing.T) {
	db := setupTestDB(t)
	svc := newAgentServiceForTest(t, db)

	modelID := uint(1)
	a := &Agent{Name: "Agent", Type: AgentTypeAssistant, ModelID: &modelID, CreatedBy: 1}
	_ = svc.Create(a)

	a.Description = "Updated desc"
	a.Temperature = 0.5
	if err := svc.Update(a); err != nil {
		t.Fatalf("update agent: %v", err)
	}

	loaded, err := svc.Get(a.ID)
	if err != nil {
		t.Fatalf("get agent: %v", err)
	}
	if loaded.Description != "Updated desc" {
		t.Errorf("description: expected %q, got %q", "Updated desc", loaded.Description)
	}
	if loaded.Temperature != 0.5 {
		t.Errorf("temperature: expected %f, got %f", 0.5, loaded.Temperature)
	}
}

func TestAgentService_Delete_WithRunningSessions(t *testing.T) {
	db := setupTestDB(t)
	svc := newAgentServiceForTest(t, db)
	repo := svc.repo

	modelID := uint(1)
	a := &Agent{Name: "Agent", Type: AgentTypeAssistant, ModelID: &modelID, CreatedBy: 1}
	_ = svc.Create(a)

	// Create running session
	session := &AgentSession{AgentID: a.ID, UserID: 1, Status: SessionStatusRunning}
	_ = repo.db.Create(session).Error

	err := svc.Delete(a.ID)
	if err != ErrAgentHasRunningSessions {
		t.Errorf("expected %v, got %v", ErrAgentHasRunningSessions, err)
	}
}

func TestAgentService_Delete_Success(t *testing.T) {
	db := setupTestDB(t)
	svc := newAgentServiceForTest(t, db)

	modelID := uint(1)
	a := &Agent{Name: "Agent", Type: AgentTypeAssistant, ModelID: &modelID, CreatedBy: 1}
	_ = svc.Create(a)

	if err := svc.Delete(a.ID); err != nil {
		t.Fatalf("delete agent: %v", err)
	}

	_, err := svc.Get(a.ID)
	if err == nil {
		t.Error("expected agent to be deleted")
	}
}

func TestAgentService_UpdateBindings_And_GetBindings(t *testing.T) {
	db := setupTestDB(t)
	svc := newAgentServiceForTest(t, db)

	modelID := uint(1)
	a := &Agent{Name: "Agent", Type: AgentTypeAssistant, ModelID: &modelID, CreatedBy: 1}
	_ = svc.Create(a)

	err := svc.UpdateBindings(a.ID, []uint{1, 2}, []uint{3}, []uint{4}, []uint{5})
	if err != nil {
		t.Fatalf("update bindings: %v", err)
	}

	toolIDs, skillIDs, mcpIDs, kbIDs, err := svc.GetBindings(a.ID)
	if err != nil {
		t.Fatalf("get bindings: %v", err)
	}

	if len(toolIDs) != 2 || toolIDs[0] != 1 || toolIDs[1] != 2 {
		t.Errorf("toolIDs: expected [1 2], got %v", toolIDs)
	}
	if len(skillIDs) != 1 || skillIDs[0] != 3 {
		t.Errorf("skillIDs: expected [3], got %v", skillIDs)
	}
	if len(mcpIDs) != 1 || mcpIDs[0] != 4 {
		t.Errorf("mcpIDs: expected [4], got %v", mcpIDs)
	}
	if len(kbIDs) != 1 || kbIDs[0] != 5 {
		t.Errorf("kbIDs: expected [5], got %v", kbIDs)
	}

	// Replace with empty
	_ = svc.UpdateBindings(a.ID, []uint{}, []uint{}, []uint{}, []uint{})
	toolIDs, _, _, _, _ = svc.GetBindings(a.ID)
	if len(toolIDs) != 0 {
		t.Errorf("toolIDs after clear: expected [], got %v", toolIDs)
	}
}

func TestAgentService_ListTemplates(t *testing.T) {
	db := setupTestDB(t)
	svc := newAgentServiceForTest(t, db)
	repo := svc.repo

	_ = repo.db.Create(&AgentTemplate{Name: "T1", Type: AgentTypeAssistant}).Error
	_ = repo.db.Create(&AgentTemplate{Name: "T2", Type: AgentTypeCoding}).Error

	templates, err := svc.ListTemplates()
	if err != nil {
		t.Fatalf("list templates: %v", err)
	}
	if len(templates) != 2 {
		t.Errorf("expected 2 templates, got %d", len(templates))
	}
}

package runtime

import (
	"errors"
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

func TestAgentService_GetAccessible_HidesPrivateAgentFromOthers(t *testing.T) {
	db := setupTestDB(t)
	svc := newAgentServiceForTest(t, db)

	modelID := uint(1)
	a := &Agent{Name: "Agent", Type: AgentTypeAssistant, ModelID: &modelID, CreatedBy: 1, Visibility: AgentVisibilityPrivate}
	_ = svc.Create(a)

	if _, err := svc.GetAccessible(a.ID, 2); err != ErrAgentNotFound {
		t.Fatalf("expected ErrAgentNotFound, got %v", err)
	}
	if _, err := svc.GetOwned(a.ID, 2); err != ErrAgentNotFound {
		t.Fatalf("expected ErrAgentNotFound, got %v", err)
	}
	if _, err := svc.GetAccessible(a.ID, 1); err != nil {
		t.Fatalf("creator should access private agent: %v", err)
	}
}

func TestAgentService_UpdateBindings_And_GetBindings(t *testing.T) {
	db := setupTestDB(t)
	svc := newAgentServiceForTest(t, db)
	toolIDs, skillIDs, mcpIDs, kbIDs, kgIDs := seedAgentBindingTargets(t, db)

	modelID := uint(1)
	a := &Agent{Name: "Agent", Type: AgentTypeAssistant, ModelID: &modelID, CreatedBy: 1}
	_ = svc.Create(a)

	err := svc.UpdateBindings(a.ID, []uint{toolIDs[0], toolIDs[1], toolIDs[0]}, skillIDs[:1], mcpIDs[:1], kbIDs[:1], kgIDs[:1])
	if err != nil {
		t.Fatalf("update bindings: %v", err)
	}

	toolIDs, skillIDs, mcpIDs, kbIDs, kgIDs, err = svc.GetBindings(a.ID)
	if err != nil {
		t.Fatalf("get bindings: %v", err)
	}

	if len(toolIDs) != 2 {
		t.Errorf("toolIDs: expected 2 unique IDs, got %v", toolIDs)
	}
	if len(skillIDs) != 1 {
		t.Errorf("skillIDs: expected 1 ID, got %v", skillIDs)
	}
	if len(mcpIDs) != 1 {
		t.Errorf("mcpIDs: expected 1 ID, got %v", mcpIDs)
	}
	if len(kbIDs) != 1 {
		t.Errorf("kbIDs: expected 1 ID, got %v", kbIDs)
	}
	if len(kgIDs) != 1 {
		t.Errorf("kgIDs: expected 1 ID, got %v", kgIDs)
	}

	// Replace with empty
	_ = svc.UpdateBindings(a.ID, []uint{}, []uint{}, []uint{}, []uint{}, []uint{})
	toolIDs, _, _, _, _, _ = svc.GetBindings(a.ID)
	if len(toolIDs) != 0 {
		t.Errorf("toolIDs after clear: expected [], got %v", toolIDs)
	}
}

func TestAgentService_UpdateBindings_RejectsInvalidIDAtomically(t *testing.T) {
	db := setupTestDB(t)
	svc := newAgentServiceForTest(t, db)
	toolIDs, _, _, _, _ := seedAgentBindingTargets(t, db)

	modelID := uint(1)
	a := &Agent{Name: "Agent", Type: AgentTypeAssistant, ModelID: &modelID, CreatedBy: 1}
	if err := svc.Create(a); err != nil {
		t.Fatalf("create agent: %v", err)
	}
	if err := svc.UpdateBindings(a.ID, toolIDs[:1], nil, nil, nil, nil); err != nil {
		t.Fatalf("seed binding: %v", err)
	}

	err := svc.UpdateBindings(a.ID, []uint{999999}, nil, nil, nil, nil)
	if !errors.Is(err, ErrInvalidBinding) {
		t.Fatalf("expected ErrInvalidBinding, got %v", err)
	}

	loadedToolIDs, _, _, _, _, err := svc.GetBindings(a.ID)
	if err != nil {
		t.Fatalf("get bindings: %v", err)
	}
	if len(loadedToolIDs) != 1 || loadedToolIDs[0] != toolIDs[0] {
		t.Fatalf("binding should be unchanged, got %v", loadedToolIDs)
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

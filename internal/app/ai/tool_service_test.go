package ai

import (
	"testing"
)

func TestToolService_List(t *testing.T) {
	db := setupTestDB(t)
	svc := newToolServiceForTest(t, db)
	repo := svc.repo

	// Seed tools
	_ = repo.Create(&Tool{Name: "search", Toolkit: "general", DisplayName: "Search", IsActive: true})
	_ = repo.Create(&Tool{Name: "calc", Toolkit: "general", DisplayName: "Calculator", IsActive: true})

	tools, err := svc.List()
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}
	if len(tools) != 2 {
		t.Errorf("expected 2 tools, got %d", len(tools))
	}
}

func TestToolService_ToggleActive(t *testing.T) {
	db := setupTestDB(t)
	svc := newToolServiceForTest(t, db)
	repo := svc.repo

	tool := &Tool{Name: "search", Toolkit: "general", DisplayName: "Search", IsActive: true}
	_ = repo.Create(tool)

	updated, err := svc.ToggleActive(tool.ID, false)
	if err != nil {
		t.Fatalf("toggle active: %v", err)
	}
	if updated.IsActive {
		t.Error("expected tool to be inactive")
	}
}

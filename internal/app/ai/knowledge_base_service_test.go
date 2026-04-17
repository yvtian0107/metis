package ai

import "testing"

func TestKnowledgeBaseService_Create(t *testing.T) {
	db := setupTestDB(t)
	graphRepo := &stubKnowledgeGraphRepo{}
	svc := newKnowledgeBaseServiceForTest(t, db, graphRepo)

	kb := &KnowledgeBase{Name: "KB1"}
	if err := svc.Create(kb); err != nil {
		t.Fatalf("create: %v", err)
	}
	if kb.CompileStatus != CompileStatusIdle {
		t.Errorf("compileStatus: expected %q, got %q", CompileStatusIdle, kb.CompileStatus)
	}
	if kb.ID == 0 {
		t.Error("expected ID to be assigned")
	}
}

func TestKnowledgeBaseService_Get(t *testing.T) {
	db := setupTestDB(t)
	graphRepo := &stubKnowledgeGraphRepo{}
	svc := newKnowledgeBaseServiceForTest(t, db, graphRepo)

	kb := &KnowledgeBase{Name: "KB1"}
	_ = svc.Create(kb)

	loaded, err := svc.Get(kb.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if loaded.Name != "KB1" {
		t.Errorf("name: expected %q, got %q", "KB1", loaded.Name)
	}
}

func TestKnowledgeBaseService_Get_NotFound(t *testing.T) {
	db := setupTestDB(t)
	graphRepo := &stubKnowledgeGraphRepo{}
	svc := newKnowledgeBaseServiceForTest(t, db, graphRepo)

	_, err := svc.Get(9999)
	if err != ErrKnowledgeBaseNotFound {
		t.Errorf("expected %v, got %v", ErrKnowledgeBaseNotFound, err)
	}
}

func TestKnowledgeBaseService_Update(t *testing.T) {
	db := setupTestDB(t)
	graphRepo := &stubKnowledgeGraphRepo{}
	svc := newKnowledgeBaseServiceForTest(t, db, graphRepo)

	kb := &KnowledgeBase{Name: "KB1"}
	_ = svc.Create(kb)

	kb.Description = "Updated"
	if err := svc.Update(kb); err != nil {
		t.Fatalf("update: %v", err)
	}

	loaded, _ := svc.Get(kb.ID)
	if loaded.Description != "Updated" {
		t.Errorf("description: expected %q, got %q", "Updated", loaded.Description)
	}
}

func TestKnowledgeBaseService_Delete_Cascade(t *testing.T) {
	db := setupTestDB(t)
	graphRepo := &stubKnowledgeGraphRepo{}
	svc := newKnowledgeBaseServiceForTest(t, db, graphRepo)
	sourceSvc := newKnowledgeSourceServiceForTest(t, db, graphRepo)

	kb := &KnowledgeBase{Name: "KB1"}
	_ = svc.Create(kb)

	src := &KnowledgeSource{KbID: kb.ID, Title: "Source1", Format: SourceFormatMarkdown}
	_ = sourceSvc.Create(src)

	if err := svc.Delete(kb.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	_, err := svc.Get(kb.ID)
	if err != ErrKnowledgeBaseNotFound {
		t.Errorf("expected kb not found, got %v", err)
	}

	if len(graphRepo.deleteGraphCalls) != 1 || graphRepo.deleteGraphCalls[0] != kb.ID {
		t.Errorf("expected DeleteGraph called with %d, got %v", kb.ID, graphRepo.deleteGraphCalls)
	}

	_, err = sourceSvc.Get(src.ID)
	if err != ErrSourceNotFound {
		t.Errorf("expected source not found, got %v", err)
	}
}

package ai

import "testing"

func TestKnowledgeSourceService_Create_URLWithCrawlSettings(t *testing.T) {
	db := setupTestDB(t)
	graphRepo := &stubKnowledgeGraphRepo{}
	kbSvc := newKnowledgeBaseServiceForTest(t, db, graphRepo)
	svc := newKnowledgeSourceServiceForTest(t, db, graphRepo)

	kb := &KnowledgeBase{Name: "KB1"}
	_ = kbSvc.Create(kb)

	src := &KnowledgeSource{
		KbID:          kb.ID,
		Title:         "Example URL",
		Format:        SourceFormatURL,
		SourceURL:     "https://example.com",
		CrawlDepth:    2,
		URLPattern:    "/docs",
		CrawlEnabled:  true,
		CrawlSchedule: "0 9 * * *",
	}
	if err := svc.Create(src); err != nil {
		t.Fatalf("create: %v", err)
	}

	if src.ID == 0 {
		t.Error("expected ID to be assigned")
	}

	loaded, _ := kbSvc.Get(kb.ID)
	if loaded.SourceCount != 1 {
		t.Errorf("sourceCount: expected 1, got %d", loaded.SourceCount)
	}
}

func TestKnowledgeSourceService_ListByKB(t *testing.T) {
	db := setupTestDB(t)
	graphRepo := &stubKnowledgeGraphRepo{}
	kbSvc := newKnowledgeBaseServiceForTest(t, db, graphRepo)
	svc := newKnowledgeSourceServiceForTest(t, db, graphRepo)

	kb := &KnowledgeBase{Name: "KB1"}
	_ = kbSvc.Create(kb)

	_ = svc.Create(&KnowledgeSource{KbID: kb.ID, Title: "S1", Format: SourceFormatMarkdown})
	_ = svc.Create(&KnowledgeSource{KbID: kb.ID, Title: "S2", Format: SourceFormatMarkdown})

	items, total, err := svc.repo.List(SourceListParams{KbID: kb.ID, Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 2 {
		t.Errorf("total: expected 2, got %d", total)
	}
	if len(items) != 2 {
		t.Errorf("items: expected 2, got %d", len(items))
	}
}

func TestKnowledgeSourceService_Delete(t *testing.T) {
	db := setupTestDB(t)
	graphRepo := &stubKnowledgeGraphRepo{}
	kbSvc := newKnowledgeBaseServiceForTest(t, db, graphRepo)
	svc := newKnowledgeSourceServiceForTest(t, db, graphRepo)

	kb := &KnowledgeBase{Name: "KB1"}
	_ = kbSvc.Create(kb)

	src := &KnowledgeSource{KbID: kb.ID, Title: "Parent", Format: SourceFormatURL}
	_ = svc.Create(src)

	if err := svc.Delete(src.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	_, err := svc.Get(src.ID)
	if err != ErrSourceNotFound {
		t.Errorf("expected not found, got %v", err)
	}

	if len(graphRepo.deleteNodesBySource) != 1 {
		t.Fatalf("expected DeleteNodesBySourceID called once, got %d", len(graphRepo.deleteNodesBySource))
	}
	if graphRepo.deleteNodesBySource[0].kbID != kb.ID || graphRepo.deleteNodesBySource[0].sourceID != src.ID {
		t.Errorf("unexpected call args: %+v", graphRepo.deleteNodesBySource[0])
	}

	loaded, _ := kbSvc.Get(kb.ID)
	if loaded.SourceCount != 0 {
		t.Errorf("sourceCount: expected 0, got %d", loaded.SourceCount)
	}
}

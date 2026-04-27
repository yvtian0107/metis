package runtime

import (
	"context"
	"errors"
	"testing"

	"gorm.io/gorm"
)

func seedProvider(t *testing.T, db *gorm.DB, svc *ProviderService, providerType string) *Provider {
	t.Helper()
	p, err := svc.Create("Test "+providerType, providerType, "https://example.com", "key")
	if err != nil {
		t.Fatalf("seed provider: %v", err)
	}
	return p
}

func TestModelService_Create(t *testing.T) {
	db := setupTestDB(t)
	providerSvc := newProviderServiceForTest(t, db)
	svc := newModelServiceForTest(t, db)

	p := seedProvider(t, db, providerSvc, ProviderTypeOpenAI)

	m := &AIModel{
		ModelID:     "gpt-4",
		DisplayName: "GPT-4",
		ProviderID:  p.ID,
		Type:        ModelTypeLLM,
	}
	if err := svc.Create(m); err != nil {
		t.Fatalf("create model: %v", err)
	}

	if m.ProviderID != p.ID {
		t.Errorf("providerID: expected %d, got %d", p.ID, m.ProviderID)
	}
	if m.Status != ModelStatusActive {
		t.Errorf("status: expected %q, got %q", ModelStatusActive, m.Status)
	}
}

func TestModelService_Update(t *testing.T) {
	db := setupTestDB(t)
	providerSvc := newProviderServiceForTest(t, db)
	svc := newModelServiceForTest(t, db)

	p := seedProvider(t, db, providerSvc, ProviderTypeOpenAI)
	m := &AIModel{ModelID: "gpt-4", DisplayName: "GPT-4", ProviderID: p.ID, Type: ModelTypeLLM}
	_ = svc.Create(m)

	m.DisplayName = "GPT-4 Updated"
	m.ContextWindow = 128000
	m.InputPrice = 0.005
	m.OutputPrice = 0.015
	if err := svc.Update(m); err != nil {
		t.Fatalf("update model: %v", err)
	}

	loaded, err := svc.Get(m.ID)
	if err != nil {
		t.Fatalf("get model: %v", err)
	}
	if loaded.DisplayName != "GPT-4 Updated" {
		t.Errorf("displayName: expected %q, got %q", "GPT-4 Updated", loaded.DisplayName)
	}
	if loaded.ContextWindow != 128000 {
		t.Errorf("contextWindow: expected %d, got %d", 128000, loaded.ContextWindow)
	}
	if loaded.InputPrice != 0.005 {
		t.Errorf("inputPrice: expected %f, got %f", 0.005, loaded.InputPrice)
	}
}

func TestModelService_UpdateRejectsInvalidTypeAndStatus(t *testing.T) {
	db := setupTestDB(t)
	providerSvc := newProviderServiceForTest(t, db)
	svc := newModelServiceForTest(t, db)

	p := seedProvider(t, db, providerSvc, ProviderTypeOpenAI)
	m := &AIModel{ModelID: "gpt-4", DisplayName: "GPT-4", ProviderID: p.ID, Type: ModelTypeLLM}
	if err := svc.Create(m); err != nil {
		t.Fatalf("create model: %v", err)
	}

	m.Type = "invalid"
	if err := svc.Update(m); !errors.Is(err, ErrInvalidType) {
		t.Fatalf("expected ErrInvalidType, got %v", err)
	}

	m.Type = ModelTypeLLM
	m.Status = "invalid"
	if err := svc.Update(m); !errors.Is(err, ErrInvalidStatus) {
		t.Fatalf("expected ErrInvalidStatus, got %v", err)
	}
}

func TestModelService_Delete(t *testing.T) {
	db := setupTestDB(t)
	providerSvc := newProviderServiceForTest(t, db)
	svc := newModelServiceForTest(t, db)

	p := seedProvider(t, db, providerSvc, ProviderTypeOpenAI)
	m := &AIModel{ModelID: "gpt-4", DisplayName: "GPT-4", ProviderID: p.ID, Type: ModelTypeLLM}
	_ = svc.Create(m)

	if err := svc.Delete(m.ID); err != nil {
		t.Fatalf("delete model: %v", err)
	}

	_, err := svc.Get(m.ID)
	if err == nil {
		t.Error("expected model to be deleted")
	}
}

func TestModelService_SetDefault_SwitchesDefault(t *testing.T) {
	db := setupTestDB(t)
	providerSvc := newProviderServiceForTest(t, db)
	svc := newModelServiceForTest(t, db)

	p := seedProvider(t, db, providerSvc, ProviderTypeOpenAI)
	m1 := &AIModel{ModelID: "gpt-4", DisplayName: "GPT-4", ProviderID: p.ID, Type: ModelTypeLLM}
	m2 := &AIModel{ModelID: "gpt-3.5", DisplayName: "GPT-3.5", ProviderID: p.ID, Type: ModelTypeLLM}
	_ = svc.Create(m1)
	_ = svc.Create(m2)

	// Set first as default
	if err := svc.SetDefault(m1.ID); err != nil {
		t.Fatalf("set default m1: %v", err)
	}
	loaded1, _ := svc.Get(m1.ID)
	if !loaded1.IsDefault {
		t.Error("m1 should be default")
	}

	// Switch default to second
	if err := svc.SetDefault(m2.ID); err != nil {
		t.Fatalf("set default m2: %v", err)
	}
	loaded2, _ := svc.Get(m2.ID)
	if !loaded2.IsDefault {
		t.Error("m2 should be default")
	}

	// Verify m1 is no longer default
	loaded1, _ = svc.Get(m1.ID)
	if loaded1.IsDefault {
		t.Error("m1 should no longer be default")
	}
}

func TestModelService_SyncModels_Anthropic(t *testing.T) {
	db := setupTestDB(t)
	providerSvc := newProviderServiceForTest(t, db)
	svc := newModelServiceForTest(t, db)

	p := seedProvider(t, db, providerSvc, ProviderTypeAnthropic)

	added, err := svc.SyncModels(context.Background(), p.ID)
	if err != nil {
		t.Fatalf("sync models: %v", err)
	}
	if added != len(AnthropicPresetModels) {
		t.Errorf("added: expected %d, got %d", len(AnthropicPresetModels), added)
	}

	// Verify one preset model
	m, err := svc.repo.FindByModelIDAndProvider("claude-opus-4-20250514", p.ID)
	if err != nil {
		t.Fatalf("find synced model: %v", err)
	}
	if m.DisplayName != "Claude Opus 4" {
		t.Errorf("displayName: expected %q, got %q", "Claude Opus 4", m.DisplayName)
	}
}

func TestModelService_SyncModels_Idempotent(t *testing.T) {
	db := setupTestDB(t)
	providerSvc := newProviderServiceForTest(t, db)
	svc := newModelServiceForTest(t, db)

	p := seedProvider(t, db, providerSvc, ProviderTypeAnthropic)

	first, _ := svc.SyncModels(context.Background(), p.ID)
	second, _ := svc.SyncModels(context.Background(), p.ID)

	if first != len(AnthropicPresetModels) {
		t.Errorf("first sync: expected %d, got %d", len(AnthropicPresetModels), first)
	}
	if second != 0 {
		t.Errorf("second sync: expected 0, got %d", second)
	}
}

func TestModelService_SetDefault_ScopedToProvider(t *testing.T) {
	db := setupTestDB(t)
	providerSvc := newProviderServiceForTest(t, db)
	svc := newModelServiceForTest(t, db)

	pA := seedProvider(t, db, providerSvc, ProviderTypeOpenAI)
	pB := seedProvider(t, db, providerSvc, ProviderTypeAnthropic)

	mA := &AIModel{ModelID: "gpt-4", DisplayName: "GPT-4", ProviderID: pA.ID, Type: ModelTypeLLM}
	mB := &AIModel{ModelID: "claude-opus", DisplayName: "Claude Opus", ProviderID: pB.ID, Type: ModelTypeLLM}
	_ = svc.Create(mA)
	_ = svc.Create(mB)

	// Set gpt-4 as default in Provider A
	if err := svc.SetDefault(mA.ID); err != nil {
		t.Fatalf("set default mA: %v", err)
	}
	loadedA, _ := svc.Get(mA.ID)
	if !loadedA.IsDefault {
		t.Error("mA should be default")
	}

	// Set claude-opus as default in Provider B
	if err := svc.SetDefault(mB.ID); err != nil {
		t.Fatalf("set default mB: %v", err)
	}
	loadedB, _ := svc.Get(mB.ID)
	if !loadedB.IsDefault {
		t.Error("mB should be default")
	}

	// Verify: gpt-4 in Provider A should STILL be default (not cleared)
	loadedA, _ = svc.Get(mA.ID)
	if !loadedA.IsDefault {
		t.Error("mA should still be default — SetDefault must be scoped to provider")
	}
}

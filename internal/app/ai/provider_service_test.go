package ai

import (
	"strings"
	"testing"

	"metis/internal/database"
)

func TestProviderService_Create_OpenAI(t *testing.T) {
	db := setupTestDB(t)
	svc := newProviderServiceForTest(t, db)

	p, err := svc.Create("OpenAI", ProviderTypeOpenAI, "https://api.openai.com/v1", "sk-test-key")
	if err != nil {
		t.Fatalf("create provider: %v", err)
	}

	if p.Protocol != "openai" {
		t.Errorf("protocol: expected %q, got %q", "openai", p.Protocol)
	}
	if p.Status != ProviderStatusInactive {
		t.Errorf("status: expected %q, got %q", ProviderStatusInactive, p.Status)
	}
	if len(p.APIKeyEncrypted) == 0 {
		t.Error("expected API key to be encrypted")
	}

	// Verify encryption round-trip
	plain, err := svc.DecryptAPIKey(p)
	if err != nil {
		t.Fatalf("decrypt api key: %v", err)
	}
	if plain != "sk-test-key" {
		t.Errorf("decrypted key: expected %q, got %q", "sk-test-key", plain)
	}
}

func TestProviderService_Create_Anthropic(t *testing.T) {
	db := setupTestDB(t)
	svc := newProviderServiceForTest(t, db)

	p, err := svc.Create("Anthropic", ProviderTypeAnthropic, "https://api.anthropic.com", "sk-ant-test")
	if err != nil {
		t.Fatalf("create provider: %v", err)
	}

	if p.Protocol != "anthropic" {
		t.Errorf("protocol: expected %q, got %q", "anthropic", p.Protocol)
	}
}

func TestProviderService_Update_PreservesKey(t *testing.T) {
	db := setupTestDB(t)
	svc := newProviderServiceForTest(t, db)

	p, err := svc.Create("OpenAI", ProviderTypeOpenAI, "https://api.openai.com/v1", "original-key")
	if err != nil {
		t.Fatalf("create provider: %v", err)
	}

	updated, err := svc.Update(p.ID, "OpenAI Updated", ProviderTypeOpenAI, "https://api.openai.com/v2", "")
	if err != nil {
		t.Fatalf("update provider: %v", err)
	}

	// Verify key preserved
	plain, err := svc.DecryptAPIKey(updated)
	if err != nil {
		t.Fatalf("decrypt api key: %v", err)
	}
	if plain != "original-key" {
		t.Errorf("expected key preserved as %q, got %q", "original-key", plain)
	}
}

func TestProviderService_Update_ReencryptsKey(t *testing.T) {
	db := setupTestDB(t)
	svc := newProviderServiceForTest(t, db)

	p, err := svc.Create("OpenAI", ProviderTypeOpenAI, "https://api.openai.com/v1", "old-key")
	if err != nil {
		t.Fatalf("create provider: %v", err)
	}

	updated, err := svc.Update(p.ID, "OpenAI", ProviderTypeOpenAI, "https://api.openai.com/v1", "new-key")
	if err != nil {
		t.Fatalf("update provider: %v", err)
	}

	plain, err := svc.DecryptAPIKey(updated)
	if err != nil {
		t.Fatalf("decrypt api key: %v", err)
	}
	if plain != "new-key" {
		t.Errorf("expected key re-encrypted to %q, got %q", "new-key", plain)
	}
}

func TestProviderService_Delete_CascadesModels(t *testing.T) {
	db := setupTestDB(t)
	svc := newProviderServiceForTest(t, db)
	modelRepo := &ModelRepo{db: &database.DB{DB: db}}

	p, err := svc.Create("OpenAI", ProviderTypeOpenAI, "https://api.openai.com/v1", "key")
	if err != nil {
		t.Fatalf("create provider: %v", err)
	}

	// Create associated model
	m := &AIModel{ModelID: "gpt-4", DisplayName: "GPT-4", ProviderID: p.ID, Type: ModelTypeLLM}
	if err := modelRepo.Create(m); err != nil {
		t.Fatalf("create model: %v", err)
	}

	if err := svc.Delete(p.ID); err != nil {
		t.Fatalf("delete provider: %v", err)
	}

	// Verify provider deleted
	var count int64
	db.Model(&Provider{}).Where("id = ?", p.ID).Count(&count)
	if count != 0 {
		t.Errorf("expected provider to be deleted, count=%d", count)
	}

	// Verify model deleted
	db.Model(&AIModel{}).Where("provider_id = ?", p.ID).Count(&count)
	if count != 0 {
		t.Errorf("expected models to be deleted, count=%d", count)
	}
}

func TestProviderService_MaskAPIKey(t *testing.T) {
	db := setupTestDB(t)
	svc := newProviderServiceForTest(t, db)

	// Long key
	p, _ := svc.Create("OpenAI", ProviderTypeOpenAI, "https://api.openai.com/v1", "sk-1234567890abcdef")
	masked := svc.MaskAPIKey(p)
	want := "sk-****90abcdef"
	if !strings.HasPrefix(masked, "sk-") || !strings.Contains(masked, "****") {
		t.Errorf("long key mask: expected pattern %q, got %q", want, masked)
	}

	// Short key
	p2, _ := svc.Create("Anthropic", ProviderTypeAnthropic, "https://api.anthropic.com", "short")
	masked2 := svc.MaskAPIKey(p2)
	if masked2 != "****" {
		t.Errorf("short key mask: expected %q, got %q", "****", masked2)
	}
}

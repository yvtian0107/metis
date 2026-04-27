package runtime

import (
	"testing"

	casbinpkg "metis/internal/casbin"
	"metis/internal/model"
)

func TestSeedAI_AddsPoliciesForRegisteredSessionRoutes(t *testing.T) {
	db := setupTestDB(t)
	if err := db.AutoMigrate(&model.Menu{}); err != nil {
		t.Fatalf("migrate menu: %v", err)
	}
	enforcer, err := casbinpkg.NewEnforcerWithDB(db)
	if err != nil {
		t.Fatalf("create enforcer: %v", err)
	}

	if err := SeedAI(db, enforcer); err != nil {
		t.Fatalf("seed ai: %v", err)
	}

	policies := [][]string{
		{"admin", "/api/v1/ai/sessions/:sid", "PUT"},
		{"admin", "/api/v1/ai/sessions/:sid/chat", "POST"},
		{"admin", "/api/v1/ai/sessions/:sid/messages/:mid", "PUT"},
		{"admin", "/api/v1/ai/sessions/:sid/continue", "POST"},
	}
	for _, policy := range policies {
		ok, err := enforcer.HasPolicy(policy)
		if err != nil {
			t.Fatalf("check policy %v: %v", policy, err)
		}
		if !ok {
			t.Fatalf("expected policy %v", policy)
		}
	}
}

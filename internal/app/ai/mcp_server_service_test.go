package ai

import (
	"testing"
)

func TestMCPServerService_Create_SSERequiresURL(t *testing.T) {
	db := setupTestDB(t)
	svc := newMCPServerServiceForTest(t, db)

	m := &MCPServer{Name: "Test", Transport: MCPTransportSSE, URL: ""}
	err := svc.Create(m, "")
	if err != ErrSSERequiresURL {
		t.Errorf("expected %v, got %v", ErrSSERequiresURL, err)
	}
}

func TestMCPServerService_Create_STDIORequiresCommand(t *testing.T) {
	db := setupTestDB(t)
	svc := newMCPServerServiceForTest(t, db)

	m := &MCPServer{Name: "Test", Transport: MCPTransportSTDIO, Command: ""}
	err := svc.Create(m, "")
	if err != ErrSTDIORequiresCommand {
		t.Errorf("expected %v, got %v", ErrSTDIORequiresCommand, err)
	}
}

func TestMCPServerService_Create_ValidSSE(t *testing.T) {
	db := setupTestDB(t)
	svc := newMCPServerServiceForTest(t, db)

	m := &MCPServer{Name: "Test", Transport: MCPTransportSSE, URL: "https://example.com/sse"}
	err := svc.Create(m, "")
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}

	loaded, err := svc.Get(m.ID)
	if err != nil {
		t.Fatalf("get mcp server: %v", err)
	}
	if loaded.URL != "https://example.com/sse" {
		t.Errorf("url: expected %q, got %q", "https://example.com/sse", loaded.URL)
	}
}

func TestMCPServerService_Create_EncryptsAuthConfig(t *testing.T) {
	db := setupTestDB(t)
	svc := newMCPServerServiceForTest(t, db)

	m := &MCPServer{Name: "Test", Transport: MCPTransportSSE, URL: "https://example.com/sse", AuthType: AuthTypeAPIKey}
	err := svc.Create(m, `{"key":"secret123"}`)
	if err != nil {
		t.Fatalf("create mcp server: %v", err)
	}

	if len(m.AuthConfigEncrypted) == 0 {
		t.Error("expected auth config to be encrypted")
	}

	plain, err := svc.DecryptAuthConfig(m)
	if err != nil {
		t.Fatalf("decrypt auth config: %v", err)
	}
	if plain != `{"key":"secret123"}` {
		t.Errorf("decrypted config: expected %q, got %q", `{"key":"secret123"}`, plain)
	}
}

func TestMCPServerService_MaskAuthConfig(t *testing.T) {
	db := setupTestDB(t)
	svc := newMCPServerServiceForTest(t, db)

	m := &MCPServer{Name: "Test", Transport: MCPTransportSSE, URL: "https://example.com/sse", AuthType: AuthTypeAPIKey}
	_ = svc.Create(m, `{"key":"sk-1234567890abcdef"}`)

	masked := svc.MaskAuthConfig(m)
	if masked == "" {
		t.Fatal("expected masked config, got empty string")
	}
	if !contains(masked, "****") {
		t.Errorf("expected masked config to contain ****, got %q", masked)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsAt(s, substr))
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

package tools

import (
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"metis/internal/model"
)

func TestSessionStateStoreInvalidJSONReturnsError(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:itsm_state_invalid_json?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.Exec("CREATE TABLE ai_agent_sessions (id INTEGER PRIMARY KEY, state TEXT)").Error; err != nil {
		t.Fatalf("create table: %v", err)
	}
	if err := db.AutoMigrate(&model.AuditLog{}); err != nil {
		t.Fatalf("migrate audit log: %v", err)
	}
	if err := db.Exec("INSERT INTO ai_agent_sessions (id, state) VALUES (?, ?)", 42, "{bad-json").Error; err != nil {
		t.Fatalf("insert state: %v", err)
	}

	store := NewSessionStateStore(db)
	state, err := store.GetState(42)
	if err == nil {
		t.Fatalf("expected invalid state json error, got state=%+v", state)
	}
	if !strings.Contains(err.Error(), "invalid service desk state") {
		t.Fatalf("unexpected error: %v", err)
	}
	var audit model.AuditLog
	if err := db.Where("action = ? AND resource_id = ?", "itsm.service_desk.state_invalid", "42").First(&audit).Error; err != nil {
		t.Fatalf("expected invalid-state audit log: %v", err)
	}
	if audit.Level != model.AuditLevelError {
		t.Fatalf("expected error audit level, got %s", audit.Level)
	}
}

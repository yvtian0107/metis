package engine

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// --- renderTemplate tests ---

// setupNotifyTestDB creates a private in-memory SQLite database with the tables needed for renderTemplate.
func setupNotifyTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:notify_%p?mode=memory&cache=shared", t)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	if err := db.AutoMigrate(&ticketModel{}, &processVariableModel{}, &timelineModel{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	return db
}

func TestRenderTemplate_TicketCode(t *testing.T) {
	db := setupNotifyTestDB(t)
	db.Create(&ticketModel{ID: 42, Status: "in_progress"})

	e := &ClassicEngine{}
	got := e.renderTemplate(db, 42, "root", "工单编号: {{ticket.code}}")
	want := "工单编号: TICK-000042"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRenderTemplate_TicketStatus(t *testing.T) {
	db := setupNotifyTestDB(t)
	db.Create(&ticketModel{ID: 1, Status: "completed"})

	e := &ClassicEngine{}
	got := e.renderTemplate(db, 1, "root", "当前状态: {{ticket.status}}")
	want := "当前状态: completed"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRenderTemplate_VarReplacement(t *testing.T) {
	db := setupNotifyTestDB(t)
	db.Create(&ticketModel{ID: 1, Status: "in_progress"})
	db.Create(&processVariableModel{TicketID: 1, ScopeID: "root", Key: "vpn_account", Value: "zhangsan"})
	db.Create(&processVariableModel{TicketID: 1, ScopeID: "root", Key: "reason", Value: "出差需要"})

	e := &ClassicEngine{}
	got := e.renderTemplate(db, 1, "root", "VPN账号 {{var.vpn_account}} 申请原因: {{var.reason}}")
	want := "VPN账号 zhangsan 申请原因: 出差需要"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRenderTemplate_AllPlaceholders(t *testing.T) {
	db := setupNotifyTestDB(t)
	db.Create(&ticketModel{ID: 7, Status: "pending"})
	db.Create(&processVariableModel{TicketID: 7, ScopeID: "root", Key: "env", Value: "production"})

	e := &ClassicEngine{}
	tmpl := "工单 {{ticket.code}} 状态 {{ticket.status}} 环境 {{var.env}}"
	got := e.renderTemplate(db, 7, "root", tmpl)
	want := "工单 TICK-000007 状态 pending 环境 production"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRenderTemplate_NoPlaceholders(t *testing.T) {
	db := setupNotifyTestDB(t)
	db.Create(&ticketModel{ID: 1, Status: "open"})

	e := &ClassicEngine{}
	tmpl := "这是一段纯文本，没有任何占位符。"
	got := e.renderTemplate(db, 1, "root", tmpl)
	if got != tmpl {
		t.Errorf("no-placeholder template should be unchanged, got %q", got)
	}
}

func TestRenderTemplate_UnknownVarLeftAsIs(t *testing.T) {
	db := setupNotifyTestDB(t)
	db.Create(&ticketModel{ID: 1, Status: "open"})

	e := &ClassicEngine{}
	tmpl := "值: {{var.nonexistent}}"
	got := e.renderTemplate(db, 1, "root", tmpl)
	// Unknown var placeholders remain unreplaced since no matching processVariable exists
	if got != tmpl {
		t.Errorf("unknown var should remain, got %q", got)
	}
}

func TestRenderTemplate_TicketNotFound(t *testing.T) {
	db := setupNotifyTestDB(t)
	// No ticket created — renderTemplate should return the template unchanged

	e := &ClassicEngine{}
	tmpl := "工单 {{ticket.code}}"
	got := e.renderTemplate(db, 999, "root", tmpl)
	if got != tmpl {
		t.Errorf("missing ticket should return original template, got %q", got)
	}
}

func TestRenderTemplate_ScopeIsolation(t *testing.T) {
	db := setupNotifyTestDB(t)
	db.Create(&ticketModel{ID: 1, Status: "in_progress"})
	db.Create(&processVariableModel{TicketID: 1, ScopeID: "root", Key: "x", Value: "root_val"})
	db.Create(&processVariableModel{TicketID: 1, ScopeID: "sub1", Key: "x", Value: "sub_val"})

	e := &ClassicEngine{}

	gotRoot := e.renderTemplate(db, 1, "root", "{{var.x}}")
	if gotRoot != "root_val" {
		t.Errorf("root scope: got %q, want %q", gotRoot, "root_val")
	}

	gotSub := e.renderTemplate(db, 1, "sub1", "{{var.x}}")
	if gotSub != "sub_val" {
		t.Errorf("sub scope: got %q, want %q", gotSub, "sub_val")
	}
}

func TestRenderTemplate_MultipleOccurrences(t *testing.T) {
	db := setupNotifyTestDB(t)
	db.Create(&ticketModel{ID: 5, Status: "open"})

	e := &ClassicEngine{}
	tmpl := "{{ticket.code}} and again {{ticket.code}}"
	got := e.renderTemplate(db, 5, "root", tmpl)
	want := "TICK-000005 and again TICK-000005"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRenderTemplate_CodeFormat(t *testing.T) {
	db := setupNotifyTestDB(t)

	tests := []struct {
		ticketID uint
		wantCode string
	}{
		{1, "TICK-000001"},
		{999999, "TICK-999999"},
		{1000000, "TICK-1000000"},
	}

	e := &ClassicEngine{}
	for _, tt := range tests {
		db.Create(&ticketModel{ID: tt.ticketID, Status: "open"})
		got := e.renderTemplate(db, tt.ticketID, "root", "{{ticket.code}}")
		want := tt.wantCode
		if got != want {
			t.Errorf("ticketID=%d: got %q, want %q", tt.ticketID, got, want)
		}
	}
}

// --- NotificationSender mock and interface contract tests ---

// mockNotificationSender records calls and optionally returns an error.
type mockNotificationSender struct {
	calls []mockSendCall
	err   error // if non-nil, Send returns this error
}

type mockSendCall struct {
	ChannelID    uint
	Subject      string
	Body         string
	RecipientIDs []uint
}

func (m *mockNotificationSender) Send(_ context.Context, channelID uint, subject, body string, recipientIDs []uint) error {
	m.calls = append(m.calls, mockSendCall{
		ChannelID:    channelID,
		Subject:      subject,
		Body:         body,
		RecipientIDs: recipientIDs,
	})
	return m.err
}

func TestNotificationSender_MockRecordsCalls(t *testing.T) {
	mock := &mockNotificationSender{}

	err := mock.Send(context.Background(), 10, "VPN申请通知", "您的VPN已开通", []uint{1, 2, 3})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mock.calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(mock.calls))
	}

	call := mock.calls[0]
	if call.ChannelID != 10 {
		t.Errorf("channelID: got %d, want 10", call.ChannelID)
	}
	if call.Subject != "VPN申请通知" {
		t.Errorf("subject: got %q, want %q", call.Subject, "VPN申请通知")
	}
	if call.Body != "您的VPN已开通" {
		t.Errorf("body: got %q, want %q", call.Body, "您的VPN已开通")
	}
	if len(call.RecipientIDs) != 3 || call.RecipientIDs[0] != 1 || call.RecipientIDs[2] != 3 {
		t.Errorf("recipientIDs: got %v, want [1 2 3]", call.RecipientIDs)
	}
}

func TestNotificationSender_MockMultipleCalls(t *testing.T) {
	mock := &mockNotificationSender{}

	_ = mock.Send(context.Background(), 1, "s1", "b1", []uint{10})
	_ = mock.Send(context.Background(), 2, "s2", "b2", []uint{20, 30})

	if len(mock.calls) != 2 {
		t.Fatalf("expected 2 calls, got %d", len(mock.calls))
	}
	if mock.calls[0].ChannelID != 1 || mock.calls[1].ChannelID != 2 {
		t.Error("call order or channelIDs mismatch")
	}
}

func TestNotificationSender_FailingMockDoesNotPanic(t *testing.T) {
	sendErr := errors.New("smtp connection refused")
	mock := &mockNotificationSender{err: sendErr}

	err := mock.Send(context.Background(), 5, "test", "body", []uint{1})
	if !errors.Is(err, sendErr) {
		t.Errorf("expected sendErr, got %v", err)
	}
	// The call should still be recorded even when returning an error
	if len(mock.calls) != 1 {
		t.Fatalf("expected 1 call recorded, got %d", len(mock.calls))
	}
}

func TestNotificationSender_InterfaceSatisfied(t *testing.T) {
	// Compile-time check that mockNotificationSender satisfies NotificationSender
	var _ NotificationSender = (*mockNotificationSender)(nil)
}

func TestClassicEngine_AcceptsNilNotifier(t *testing.T) {
	// ClassicEngine should work with a nil notifier (notifications disabled)
	e := NewClassicEngine(NewParticipantResolver(nil), nil, nil)
	if e == nil {
		t.Fatal("NewClassicEngine returned nil")
	}
	if e.notifier != nil {
		t.Error("expected nil notifier")
	}
}

func TestClassicEngine_AcceptsMockNotifier(t *testing.T) {
	mock := &mockNotificationSender{}
	e := NewClassicEngine(NewParticipantResolver(nil), nil, mock)
	if e == nil {
		t.Fatal("NewClassicEngine returned nil")
	}
	if e.notifier == nil {
		t.Error("expected non-nil notifier")
	}

	// Verify the notifier is usable through the engine
	err := e.notifier.Send(context.Background(), 1, "test", "body", []uint{1})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(mock.calls) != 1 {
		t.Errorf("expected 1 call on mock, got %d", len(mock.calls))
	}
}

func TestClassicEngine_FailingNotifierDoesNotPanic(t *testing.T) {
	mock := &mockNotificationSender{err: fmt.Errorf("network error")}
	e := NewClassicEngine(NewParticipantResolver(nil), nil, mock)

	// Calling Send through the engine's notifier should not panic even on error
	err := e.notifier.Send(context.Background(), 1, "test", "body", []uint{1})
	if err == nil {
		t.Error("expected error from failing notifier")
	}
}

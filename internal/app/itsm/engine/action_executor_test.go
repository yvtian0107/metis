package engine

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupActionExecutorDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&ticketModel{}, &serviceActionModel{}, &actionExecutionModel{}); err != nil {
		t.Fatalf("migrate db: %v", err)
	}
	return db
}

func TestActionExecutorExecuteRendersTemplateAndPersistsSuccess(t *testing.T) {
	db := setupActionExecutorDB(t)
	if err := db.Create(&ticketModel{
		ID:          1,
		Code:        "TICK-EXEC-001",
		Status:      "waiting_human",
		RequesterID: 7,
		PriorityID:  3,
		FormData:    `{"env":"prod"}`,
	}).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}

	requests := make(chan struct {
		method string
		header http.Header
		body   string
	}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		requests <- struct {
			method string
			header http.Header
			body   string
		}{
			method: r.Method,
			header: r.Header.Clone(),
			body:   string(body),
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	configJSON := fmt.Sprintf(`{
		"url": %q,
		"method": "POST",
		"headers": {"X-Test":"1"},
		"body": {"ticket":"{{ticket.code}}","env":"{{ticket.form_data.env}}","requester":"{{ticket.requester_id}}"},
		"timeout": 5,
		"retries": 0
	}`, server.URL)
	if err := db.Create(&serviceActionModel{
		ID:         1,
		Name:       "Webhook",
		Code:       "notify-webhook",
		ServiceID:  1,
		IsActive:   true,
		ActionType: "http",
		ConfigJSON: configJSON,
	}).Error; err != nil {
		t.Fatalf("create action: %v", err)
	}

	executor := NewActionExecutor(db)
	if err := executor.Execute(context.Background(), 1, 2, 1); err != nil {
		t.Fatalf("execute action: %v", err)
	}

	req := <-requests
	if req.method != http.MethodPost {
		t.Fatalf("method = %s, want POST", req.method)
	}
	if got := req.header.Get("Content-Type"); got != "application/json" {
		t.Fatalf("content-type = %q, want application/json", got)
	}
	if !strings.Contains(req.body, `"ticket":"TICK-EXEC-001"`) || !strings.Contains(req.body, `"env":"prod"`) || !strings.Contains(req.body, `"requester":"7"`) {
		t.Fatalf("unexpected request body: %s", req.body)
	}

	var executions []actionExecutionModel
	if err := db.Find(&executions).Error; err != nil {
		t.Fatalf("list executions: %v", err)
	}
	if len(executions) != 1 {
		t.Fatalf("execution count = %d, want 1", len(executions))
	}
	if executions[0].Status != "success" || executions[0].RetryCount != 0 {
		t.Fatalf("unexpected execution row: %+v", executions[0])
	}
	if !strings.Contains(executions[0].RequestPayload, `"ticket":"TICK-EXEC-001"`) || !strings.Contains(executions[0].ResponsePayload, `"ok":true`) {
		t.Fatalf("unexpected execution payloads: %+v", executions[0])
	}
}

func TestActionExecutorExecuteWithConfigRecordsHTTPFailures(t *testing.T) {
	db := setupActionExecutorDB(t)
	if err := db.Create(&ticketModel{
		ID:          1,
		Code:        "TICK-EXEC-FAIL",
		Status:      "decisioning",
		RequesterID: 9,
		PriorityID:  2,
	}).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	}))
	defer server.Close()

	configJSON := fmt.Sprintf(`{"url":%q,"method":"POST","body":"retry body","timeout":5,"retries":0}`, server.URL)
	executor := NewActionExecutor(db)
	_, _, err := executor.ExecuteWithConfig(context.Background(), 1, 3, 99, "http", configJSON)
	if err == nil || !strings.Contains(err.Error(), "HTTP 500") {
		t.Fatalf("expected HTTP 500 error, got %v", err)
	}

	var executions []actionExecutionModel
	if err := db.Order("id asc").Find(&executions).Error; err != nil {
		t.Fatalf("list executions: %v", err)
	}
	if len(executions) != 1 {
		t.Fatalf("execution count = %d, want 1", len(executions))
	}
	if executions[0].Status != "failed" || executions[0].FailureReason != "HTTP 500" {
		t.Fatalf("unexpected failure row: %+v", executions[0])
	}
	if executions[0].ResponsePayload != "boom" {
		t.Fatalf("response payload = %q, want boom", executions[0].ResponsePayload)
	}
}

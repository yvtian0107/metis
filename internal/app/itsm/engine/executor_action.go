package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"strings"
	"time"

	"gorm.io/gorm"
)

// ActionConfig represents the HTTP webhook configuration for a ServiceAction.
type ActionConfig struct {
	URL     string            `json:"url"`
	Method  string            `json:"method"`
	Headers map[string]string `json:"headers"`
	Body    string            `json:"body"`    // template with {{ticket.*}} variables
	Timeout int               `json:"timeout"` // seconds, default 30
	Retries int               `json:"retries"` // default 3
}

// ActionExecutor handles HTTP webhook execution for action nodes.
type ActionExecutor struct {
	db     *gorm.DB
	client *http.Client
}

func NewActionExecutor(db *gorm.DB) *ActionExecutor {
	return &ActionExecutor{
		db:     db,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

// Execute runs the HTTP webhook for an action task.
func (e *ActionExecutor) Execute(ctx context.Context, ticketID, activityID, actionID uint) error {
	// Load the service action config
	var action serviceActionModel
	if err := e.db.First(&action, actionID).Error; err != nil {
		return fmt.Errorf("action %d not found: %w", actionID, err)
	}

	var config ActionConfig
	if err := json.Unmarshal([]byte(action.ConfigJSON), &config); err != nil {
		return fmt.Errorf("invalid action config: %w", err)
	}
	if config.Method == "" {
		config.Method = "POST"
	}
	if config.Timeout == 0 {
		config.Timeout = 30
	}
	if config.Retries == 0 {
		config.Retries = 3
	}

	// Load ticket for template variable substitution
	var ticket ticketModel
	if err := e.db.First(&ticket, ticketID).Error; err != nil {
		return fmt.Errorf("ticket %d not found: %w", ticketID, err)
	}

	// Replace template variables in body
	body := replaceTemplateVars(config.Body, &ticket)

	// Execute with retry
	var lastErr error
	for attempt := 0; attempt <= config.Retries; attempt++ {
		if attempt > 0 {
			// Exponential backoff: 2^attempt seconds
			backoff := time.Duration(math.Pow(2, float64(attempt))) * time.Second
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
		}

		status, respBody, err := e.doHTTPRequest(ctx, config, body)
		execStatus := "success"
		failureReason := ""
		if err != nil {
			execStatus = "failed"
			failureReason = err.Error()
			lastErr = err
		} else if status < 200 || status >= 300 {
			execStatus = "failed"
			failureReason = fmt.Sprintf("HTTP %d", status)
			lastErr = fmt.Errorf("HTTP %d", status)
		} else {
			lastErr = nil
		}

		// Record execution
		exec := &actionExecutionModel{
			TicketID:        ticketID,
			ActivityID:      activityID,
			ServiceActionID: actionID,
			Status:          execStatus,
			RequestPayload:  body,
			ResponsePayload: respBody,
			FailureReason:   failureReason,
			RetryCount:      attempt,
		}
		e.db.Create(exec)

		if lastErr == nil {
			return nil
		}

		slog.Warn("action execution failed, retrying",
			"ticketID", ticketID, "actionID", actionID, "attempt", attempt, "error", lastErr)
	}

	return lastErr
}

func (e *ActionExecutor) doHTTPRequest(ctx context.Context, config ActionConfig, body string) (int, string, error) {
	client := &http.Client{Timeout: time.Duration(config.Timeout) * time.Second}

	var reqBody io.Reader
	if body != "" {
		reqBody = bytes.NewBufferString(body)
	}

	req, err := http.NewRequestWithContext(ctx, config.Method, config.URL, reqBody)
	if err != nil {
		return 0, "", err
	}

	for k, v := range config.Headers {
		req.Header.Set(k, v)
	}
	if req.Header.Get("Content-Type") == "" && reqBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024)) // 64KB limit
	return resp.StatusCode, string(respBytes), nil
}

func replaceTemplateVars(template string, ticket *ticketModel) string {
	if template == "" {
		return ""
	}
	pairs := []string{
		"{{ticket.id}}", fmt.Sprintf("%d", ticket.ID),
		"{{ticket.code}}", ticket.Code,
		"{{ticket.status}}", ticket.Status,
		"{{ticket.requester_id}}", fmt.Sprintf("%d", ticket.RequesterID),
		"{{ticket.priority_id}}", fmt.Sprintf("%d", ticket.PriorityID),
	}

	// Support {{ticket.form_data.<key>}} by parsing FormData JSON.
	if ticket.FormData != "" {
		var formData map[string]any
		if json.Unmarshal([]byte(ticket.FormData), &formData) == nil {
			for k, v := range formData {
				pairs = append(pairs, fmt.Sprintf("{{ticket.form_data.%s}}", k), fmt.Sprint(v))
			}
		}
	}

	return strings.NewReplacer(pairs...).Replace(template)
}

// --- DB model for action execution records ---

type serviceActionModel struct {
	ID         uint   `gorm:"primaryKey"`
	ConfigJSON string `gorm:"column:config_json;type:text"`
}

func (serviceActionModel) TableName() string { return "itsm_service_actions" }

type actionExecutionModel struct {
	ID              uint      `gorm:"primaryKey;autoIncrement"`
	TicketID        uint      `gorm:"column:ticket_id;not null"`
	ActivityID      uint      `gorm:"column:activity_id;not null"`
	ServiceActionID uint      `gorm:"column:service_action_id;not null"`
	Status          string    `gorm:"column:status;size:16;default:pending"`
	RequestPayload  string    `gorm:"column:request_payload;type:text"`
	ResponsePayload string    `gorm:"column:response_payload;type:text"`
	FailureReason   string    `gorm:"column:failure_reason;type:text"`
	RetryCount      int       `gorm:"column:retry_count;default:0"`
	CreatedAt       time.Time `gorm:"column:created_at;autoCreateTime"`
}

func (actionExecutionModel) TableName() string { return "itsm_ticket_action_executions" }

package testutil

import (
	"encoding/json"
	"fmt"
	. "metis/internal/app/itsm/domain"
	orgdomain "metis/internal/app/org/domain"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	ai "metis/internal/app/ai/runtime"
	"metis/internal/model"
)

func NewTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	gdb, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := gdb.AutoMigrate(
		&ServiceCatalog{},
		&ServiceDefinition{},
		&ServiceDefinitionVersion{},
		&ServiceAction{},
		&ServiceKnowledgeDocument{},
		&Priority{},
		&SLATemplate{},
		&EscalationRule{},
		&Ticket{},
		&TicketActivity{},
		&TicketAssignment{},
		&TicketTimeline{},
		&TicketActionExecution{},
		&ExecutionToken{},
		&ServiceDeskSubmission{},
		&ai.Agent{},
		&ai.AgentSession{},
		&ai.SessionMessage{},
		&ai.Provider{},
		&ai.AIModel{},
		&ai.Tool{},
		&ai.AgentTool{},
		&model.Menu{},
		&model.SystemConfig{},
		&model.User{},
		&model.TaskExecution{},
		&orgdomain.Department{},
		&orgdomain.Position{},
		&orgdomain.UserPosition{},
		&orgdomain.DepartmentPosition{},
	); err != nil {
		t.Fatalf("migrate test db: %v", err)
	}
	return gdb
}

func DecodeResponseBody[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return v
}

func NewGinContext(method, path string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(method, path, nil)
	return c, rec
}

func SeedSmartSubmissionService(t *testing.T, db *gorm.DB) ServiceDefinition {
	t.Helper()
	priority := Priority{Name: "P3", Code: "P3", Value: 3, Color: "#64748b", IsActive: true}
	if err := db.Create(&priority).Error; err != nil {
		t.Fatalf("create priority: %v", err)
	}
	catalog := ServiceCatalog{Name: "账号与权限", Code: "account", IsActive: true}
	if err := db.Create(&catalog).Error; err != nil {
		t.Fatalf("create catalog: %v", err)
	}
	agent := ai.Agent{Name: "流程决策智能体", Type: ai.AgentTypeAssistant, IsActive: true}
	if err := db.Create(&agent).Error; err != nil {
		t.Fatalf("create agent: %v", err)
	}
	service := ServiceDefinition{
		Name:              "VPN 开通申请",
		Code:              "vpn-access",
		CatalogID:         catalog.ID,
		EngineType:        "smart",
		CollaborationSpec: "收到申请后分配网络管理员处理。",
		AgentID:           &agent.ID,
		IsActive:          true,
	}
	if err := db.Create(&service).Error; err != nil {
		t.Fatalf("create service: %v", err)
	}
	return service
}

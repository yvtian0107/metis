package tools

import (
	"testing"

	"gorm.io/gorm"

	"metis/internal/app"
	"metis/internal/app/itsm/domain"
	"metis/internal/app/itsm/engine"
	"metis/internal/app/itsm/testutil"
	"metis/internal/model"
)

type testOrgResolver struct{}

func (testOrgResolver) GetUserDeptScope(uint, bool) ([]uint, error) { return nil, nil }
func (testOrgResolver) GetUserPositionIDs(uint) ([]uint, error)     { return nil, nil }
func (testOrgResolver) GetUserDepartmentIDs(uint) ([]uint, error)   { return nil, nil }
func (testOrgResolver) GetUserPositions(uint) ([]app.OrgPosition, error) {
	return nil, nil
}
func (testOrgResolver) GetUserDepartment(uint) (*app.OrgDepartment, error) { return nil, nil }
func (testOrgResolver) QueryContext(string, string, string, bool) (*app.OrgContextResult, error) {
	return nil, nil
}
func (testOrgResolver) FindUsersByPositionCode(string) ([]uint, error) { return nil, nil }
func (testOrgResolver) FindUsersByDepartmentCode(string) ([]uint, error) {
	return nil, nil
}
func (testOrgResolver) FindUsersByPositionAndDepartment(string, string) ([]uint, error) {
	return nil, nil
}
func (testOrgResolver) FindUsersByPositionID(uint) ([]uint, error)   { return nil, nil }
func (testOrgResolver) FindUsersByDepartmentID(uint) ([]uint, error) { return nil, nil }
func (testOrgResolver) FindManagerByUserID(uint) (uint, error)       { return 0, nil }

func TestValidateParticipants_OnlyChecksSelectedInitialRoute(t *testing.T) {
	db := testutil.NewTestDB(t)
	seedParticipantUser(t, db, 1, "network_admin", "it")
	op := NewOperator(db, engine.NewParticipantResolver(testOrgResolver{}), testOrgResolver{}, nil, nil, nil)
	service := seedRoutingService(t, db)

	result, err := op.ValidateParticipants(service.ID, map[string]any{"request_kind": "online_support"})
	if err != nil {
		t.Fatalf("validate participants: %v", err)
	}
	if result == nil || !result.OK {
		t.Fatalf("expected network branch to pass without security admin, got %+v", result)
	}
}

func TestValidateParticipants_FailsSelectedRouteWhenParticipantMissing(t *testing.T) {
	db := testutil.NewTestDB(t)
	seedParticipantUser(t, db, 1, "network_admin", "it")
	op := NewOperator(db, engine.NewParticipantResolver(testOrgResolver{}), testOrgResolver{}, nil, nil, nil)
	service := seedRoutingService(t, db)

	result, err := op.ValidateParticipants(service.ID, map[string]any{"request_kind": "security_compliance"})
	if err != nil {
		t.Fatalf("validate participants: %v", err)
	}
	if result == nil || result.OK {
		t.Fatalf("expected missing security branch participant to fail, got %+v", result)
	}
	if result.NodeLabel != "信息安全管理员处理" {
		t.Fatalf("expected failure on security branch, got %+v", result)
	}
}

func TestValidateParticipants_FailsWhenNoInitialRouteMatches(t *testing.T) {
	db := testutil.NewTestDB(t)
	seedParticipantUser(t, db, 1, "network_admin", "it")
	seedParticipantUser(t, db, 2, "security_admin", "it")
	op := NewOperator(db, engine.NewParticipantResolver(testOrgResolver{}), testOrgResolver{}, nil, nil, nil)
	service := seedRoutingService(t, db)

	result, err := op.ValidateParticipants(service.ID, map[string]any{"request_kind": "unknown"})
	if err != nil {
		t.Fatalf("validate participants: %v", err)
	}
	if result == nil || result.OK {
		t.Fatalf("expected unmatched initial route to fail, got %+v", result)
	}
}

func seedRoutingService(t *testing.T, db *gorm.DB) domain.ServiceDefinition {
	t.Helper()
	service := domain.ServiceDefinition{
		Name:         "VPN 开通申请",
		Code:         "vpn-routing-test",
		EngineType:   "smart",
		IsActive:     true,
		WorkflowJSON: []byte(routingWorkflowJSON),
	}
	if err := db.Create(&service).Error; err != nil {
		t.Fatalf("create service: %v", err)
	}
	return service
}

func seedParticipantUser(t *testing.T, db *gorm.DB, userID uint, positionCode string, departmentCode string) {
	t.Helper()
	user := model.User{Username: positionCode + "_user", IsActive: true}
	user.ID = userID
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	dept := struct {
		ID   uint
		Code string
		Name string
	}{Code: departmentCode, Name: departmentCode}
	if err := db.Table("departments").Where("code = ?", departmentCode).FirstOrCreate(&dept, map[string]any{"code": departmentCode}).Error; err != nil {
		t.Fatalf("create department: %v", err)
	}
	pos := struct {
		ID   uint
		Code string
		Name string
	}{Code: positionCode, Name: positionCode}
	if err := db.Table("positions").Where("code = ?", positionCode).FirstOrCreate(&pos, map[string]any{"code": positionCode}).Error; err != nil {
		t.Fatalf("create position: %v", err)
	}
	if err := db.Table("user_positions").Create(map[string]any{
		"user_id":       user.ID,
		"position_id":   pos.ID,
		"department_id": dept.ID,
	}).Error; err != nil {
		t.Fatalf("create user position: %v", err)
	}
}

const routingWorkflowJSON = `{
  "nodes": [
    {"id": "start", "type": "start", "data": {"label": "开始"}},
    {"id": "request", "type": "form", "data": {"label": "填写 VPN 开通申请", "participants": [{"type": "requester"}]}},
    {"id": "route", "type": "exclusive", "data": {"label": "访问原因路由"}},
    {"id": "network_process", "type": "process", "data": {"label": "网络管理员处理", "participants": [{"type": "position_department", "department_code": "it", "position_code": "network_admin"}]}},
    {"id": "security_process", "type": "process", "data": {"label": "信息安全管理员处理", "participants": [{"type": "position_department", "department_code": "it", "position_code": "security_admin"}]}},
    {"id": "end", "type": "end", "data": {"label": "结束"}}
  ],
  "edges": [
    {"id": "edge_start_request", "source": "start", "target": "request"},
    {"id": "edge_request_route", "source": "request", "target": "route"},
    {"id": "edge_route_network", "source": "route", "target": "network_process", "data": {"condition": {"field": "form.request_kind", "operator": "contains_any", "value": ["online_support", "troubleshooting"], "edge_id": "edge_route_network"}}},
    {"id": "edge_route_security", "source": "route", "target": "security_process", "data": {"condition": {"field": "form.request_kind", "operator": "contains_any", "value": ["security_compliance", "external_collaboration"], "edge_id": "edge_route_security"}}},
    {"id": "edge_network_end", "source": "network_process", "target": "end"},
    {"id": "edge_security_end", "source": "security_process", "target": "end"}
  ]
}`

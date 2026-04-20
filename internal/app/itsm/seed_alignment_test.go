package itsm

import (
	"strings"
	"testing"

	"github.com/casbin/casbin/v2"
	casbinmodel "github.com/casbin/casbin/v2/model"
	"github.com/casbin/casbin/v2/persist"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	aiapp "metis/internal/app/ai"
	org "metis/internal/app/org"
	coremodel "metis/internal/model"
)

type noopAdapter struct{}

func (noopAdapter) LoadPolicy(casbinmodel.Model) error                        { return nil }
func (noopAdapter) SavePolicy(casbinmodel.Model) error                        { return nil }
func (noopAdapter) AddPolicy(string, string, []string) error                  { return nil }
func (noopAdapter) RemovePolicy(string, string, []string) error               { return nil }
func (noopAdapter) RemoveFilteredPolicy(string, string, int, ...string) error { return nil }

var _ persist.Adapter = (*noopAdapter)(nil)

func newSeedAlignmentDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(
		&ServiceCatalog{}, &ServiceDefinition{}, &ServiceAction{}, &Priority{}, &SLATemplate{},
		&org.Department{}, &org.Position{}, &org.DepartmentPosition{}, &org.UserPosition{},
		&coremodel.User{}, &coremodel.Role{}, &coremodel.Menu{}, &coremodel.SystemConfig{},
		&aiapp.Agent{}, &aiapp.Tool{}, &aiapp.AgentTool{},
	); err != nil {
		t.Fatalf("migrate db: %v", err)
	}
	for _, name := range []string{
		"general.current_time",
		"system.current_user_profile",
	} {
		tool := aiapp.Tool{
			Toolkit:     "test",
			Name:        name,
			DisplayName: name,
			Description: "test seed tool",
			IsActive:    true,
		}
		if err := db.Where(aiapp.Tool{Name: name}).FirstOrCreate(&tool).Error; err != nil {
			t.Fatalf("seed tool %s: %v", name, err)
		}
	}
	return db
}

func newTestEnforcer(t *testing.T) *casbin.Enforcer {
	t.Helper()
	m, err := casbinmodel.NewModelFromString(`[request_definition]
r = sub, obj, act
[policy_definition]
p = sub, obj, act
[role_definition]
g = _, _
[policy_effect]
e = some(where (p.eft == allow))
[matchers]
m = r.sub == p.sub && r.obj == p.obj && r.act == p.act
`)
	if err != nil {
		t.Fatalf("create casbin model: %v", err)
	}
	e, err := casbin.NewEnforcer(m, &noopAdapter{})
	if err != nil {
		t.Fatalf("create casbin enforcer: %v", err)
	}
	return e
}

func TestBuiltInSmartSeedsAlignParticipantsAndInstallAdminIdentity(t *testing.T) {
	db := newSeedAlignmentDB(t)
	enforcer := newTestEnforcer(t)

	adminRole := coremodel.Role{Name: "Admin", Code: coremodel.RoleAdmin}
	if err := db.Create(&adminRole).Error; err != nil {
		t.Fatalf("create admin role: %v", err)
	}
	admin := coremodel.User{Username: "admin", IsActive: true, RoleID: adminRole.ID}
	if err := db.Create(&admin).Error; err != nil {
		t.Fatalf("create admin user: %v", err)
	}

	var orgApp org.OrgApp
	if err := orgApp.Seed(db, enforcer, true); err != nil {
		t.Fatalf("seed org: %v", err)
	}
	var itsmApp ITSMApp
	if err := itsmApp.Seed(db, enforcer, true); err != nil {
		t.Fatalf("seed itsm: %v", err)
	}

	var dept org.Department
	if err := db.Where("code = ?", "it").First(&dept).Error; err != nil {
		t.Fatalf("load it dept: %v", err)
	}
	var pos org.Position
	if err := db.Where("code = ?", "it_admin").First(&pos).Error; err != nil {
		t.Fatalf("load it_admin: %v", err)
	}
	if err := db.Create(&org.UserPosition{UserID: admin.ID, DepartmentID: dept.ID, PositionID: pos.ID, IsPrimary: true}).Error; err != nil {
		t.Fatalf("assign admin identity: %v", err)
	}

	t.Run("org positions include required built-ins", func(t *testing.T) {
		for _, code := range []string{"it_admin", "db_admin", "network_admin", "security_admin", "ops_admin", "serial_reviewer"} {
			var count int64
			if err := db.Model(&org.Position{}).Where("code = ?", code).Count(&count).Error; err != nil {
				t.Fatalf("count position %s: %v", code, err)
			}
			if count != 1 {
				t.Fatalf("expected position %s to exist once, got %d", code, count)
			}
		}
	})

	t.Run("it department allows ops admin", func(t *testing.T) {
		var dept org.Department
		if err := db.Where("code = ?", "it").First(&dept).Error; err != nil {
			t.Fatalf("load it dept: %v", err)
		}
		var pos org.Position
		if err := db.Where("code = ?", "ops_admin").First(&pos).Error; err != nil {
			t.Fatalf("load ops_admin: %v", err)
		}
		var count int64
		if err := db.Model(&org.DepartmentPosition{}).Where("department_id = ? AND position_id = ?", dept.ID, pos.ID).Count(&count).Error; err != nil {
			t.Fatalf("count dept-position: %v", err)
		}
		if count != 1 {
			t.Fatalf("expected ops_admin to be allowed in it, got %d", count)
		}
	})

	t.Run("built-in smart services reference aligned participant codes", func(t *testing.T) {
		var services []ServiceDefinition
		if err := db.Where("engine_type = ?", "smart").Find(&services).Error; err != nil {
			t.Fatalf("load smart services: %v", err)
		}
		wanted := map[string][]string{
			"boss-serial-change-request":     {"serial-reviewer", "ops_admin"},
			"db-backup-whitelist-action-e2e": {"db_admin"},
			"prod-server-temporary-access":   {"ops_admin", "network_admin", "security_admin"},
			"vpn-access-request":             {"network_admin", "security_admin"},
			"copilot-account-request":        {"IT管理员"},
		}
		for _, svc := range services {
			needles, ok := wanted[svc.Code]
			if !ok {
				continue
			}
			for _, needle := range needles {
				if !strings.Contains(svc.CollaborationSpec, needle) {
					t.Fatalf("service %s missing participant marker %q in collaboration spec", svc.Code, needle)
				}
			}
			if strings.Contains(svc.CollaborationSpec, "dba_admin") {
				t.Fatalf("service %s should not reference legacy dba_admin code", svc.Code)
			}
		}
	})

	t.Run("decision agent gets required tool bindings", func(t *testing.T) {
		var agent aiapp.Agent
		if err := db.Where("name = ?", "流程决策智能体").First(&agent).Error; err != nil {
			t.Fatalf("load decision agent: %v", err)
		}

		var tools []aiapp.Tool
		if err := db.Table("ai_tools").
			Joins("JOIN ai_agent_tools ON ai_agent_tools.tool_id = ai_tools.id").
			Where("ai_agent_tools.agent_id = ?", agent.ID).
			Find(&tools).Error; err != nil {
			t.Fatalf("load decision agent tools: %v", err)
		}

		have := map[string]bool{}
		for _, tool := range tools {
			have[tool.Name] = true
		}
		for _, name := range []string{
			"decision.ticket_context",
			"decision.knowledge_search",
			"decision.resolve_participant",
			"decision.user_workload",
			"decision.similar_history",
			"decision.sla_status",
			"decision.list_actions",
			"decision.execute_action",
		} {
			if !have[name] {
				t.Fatalf("expected decision agent to bind tool %s", name)
			}
		}
	})

	t.Run("install admin gets it_admin identity", func(t *testing.T) {
		var count int64
		if err := db.Table("user_positions").Where("user_id = ? AND department_id = ? AND position_id = ? AND is_primary = ?", admin.ID, dept.ID, pos.ID, true).Count(&count).Error; err != nil {
			t.Fatalf("count admin user position: %v", err)
		}
		if count != 1 {
			t.Fatalf("expected admin to have primary it/it_admin identity, got %d", count)
		}
	})
}

package support

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/casbin/casbin/v2"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/samber/do/v2"
	"gorm.io/gorm"

	"metis/internal/app"
	ai "metis/internal/app/ai/runtime"
	"metis/internal/app/itsm/definition"
	"metis/internal/app/itsm/domain"
	"metis/internal/app/itsm/engine"
	"metis/internal/app/itsm/sla"
	"metis/internal/app/itsm/ticket"
	org "metis/internal/app/org/domain"
	casbinx "metis/internal/casbin"
	"metis/internal/database"
	"metis/internal/handler"
	"metis/internal/middleware"
	"metis/internal/model"
	pkgtoken "metis/internal/pkg/token"
	"metis/internal/repository"
	"metis/internal/service"
)

const defaultActorPassword = "Password123!"

// Context is the shared API-level BDD harness. All API steps should depend on
// this object instead of hand-rolling users, tokens, or httptest requests.
type Context struct {
	DB        *gorm.DB
	Router    *gin.Engine
	Enforcer  *casbin.Enforcer
	JWTSecret []byte

	Actors map[string]*Actor
	Client *APIClient

	CurrentTicketID   uint
	CurrentActivityID uint
	LastResponse      *APIResponse
}

// Actor is a business persona used by API BDD scenarios.
type Actor struct {
	Label    string
	Username string
	RoleCode string
	User     *model.User
	Token    string
}

type APIResponse struct {
	StatusCode int
	Code       int             `json:"code"`
	Message    string          `json:"message"`
	Data       json.RawMessage `json:"data"`
	RawBody    string
}

type APIClient struct {
	ctx *Context
}

func NewContext() *Context {
	return &Context{JWTSecret: []byte("itsm-bdd-api-secret")}
}

func (c *Context) Reset() error {
	gin.SetMode(gin.TestMode)
	c.Actors = make(map[string]*Actor)
	c.CurrentTicketID = 0
	c.CurrentActivityID = 0
	c.LastResponse = nil

	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:itsm_api_bdd_%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("open api bdd db: %w", err)
	}
	if err := c.migrate(db); err != nil {
		return err
	}
	c.DB = db

	enforcer, err := casbinx.NewEnforcerWithDB(db)
	if err != nil {
		return fmt.Errorf("create casbin enforcer: %w", err)
	}
	c.Enforcer = enforcer

	injector, err := c.buildInjector()
	if err != nil {
		return err
	}
	if err := c.seedRolesAndPolicies(); err != nil {
		return err
	}
	c.Router = c.buildRouter(injector)
	c.Client = &APIClient{ctx: c}
	return nil
}

func (c *Context) migrate(db *gorm.DB) error {
	if err := database.AutoMigrateKernel(db); err != nil {
		return fmt.Errorf("migrate kernel models: %w", err)
	}
	if err := db.AutoMigrate(
		&org.Department{}, &org.Position{}, &org.UserPosition{},
		&ai.Agent{}, &ai.AgentSession{},
		&domain.ServiceCatalog{}, &domain.ServiceDefinition{}, &domain.ServiceAction{},
		&domain.Priority{}, &domain.SLATemplate{}, &domain.EscalationRule{},
		&domain.Ticket{}, &domain.TicketActivity{}, &domain.TicketAssignment{},
		&domain.TicketTimeline{}, &domain.TicketActionExecution{}, &domain.ServiceDeskSubmission{},
		&domain.TicketLink{}, &domain.PostMortem{}, &domain.ProcessVariable{},
		&domain.ExecutionToken{}, &domain.ServiceKnowledgeDocument{},
		&model.AuditLog{},
	); err != nil {
		return fmt.Errorf("migrate itsm api bdd models: %w", err)
	}
	return nil
}

func (c *Context) buildInjector() (do.Injector, error) {
	injector := do.New()
	wrapped := &database.DB{DB: c.DB}
	do.ProvideValue(injector, wrapped)
	do.ProvideValue[[]byte](injector, c.JWTSecret)
	do.ProvideValue(injector, pkgtoken.NewBlacklist())
	do.ProvideValue[app.OrgResolver](injector, &OrgResolver{db: c.DB})

	do.Provide(injector, repository.NewRole)
	do.Provide(injector, repository.NewSysConfig)
	do.Provide(injector, repository.NewAuditLog)
	do.Provide(injector, service.NewSettings)
	do.Provide(injector, service.NewAuditLog)

	resolver := engine.NewParticipantResolver(&OrgResolver{db: c.DB})
	do.ProvideValue(injector, resolver)
	do.ProvideValue(injector, engine.NewClassicEngine(resolver, &noopSubmitter{}, nil))
	do.ProvideValue(injector, engine.NewSmartEngine(noopDecisionExecutor{}, nil, &UserProvider{db: c.DB}, resolver, &noopSubmitter{}, nil))

	do.Provide(injector, definition.NewServiceDefRepo)
	do.Provide(injector, sla.NewPriorityRepo)
	do.Provide(injector, sla.NewSLATemplateRepo)
	do.Provide(injector, ticket.NewTicketRepo)
	do.Provide(injector, ticket.NewTimelineRepo)
	do.Provide(injector, ticket.NewTicketService)
	do.Provide(injector, ticket.NewTimelineService)
	do.Provide(injector, ticket.NewTicketHandler)
	return injector, nil
}

func (c *Context) buildRouter(injector do.Injector) *gin.Engine {
	r := gin.New()
	api := r.Group("/api/v1")
	roleScope := func(string) (model.DataScope, []uint, error) { return model.DataScopeAll, nil, nil }
	auditSvc := do.MustInvoke[*service.AuditLogService](injector)
	api.Use(middleware.JWTAuth(c.JWTSecret, pkgtoken.NewBlacklist()))
	api.Use(middleware.PasswordExpiry(func() int { return 0 }))
	api.Use(middleware.CasbinAuth(c.Enforcer))
	api.Use(middleware.DataScopeMiddleware(nil, roleScope))
	api.Use(middleware.Audit(auditSvc))

	ticketH := do.MustInvoke[*ticket.TicketHandler](injector)
	g := api.Group("/itsm")
	g.GET("/tickets/approvals/pending", ticketH.PendingApprovals)
	g.GET("/tickets/approvals/history", ticketH.ApprovalHistory)
	g.GET("/tickets/:id", ticketH.Get)
	g.GET("/tickets/:id/timeline", ticketH.Timeline)
	g.POST("/tickets/:id/claim", ticketH.Claim)
	g.POST("/tickets/:id/progress", ticketH.Progress)
	return r
}

func (c *Context) seedRolesAndPolicies() error {
	roles := []model.Role{
		{Name: "管理员", Code: model.RoleAdmin, IsSystem: true, DataScope: model.DataScopeAll},
		{Name: "普通用户", Code: model.RoleUser, IsSystem: true, DataScope: model.DataScopeAll},
	}
	for i := range roles {
		if err := c.DB.Where("code = ?", roles[i].Code).FirstOrCreate(&roles[i]).Error; err != nil {
			return fmt.Errorf("seed role %s: %w", roles[i].Code, err)
		}
	}
	for _, role := range []string{model.RoleAdmin, model.RoleUser} {
		_, _ = c.Enforcer.AddGroupingPolicy(role, role)
		for _, policy := range apiPolicies() {
			_, _ = c.Enforcer.AddPolicy(role, policy.path, policy.method)
		}
	}
	return nil
}

func apiPolicies() []struct{ path, method string } {
	return []struct{ path, method string }{
		{"/api/v1/itsm/tickets/approvals/pending", http.MethodGet},
		{"/api/v1/itsm/tickets/approvals/history", http.MethodGet},
		{"/api/v1/itsm/tickets/:id", http.MethodGet},
		{"/api/v1/itsm/tickets/:id/timeline", http.MethodGet},
		{"/api/v1/itsm/tickets/:id/claim", http.MethodPost},
		{"/api/v1/itsm/tickets/:id/progress", http.MethodPost},
	}
}

func (c *Context) EnsureDefaultActors() error {
	if len(c.Actors) > 0 {
		return nil
	}
	if err := c.ensureOrg("it", "信息部", "network_admin", "网络管理员"); err != nil {
		return err
	}
	if err := c.ensureOrg("security", "安全部", "security_admin", "安全管理员"); err != nil {
		return err
	}
	actors := []struct {
		label    string
		username string
		roleCode string
		deptCode string
		posCode  string
	}{
		{"申请人", "api-requester", model.RoleUser, "it", "staff"},
		{"网络管理员", "api-network-admin", model.RoleUser, "it", "network_admin"},
		{"安全管理员", "api-security-admin", model.RoleUser, "security", "security_admin"},
		{"管理员", "api-admin", model.RoleAdmin, "it", "network_admin"},
	}
	if err := c.ensureOrg("it", "信息部", "staff", "员工"); err != nil {
		return err
	}
	for _, a := range actors {
		actor, err := c.createActor(a.label, a.username, a.roleCode, a.deptCode, a.posCode)
		if err != nil {
			return err
		}
		c.Actors[a.label] = actor
	}
	return nil
}

func (c *Context) createActor(label, username, roleCode, deptCode, posCode string) (*Actor, error) {
	var role model.Role
	if err := c.DB.Where("code = ?", roleCode).First(&role).Error; err != nil {
		return nil, fmt.Errorf("find role %s: %w", roleCode, err)
	}
	hashed, err := pkgtoken.HashPassword(defaultActorPassword)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	user := &model.User{Username: username, Password: hashed, Email: username + "@example.test", RoleID: role.ID, IsActive: true, PasswordChangedAt: &now}
	if err := c.DB.Create(user).Error; err != nil {
		return nil, fmt.Errorf("create actor %s: %w", label, err)
	}
	if err := c.assignOrg(user.ID, deptCode, posCode); err != nil {
		return nil, err
	}
	actor := &Actor{Label: label, Username: username, RoleCode: roleCode, User: user}
	if err := c.LoginAs(actor); err != nil {
		return nil, err
	}
	return actor, nil
}

func (c *Context) LoginAs(actor *Actor) error {
	accessToken, _, err := pkgtoken.GenerateAccessToken(actor.User.ID, actor.RoleCode, c.JWTSecret, pkgtoken.WithPasswordMeta(actor.User.PasswordChangedAt, actor.User.ForcePasswordReset))
	if err != nil {
		return fmt.Errorf("generate actor token: %w", err)
	}
	actor.Token = accessToken
	return nil
}

func (c *Context) SeedAssignedSmartTicket(assigneeLabel string) error {
	if err := c.EnsureDefaultActors(); err != nil {
		return err
	}
	assignee, ok := c.Actors[assigneeLabel]
	if !ok {
		return fmt.Errorf("actor %q not found", assigneeLabel)
	}
	requester := c.Actors["申请人"]
	priority := domain.Priority{Name: "普通", Code: fmt.Sprintf("api-normal-%d", time.Now().UnixNano()), Value: 3, Color: "#64748b", IsActive: true}
	if err := c.DB.Create(&priority).Error; err != nil {
		return fmt.Errorf("create priority: %w", err)
	}
	catalog := domain.ServiceCatalog{Name: "API BDD 服务目录", Code: fmt.Sprintf("api-bdd-%d", time.Now().UnixNano()), IsActive: true}
	if err := c.DB.Create(&catalog).Error; err != nil {
		return fmt.Errorf("create catalog: %w", err)
	}
	svc := domain.ServiceDefinition{Name: "API BDD 智能服务", Code: fmt.Sprintf("api-bdd-smart-%d", time.Now().UnixNano()), CatalogID: catalog.ID, EngineType: "smart", IsActive: true, CollaborationSpec: "收到申请后交由网络管理员处理。"}
	if err := c.DB.Create(&svc).Error; err != nil {
		return fmt.Errorf("create service: %w", err)
	}
	t := domain.Ticket{Code: fmt.Sprintf("API-BDD-%d", time.Now().UnixNano()), Title: "API BDD 多角色审批", ServiceID: svc.ID, EngineType: "smart", Status: domain.TicketStatusWaitingHuman, PriorityID: priority.ID, RequesterID: requester.User.ID, Source: domain.TicketSourceCatalog, FormData: domain.JSONField(`{"reason":"api-bdd"}`)}
	if err := c.DB.Create(&t).Error; err != nil {
		return fmt.Errorf("create ticket: %w", err)
	}
	act := domain.TicketActivity{TicketID: t.ID, Name: "人工处理", ActivityType: engine.NodeProcess, Status: engine.ActivityPending, AIReasoning: "API BDD seed"}
	if err := c.DB.Create(&act).Error; err != nil {
		return fmt.Errorf("create activity: %w", err)
	}
	assignment := domain.TicketAssignment{TicketID: t.ID, ActivityID: act.ID, ParticipantType: "user", UserID: &assignee.User.ID, Status: domain.AssignmentPending, IsCurrent: true}
	if err := c.DB.Create(&assignment).Error; err != nil {
		return fmt.Errorf("create assignment: %w", err)
	}
	if err := c.DB.Model(&domain.Ticket{}).Where("id = ?", t.ID).Update("current_activity_id", act.ID).Error; err != nil {
		return fmt.Errorf("update current activity: %w", err)
	}
	c.CurrentTicketID = t.ID
	c.CurrentActivityID = act.ID
	return nil
}

func (c *Context) ensureOrg(deptCode, deptName, posCode, posName string) error {
	if strings.TrimSpace(deptCode) != "" {
		dept := org.Department{Code: deptCode, Name: deptName, IsActive: true}
		if err := c.DB.Where("code = ?", deptCode).FirstOrCreate(&dept).Error; err != nil {
			return fmt.Errorf("seed dept %s: %w", deptCode, err)
		}
	}
	if strings.TrimSpace(posCode) != "" {
		pos := org.Position{Code: posCode, Name: posName, IsActive: true}
		if err := c.DB.Where("code = ?", posCode).FirstOrCreate(&pos).Error; err != nil {
			return fmt.Errorf("seed position %s: %w", posCode, err)
		}
	}
	return nil
}

func (c *Context) assignOrg(userID uint, deptCode, posCode string) error {
	var dept org.Department
	if err := c.DB.Where("code = ?", deptCode).First(&dept).Error; err != nil {
		return fmt.Errorf("find dept %s: %w", deptCode, err)
	}
	var pos org.Position
	if err := c.DB.Where("code = ?", posCode).First(&pos).Error; err != nil {
		return fmt.Errorf("find position %s: %w", posCode, err)
	}
	up := org.UserPosition{UserID: userID, DepartmentID: dept.ID, PositionID: pos.ID, IsPrimary: true}
	return c.DB.Create(&up).Error
}

func (c *APIClient) Do(actorLabel, method, path string, body any) (*APIResponse, error) {
	actor, ok := c.ctx.Actors[actorLabel]
	if !ok {
		return nil, fmt.Errorf("actor %q not found", actorLabel)
	}
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Authorization", "Bearer "+actor.Token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	c.ctx.Router.ServeHTTP(rec, req)
	resp := &APIResponse{StatusCode: rec.Code, RawBody: rec.Body.String()}
	if strings.TrimSpace(resp.RawBody) != "" {
		if err := json.Unmarshal(rec.Body.Bytes(), resp); err != nil {
			return nil, fmt.Errorf("decode api response status=%d body=%s: %w", rec.Code, resp.RawBody, err)
		}
	}
	c.ctx.LastResponse = resp
	return resp, nil
}

func (c *Context) LastDataObject() (map[string]json.RawMessage, error) {
	if c.LastResponse == nil || len(c.LastResponse.Data) == 0 {
		return nil, fmt.Errorf("last response has no data: %#v", c.LastResponse)
	}
	var data map[string]json.RawMessage
	if err := json.Unmarshal(c.LastResponse.Data, &data); err != nil {
		return nil, fmt.Errorf("decode last response data: %w", err)
	}
	return data, nil
}

func (c *Context) LoadTicketViaAPI(actorLabel string) (map[string]json.RawMessage, error) {
	_, err := c.Client.Do(actorLabel, http.MethodGet, fmt.Sprintf("/api/v1/itsm/tickets/%d", c.CurrentTicketID), nil)
	if err != nil {
		return nil, err
	}
	return c.LastDataObject()
}

func DecodeField[T any](obj map[string]json.RawMessage, field string) (T, error) {
	var zero T
	raw, ok := obj[field]
	if !ok {
		return zero, fmt.Errorf("field %q missing", field)
	}
	if err := json.Unmarshal(raw, &zero); err != nil {
		return zero, fmt.Errorf("decode field %q: %w", field, err)
	}
	return zero, nil
}

var _ = handler.R{}

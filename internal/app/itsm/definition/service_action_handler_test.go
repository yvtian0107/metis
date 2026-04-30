package definition

import (
	. "metis/internal/app/itsm/domain"
	"metis/internal/database"
	"net/http"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func newServiceActionServiceForTest(t *testing.T, db *gorm.DB, serviceDefs *ServiceDefService) *ServiceActionService {
	t.Helper()
	return &ServiceActionService{
		repo:        &ServiceActionRepo{db: &database.DB{DB: db}},
		serviceDefs: serviceDefs,
	}
}

func TestServiceActionHandlerUpdateAndDelete_RequireParentServiceMatch(t *testing.T) {
	db := newTestDB(t)
	catSvc := newCatalogServiceForTest(t, db)
	serviceDefs := newServiceDefServiceForTest(t, db)
	actionSvc := newServiceActionServiceForTest(t, db, serviceDefs)
	h := &ServiceActionHandler{svc: actionSvc}

	root, err := catSvc.Create("Root", "root", "", "", nil, 10)
	if err != nil {
		t.Fatalf("create root: %v", err)
	}
	serviceA, err := serviceDefs.Create(&ServiceDefinition{Name: "A", Code: "svc-a", CatalogID: root.ID, EngineType: "smart", CollaborationSpec: "spec"})
	if err != nil {
		t.Fatalf("create service A: %v", err)
	}
	serviceB, err := serviceDefs.Create(&ServiceDefinition{Name: "B", Code: "svc-b", CatalogID: root.ID, EngineType: "smart", CollaborationSpec: "spec"})
	if err != nil {
		t.Fatalf("create service B: %v", err)
	}
	actionB, err := actionSvc.Create(&ServiceAction{
		Name:       "Notify",
		Code:       "notify",
		ActionType: "http",
		ConfigJSON: JSONField(`{"url":"https://example.com/hook","method":"POST","timeout":30,"retries":3}`),
		ServiceID:  serviceB.ID,
	})
	if err != nil {
		t.Fatalf("create action B: %v", err)
	}

	updatePath := "/services/" + strconv.FormatUint(uint64(serviceA.ID), 10) + "/actions/" + strconv.FormatUint(uint64(actionB.ID), 10)
	rec := performJSONRequest(t, func(r *gin.Engine) {
		r.PUT("/services/:id/actions/:actionId", h.Update)
	}, http.MethodPut, updatePath, []byte(`{"name":"cross-service-update"}`))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected cross-service update to return 404, got %d body=%s", rec.Code, rec.Body.String())
	}

	deletePath := "/services/" + strconv.FormatUint(uint64(serviceA.ID), 10) + "/actions/" + strconv.FormatUint(uint64(actionB.ID), 10)
	rec = performJSONRequest(t, func(r *gin.Engine) {
		r.DELETE("/services/:id/actions/:actionId", h.Delete)
	}, http.MethodDelete, deletePath, nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected cross-service delete to return 404, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestServiceActionHandlerCreate_ValidatesHTTPConfig(t *testing.T) {
	db := newTestDB(t)
	catSvc := newCatalogServiceForTest(t, db)
	serviceDefs := newServiceDefServiceForTest(t, db)
	actionSvc := newServiceActionServiceForTest(t, db, serviceDefs)
	h := &ServiceActionHandler{svc: actionSvc}

	root, err := catSvc.Create("Root", "root", "", "", nil, 10)
	if err != nil {
		t.Fatalf("create root: %v", err)
	}
	service, err := serviceDefs.Create(&ServiceDefinition{Name: "Webhook", Code: "webhook", CatalogID: root.ID, EngineType: "smart", CollaborationSpec: "spec"})
	if err != nil {
		t.Fatalf("create service: %v", err)
	}

	tests := []struct {
		name string
		body string
	}{
		{name: "non-http action type", body: `{"name":"Bad","code":"bad-type","actionType":"shell","configJson":{"url":"https://example.com"}}`},
		{name: "invalid url", body: `{"name":"Bad","code":"bad-url","actionType":"http","configJson":{"url":"ftp://example.com"}}`},
		{name: "invalid method", body: `{"name":"Bad","code":"bad-method","actionType":"http","configJson":{"url":"https://example.com","method":"TRACE"}}`},
		{name: "timeout too large", body: `{"name":"Bad","code":"bad-timeout","actionType":"http","configJson":{"url":"https://example.com","timeout":121}}`},
		{name: "retries too large", body: `{"name":"Bad","code":"bad-retries","actionType":"http","configJson":{"url":"https://example.com","retries":6}}`},
		{name: "header injection", body: `{"name":"Bad","code":"bad-header","actionType":"http","configJson":{"url":"https://example.com","headers":{"X-Test":"ok\nbad"}}}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := "/services/" + strconv.FormatUint(uint64(service.ID), 10) + "/actions"
			rec := performJSONRequest(t, func(r *gin.Engine) {
				r.POST("/services/:id/actions", h.Create)
			}, http.MethodPost, path, []byte(tt.body))
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
			}
		})
	}
}

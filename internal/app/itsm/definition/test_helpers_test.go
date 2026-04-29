package definition

import (
	"bytes"
	"github.com/gin-gonic/gin"
	appcore "metis/internal/app"
	"metis/internal/app/itsm/engine"
	"net/http/httptest"
	"testing"

	"github.com/samber/do/v2"
	"gorm.io/gorm"

	"metis/internal/app/itsm/catalog"
	"metis/internal/app/itsm/testutil"
	"metis/internal/database"
)

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	return testutil.NewTestDB(t)
}

func newServiceDefServiceForTest(t *testing.T, db *gorm.DB) *ServiceDefService {
	t.Helper()
	wrapped := &database.DB{DB: db}
	injector := do.New()
	do.ProvideValue(injector, wrapped)
	catalogRepo, err := catalog.NewCatalogRepo(injector)
	if err != nil {
		t.Fatalf("create catalog repo: %v", err)
	}
	return &ServiceDefService{
		repo:             &ServiceDefRepo{db: wrapped},
		db:               wrapped,
		catalogs:         catalogRepo,
		llmClientFactory: nil,
		resolver:         engine.NewParticipantResolver(testServiceDefOrgResolver{}),
	}
}

func newCatalogServiceForTest(t *testing.T, db *gorm.DB) *catalog.CatalogService {
	t.Helper()
	injector := do.New()
	do.ProvideValue(injector, &database.DB{DB: db})
	do.Provide(injector, catalog.NewCatalogRepo)
	do.Provide(injector, catalog.NewCatalogService)
	return do.MustInvoke[*catalog.CatalogService](injector)
}

func decodeResponseBody[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	return testutil.DecodeResponseBody[T](t, rec)
}

func newGinContext(method, path string) (*gin.Context, *httptest.ResponseRecorder) {
	return testutil.NewGinContext(method, path)
}

func performJSONRequest(t *testing.T, routes func(*gin.Engine), method, path string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	routes(r)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(rec, req)
	return rec
}

type testServiceDefOrgResolver struct{}

func (testServiceDefOrgResolver) GetUserDeptScope(uint, bool) ([]uint, error) { return nil, nil }
func (testServiceDefOrgResolver) GetUserPositionIDs(uint) ([]uint, error)     { return nil, nil }
func (testServiceDefOrgResolver) GetUserDepartmentIDs(uint) ([]uint, error)   { return nil, nil }
func (testServiceDefOrgResolver) GetUserPositions(uint) ([]appcore.OrgPosition, error) {
	return nil, nil
}
func (testServiceDefOrgResolver) GetUserDepartment(uint) (*appcore.OrgDepartment, error) {
	return nil, nil
}
func (testServiceDefOrgResolver) QueryContext(string, string, string, bool) (*appcore.OrgContextResult, error) {
	return nil, nil
}
func (testServiceDefOrgResolver) FindUsersByPositionCode(string) ([]uint, error)   { return nil, nil }
func (testServiceDefOrgResolver) FindUsersByDepartmentCode(string) ([]uint, error) { return nil, nil }
func (testServiceDefOrgResolver) FindUsersByPositionAndDepartment(string, string) ([]uint, error) {
	return nil, nil
}
func (testServiceDefOrgResolver) FindUsersByPositionID(uint) ([]uint, error)   { return nil, nil }
func (testServiceDefOrgResolver) FindUsersByDepartmentID(uint) ([]uint, error) { return nil, nil }
func (testServiceDefOrgResolver) FindManagerByUserID(uint) (uint, error)       { return 0, nil }

var _ appcore.OrgResolver = testServiceDefOrgResolver{}

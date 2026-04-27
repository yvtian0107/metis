package definition

import (
	"bytes"
	"github.com/gin-gonic/gin"
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
		repo:     &ServiceDefRepo{db: wrapped},
		db:       wrapped,
		catalogs: catalogRepo,
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

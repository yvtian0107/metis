package catalog

import (
	"net/http/httptest"
	"testing"

	"gorm.io/gorm"

	"metis/internal/app/itsm/testutil"
	"metis/internal/database"
)

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	return testutil.NewTestDB(t)
}

func newCatalogServiceForTest(t *testing.T, db *gorm.DB) *CatalogService {
	t.Helper()
	return &CatalogService{repo: &CatalogRepo{db: &database.DB{DB: db}}}
}

func decodeResponseBody[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	return testutil.DecodeResponseBody[T](t, rec)
}

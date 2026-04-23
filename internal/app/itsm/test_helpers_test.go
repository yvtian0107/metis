package itsm

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"metis/internal/app/ai"
	"metis/internal/database"
	"metis/internal/model"
)

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	gdb, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := gdb.AutoMigrate(
		&ServiceCatalog{},
		&ServiceDefinition{},
		&ServiceAction{},
		&ServiceKnowledgeDocument{},
		&ai.Agent{},
		&ai.Provider{},
		&ai.AIModel{},
		&model.SystemConfig{},
		&model.User{},
	); err != nil {
		t.Fatalf("migrate test db: %v", err)
	}
	return gdb
}

func newCatalogServiceForTest(t *testing.T, db *gorm.DB) *CatalogService {
	t.Helper()
	return &CatalogService{repo: &CatalogRepo{db: &database.DB{DB: db}}}
}

func newServiceDefServiceForTest(t *testing.T, db *gorm.DB) *ServiceDefService {
	t.Helper()
	wrapped := &database.DB{DB: db}
	return &ServiceDefService{
		repo:     &ServiceDefRepo{db: wrapped},
		db:       wrapped,
		catalogs: &CatalogRepo{db: wrapped},
	}
}

func decodeResponseBody[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return v
}

func newGinContext(method, path string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(method, path, nil)
	return c, rec
}

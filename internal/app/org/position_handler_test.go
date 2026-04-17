package org

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/samber/do/v2"

	"metis/internal/database"
)

func newPositionHandlerForTest(t *testing.T, db *database.DB) *PositionHandler {
	t.Helper()
	injector := do.New()
	do.ProvideValue(injector, db)
	do.Provide(injector, NewPositionRepo)
	do.Provide(injector, NewPositionService)
	return &PositionHandler{svc: do.MustInvoke[*PositionService](injector)}
}

func setupPositionRouter(h *PositionHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	authed := r.Group("/api/v1")
	{
		authed.POST("/org/positions", h.Create)
		authed.GET("/org/positions", h.List)
		authed.GET("/org/positions/:id", h.Get)
		authed.PUT("/org/positions/:id", h.Update)
		authed.DELETE("/org/positions/:id", h.Delete)
	}
	return r
}

func TestPositionHandler_Create(t *testing.T) {
	db := newOrgTestDB(t)
	h := newPositionHandlerForTest(t, db)
	r := setupPositionRouter(h)

	body := `{"name":"Senior Engineer","code":"se"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/org/positions", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].(map[string]any)
	if data["code"] != "se" {
		t.Fatalf("expected code se, got %v", data["code"])
	}

	// duplicate code
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/org/positions", bytes.NewReader([]byte(body)))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w2.Code, w2.Body.String())
	}
}

func TestPositionHandler_List(t *testing.T) {
	db := newOrgTestDB(t)
	h := newPositionHandlerForTest(t, db)
	seedPosition(t, db, "Senior Engineer", "se", true)
	r := setupPositionRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/org/positions", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].(map[string]any)
	items := data["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
}

func TestPositionHandler_Get(t *testing.T) {
	db := newOrgTestDB(t)
	h := newPositionHandlerForTest(t, db)
	pos := seedPosition(t, db, "Senior Engineer", "se", true)
	r := setupPositionRouter(h)

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/org/positions/%d", pos.ID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/org/positions/9999", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w2.Code, w2.Body.String())
	}
}

func TestPositionHandler_Update(t *testing.T) {
	db := newOrgTestDB(t)
	h := newPositionHandlerForTest(t, db)
	pos := seedPosition(t, db, "Senior Engineer", "se", true)
	other := seedPosition(t, db, "Manager", "mgr", true)
	r := setupPositionRouter(h)

	body := `{"name":"Staff Engineer"}`
	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/v1/org/positions/%d", pos.ID), bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].(map[string]any)
	if data["name"] != "Staff Engineer" {
		t.Fatalf("expected name Staff Engineer, got %v", data["name"])
	}

	// duplicate code
	body2 := `{"code":"se"}`
	req2 := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/v1/org/positions/%d", other.ID), bytes.NewReader([]byte(body2)))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w2.Code, w2.Body.String())
	}

	// not found
	req3 := httptest.NewRequest(http.MethodPut, "/api/v1/org/positions/9999", bytes.NewReader([]byte(body)))
	req3.Header.Set("Content-Type", "application/json")
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, req3)
	if w3.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w3.Code, w3.Body.String())
	}
}

func TestPositionHandler_Delete(t *testing.T) {
	db := newOrgTestDB(t)
	h := newPositionHandlerForTest(t, db)
	pos := seedPosition(t, db, "Senior Engineer", "se", true)
	r := setupPositionRouter(h)

	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/org/positions/%d", pos.ID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// in use
	pos2 := seedPosition(t, db, "Engineer", "eng", true)
	role := seedRole(t, db, "user")
	dept := seedDepartment(t, db, "Engineering", "dept", nil, nil, true)
	user := seedUser(t, db, "u1", role.ID)
	seedAssignment(t, db, user.ID, dept.ID, pos2.ID, true)
	req2 := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/org/positions/%d", pos2.ID), nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w2.Code, w2.Body.String())
	}

	// not found
	req3 := httptest.NewRequest(http.MethodDelete, "/api/v1/org/positions/9999", nil)
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, req3)
	if w3.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w3.Code, w3.Body.String())
	}
}

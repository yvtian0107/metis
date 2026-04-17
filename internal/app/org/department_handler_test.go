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

func newDepartmentHandlerForTest(t *testing.T, db *database.DB) *DepartmentHandler {
	t.Helper()
	injector := do.New()
	do.ProvideValue(injector, db)
	do.Provide(injector, NewDepartmentRepo)
	do.Provide(injector, NewAssignmentRepo)
	do.Provide(injector, NewDepartmentService)
	return &DepartmentHandler{svc: do.MustInvoke[*DepartmentService](injector)}
}

func setupDepartmentRouter(h *DepartmentHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	authed := r.Group("/api/v1")
	{
		authed.POST("/org/departments", h.Create)
		authed.GET("/org/departments", h.List)
		authed.GET("/org/departments/tree", h.Tree)
		authed.GET("/org/departments/:id", h.Get)
		authed.PUT("/org/departments/:id", h.Update)
		authed.DELETE("/org/departments/:id", h.Delete)
	}
	return r
}

func TestDepartmentHandler_Create(t *testing.T) {
	db := newOrgTestDB(t)
	h := newDepartmentHandlerForTest(t, db)
	r := setupDepartmentRouter(h)

	body := `{"name":"Engineering","code":"eng","sort":1,"description":"R&D"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/org/departments", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].(map[string]any)
	if data["code"] != "eng" {
		t.Fatalf("expected code eng, got %v", data["code"])
	}

	// duplicate code
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/org/departments", bytes.NewReader([]byte(body)))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w2.Code, w2.Body.String())
	}
}

func TestDepartmentHandler_List(t *testing.T) {
	db := newOrgTestDB(t)
	h := newDepartmentHandlerForTest(t, db)
	seedDepartment(t, db, "Engineering", "eng", nil, nil, true)
	r := setupDepartmentRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/org/departments", nil)
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

func TestDepartmentHandler_Tree(t *testing.T) {
	db := newOrgTestDB(t)
	h := newDepartmentHandlerForTest(t, db)
	seedDepartment(t, db, "Parent", "parent", nil, nil, true)
	r := setupDepartmentRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/org/departments/tree", nil)
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
		t.Fatalf("expected 1 root, got %d", len(items))
	}
}

func TestDepartmentHandler_Get(t *testing.T) {
	db := newOrgTestDB(t)
	h := newDepartmentHandlerForTest(t, db)
	dept := seedDepartment(t, db, "Engineering", "eng", nil, nil, true)
	r := setupDepartmentRouter(h)

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/org/departments/%d", dept.ID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/org/departments/9999", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w2.Code, w2.Body.String())
	}
}

func TestDepartmentHandler_Update(t *testing.T) {
	db := newOrgTestDB(t)
	h := newDepartmentHandlerForTest(t, db)
	dept := seedDepartment(t, db, "Engineering", "eng", nil, nil, true)
	other := seedDepartment(t, db, "Product", "prod", nil, nil, true)
	r := setupDepartmentRouter(h)

	body := `{"name":"R&D"}`
	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/v1/org/departments/%d", dept.ID), bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].(map[string]any)
	if data["name"] != "R&D" {
		t.Fatalf("expected name R&D, got %v", data["name"])
	}

	// duplicate code
	body2 := `{"code":"eng"}`
	req2 := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/v1/org/departments/%d", other.ID), bytes.NewReader([]byte(body2)))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w2.Code, w2.Body.String())
	}

	// not found
	req3 := httptest.NewRequest(http.MethodPut, "/api/v1/org/departments/9999", bytes.NewReader([]byte(body)))
	req3.Header.Set("Content-Type", "application/json")
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, req3)
	if w3.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w3.Code, w3.Body.String())
	}
}

func TestDepartmentHandler_Delete(t *testing.T) {
	db := newOrgTestDB(t)
	h := newDepartmentHandlerForTest(t, db)
	dept := seedDepartment(t, db, "Engineering", "eng", nil, nil, true)
	r := setupDepartmentRouter(h)

	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/org/departments/%d", dept.ID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// has children
	parent := seedDepartment(t, db, "Parent", "parent", nil, nil, true)
	_ = seedDepartment(t, db, "Child", "child", &parent.ID, nil, true)
	req2 := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/org/departments/%d", parent.ID), nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w2.Code, w2.Body.String())
	}

	// has members
	dept2 := seedDepartment(t, db, "Dept2", "dept2", nil, nil, true)
	role := seedRole(t, db, "user")
	pos := seedPosition(t, db, "SE", "se", true)
	user := seedUser(t, db, "u1", role.ID)
	seedAssignment(t, db, user.ID, dept2.ID, pos.ID, true)
	req3 := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/org/departments/%d", dept2.ID), nil)
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, req3)
	if w3.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w3.Code, w3.Body.String())
	}

	// not found
	req4 := httptest.NewRequest(http.MethodDelete, "/api/v1/org/departments/9999", nil)
	w4 := httptest.NewRecorder()
	r.ServeHTTP(w4, req4)
	if w4.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w4.Code, w4.Body.String())
	}
}

package org

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"metis/internal/database"
)

func newAssignmentHandlerForTest(t *testing.T, db *database.DB) *AssignmentHandler {
	t.Helper()
	return &AssignmentHandler{
		svc:     newAssignmentService(db),
		userSvc: nil, // not used in tested endpoints
	}
}

func setupAssignmentRouter(h *AssignmentHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	authed := r.Group("/api/v1")
	{
		authed.GET("/org/users/:id/positions", h.GetUserPositions)
		authed.POST("/org/users/:id/positions", h.AddUserPosition)
		authed.DELETE("/org/users/:id/positions/:assignmentId", h.RemoveUserPosition)
		authed.PUT("/org/users/:id/positions/:assignmentId", h.UpdateUserPosition)
		authed.PUT("/org/users/:id/positions/:assignmentId/primary", h.SetPrimary)
		authed.GET("/org/assignments/users", h.ListUsers)
	}
	return r
}

func TestAssignmentHandler_GetUserPositions(t *testing.T) {
	db := newOrgTestDB(t)
	h := newAssignmentHandlerForTest(t, db)
	role := seedRole(t, db, "user")
	dept := seedDepartment(t, db, "Engineering", "eng", nil, nil, true)
	pos := seedPosition(t, db, "SE", "se", true)
	user := seedUser(t, db, "u1", role.ID)
	seedAssignment(t, db, user.ID, dept.ID, pos.ID, true)
	r := setupAssignmentRouter(h)

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/org/users/%d/positions", user.ID), nil)
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

func TestAssignmentHandler_AddUserPosition(t *testing.T) {
	db := newOrgTestDB(t)
	h := newAssignmentHandlerForTest(t, db)
	role := seedRole(t, db, "user")
	dept := seedDepartment(t, db, "Engineering", "eng", nil, nil, true)
	pos := seedPosition(t, db, "SE", "se", true)
	user := seedUser(t, db, "u1", role.ID)
	r := setupAssignmentRouter(h)

	body := fmt.Sprintf(`{"departmentId":%d,"positionId":%d,"isPrimary":true}`, dept.ID, pos.ID)
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/org/users/%d/positions", user.ID), bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// already assigned / sentinel errors
	req2 := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/org/users/%d/positions", user.ID), bytes.NewReader([]byte(body)))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w2.Code, w2.Body.String())
	}
}

func TestAssignmentHandler_RemoveUserPosition(t *testing.T) {
	db := newOrgTestDB(t)
	h := newAssignmentHandlerForTest(t, db)
	role := seedRole(t, db, "user")
	dept := seedDepartment(t, db, "Engineering", "eng", nil, nil, true)
	pos := seedPosition(t, db, "SE", "se", true)
	user := seedUser(t, db, "u1", role.ID)
	up := seedAssignment(t, db, user.ID, dept.ID, pos.ID, true)
	r := setupAssignmentRouter(h)

	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/org/users/%d/positions/%d", user.ID, up.ID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// not found
	req2 := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/org/users/%d/positions/9999", user.ID), nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w2.Code, w2.Body.String())
	}
}

func TestAssignmentHandler_UpdateUserPosition(t *testing.T) {
	db := newOrgTestDB(t)
	h := newAssignmentHandlerForTest(t, db)
	role := seedRole(t, db, "user")
	dept := seedDepartment(t, db, "Engineering", "eng", nil, nil, true)
	pos := seedPosition(t, db, "SE", "se", true)
	user := seedUser(t, db, "u1", role.ID)
	up := seedAssignment(t, db, user.ID, dept.ID, pos.ID, false)
	r := setupAssignmentRouter(h)

	body := `{"isPrimary":true}`
	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/v1/org/users/%d/positions/%d", user.ID, up.ID), bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// not found
	req2 := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/v1/org/users/%d/positions/9999", user.ID), bytes.NewReader([]byte(body)))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w2.Code, w2.Body.String())
	}
}

func TestAssignmentHandler_SetPrimary(t *testing.T) {
	db := newOrgTestDB(t)
	h := newAssignmentHandlerForTest(t, db)
	role := seedRole(t, db, "user")
	dept := seedDepartment(t, db, "Engineering", "eng", nil, nil, true)
	pos := seedPosition(t, db, "SE", "se", true)
	user := seedUser(t, db, "u1", role.ID)
	up := seedAssignment(t, db, user.ID, dept.ID, pos.ID, false)
	r := setupAssignmentRouter(h)

	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/v1/org/users/%d/positions/%d/primary", user.ID, up.ID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// not found
	req2 := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/v1/org/users/%d/positions/9999/primary", user.ID), nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w2.Code, w2.Body.String())
	}
}

func TestAssignmentHandler_ListUsers(t *testing.T) {
	db := newOrgTestDB(t)
	h := newAssignmentHandlerForTest(t, db)
	role := seedRole(t, db, "user")
	dept := seedDepartment(t, db, "Engineering", "eng", nil, nil, true)
	pos := seedPosition(t, db, "SE", "se", true)
	u1 := seedUser(t, db, "alice", role.ID)
	seedAssignment(t, db, u1.ID, dept.ID, pos.ID, true)
	r := setupAssignmentRouter(h)

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/org/assignments/users?departmentId=%d", dept.ID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// missing departmentId
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/org/assignments/users", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w2.Code, w2.Body.String())
	}
}

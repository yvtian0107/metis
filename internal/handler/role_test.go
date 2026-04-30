package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/samber/do/v2"
	"gorm.io/gorm"

	casbinpkg "metis/internal/casbin"
	"metis/internal/database"
	"metis/internal/model"
	"metis/internal/repository"
	"metis/internal/service"
)

func newTestDBForRoleHandler(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	gdb, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := gdb.AutoMigrate(
		&model.Role{},
		&model.RoleDeptScope{},
		&model.User{},
		&model.Menu{},
	); err != nil {
		t.Fatalf("migrate test db: %v", err)
	}
	return gdb
}

func newRoleHandlerForTest(t *testing.T, db *gorm.DB) (*RoleHandler, *service.RoleService, *service.CasbinService) {
	t.Helper()
	wrapped := &database.DB{DB: db}
	injector := do.New()
	do.ProvideValue(injector, wrapped)

	enforcer, err := casbinpkg.NewEnforcerWithDB(db)
	if err != nil {
		t.Fatalf("create casbin enforcer: %v", err)
	}
	do.ProvideValue(injector, enforcer)

	do.Provide(injector, repository.NewRole)
	do.Provide(injector, repository.NewMenu)
	do.Provide(injector, service.NewCasbin)
	do.Provide(injector, service.NewRole)
	do.Provide(injector, service.NewMenu)

	roleSvc := do.MustInvoke[*service.RoleService](injector)
	casbinSvc := do.MustInvoke[*service.CasbinService](injector)
	menuSvc := do.MustInvoke[*service.MenuService](injector)
	roleRepo := do.MustInvoke[*repository.RoleRepo](injector)

	return &RoleHandler{roleSvc: roleSvc, casbinSvc: casbinSvc, menuSvc: menuSvc, roleRepo: roleRepo}, roleSvc, casbinSvc
}

func setupRoleRouter(h *RoleHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("userId", uint(9999))
		c.Next()
	})
	r.GET("/api/v1/roles", h.List)
	r.POST("/api/v1/roles", h.Create)
	r.GET("/api/v1/roles/:id", h.Get)
	r.PUT("/api/v1/roles/:id", h.Update)
	r.DELETE("/api/v1/roles/:id", h.Delete)
	r.GET("/api/v1/roles/:id/permissions", h.GetPermissions)
	r.PUT("/api/v1/roles/:id/permissions", h.SetPermissions)
	r.PUT("/api/v1/roles/:id/data-scope", h.UpdateDataScope)
	return r
}

func seedRoleForRoleHandler(t *testing.T, db *gorm.DB, name, code string, isSystem bool) *model.Role {
	t.Helper()
	role := &model.Role{Name: name, Code: code, IsSystem: isSystem}
	if err := db.Create(role).Error; err != nil {
		t.Fatalf("seed role: %v", err)
	}
	return role
}

func seedUserForRoleHandler(t *testing.T, db *gorm.DB, username string, roleID uint) {
	t.Helper()
	user := &model.User{Username: username, RoleID: roleID}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
}

func seedMenuForRoleHandler(t *testing.T, db *gorm.DB, menu *model.Menu) *model.Menu {
	t.Helper()
	if err := db.Create(menu).Error; err != nil {
		t.Fatalf("seed menu: %v", err)
	}
	return menu
}

func TestRoleHandlerCreate_SetsDefaultDataScope(t *testing.T) {
	db := newTestDBForRoleHandler(t)
	h, _, _ := newRoleHandlerForTest(t, db)
	r := setupRoleRouter(h)

	body := `{"name":"Editor","code":"editor","description":"Content editor","sort":10}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/roles", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Code int        `json:"code"`
		Data model.Role `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Code != 0 {
		t.Fatalf("expected response code 0, got %d", resp.Code)
	}
	if resp.Data.DataScope != model.DataScopeAll {
		t.Fatalf("expected dataScope=all, got %s", resp.Data.DataScope)
	}
	if resp.Data.ID == 0 {
		t.Fatal("expected created role id")
	}

	var count int64
	if err := db.Model(&model.Role{}).Where("code = ?", "editor").Count(&count).Error; err != nil {
		t.Fatalf("count roles: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected role persisted once, got %d", count)
	}
}

func TestRoleHandlerList_NormalizesInvalidPagination(t *testing.T) {
	db := newTestDBForRoleHandler(t)
	h, _, _ := newRoleHandlerForTest(t, db)
	r := setupRoleRouter(h)
	seedRoleForRoleHandler(t, db, "B", "b", false)
	seedRoleForRoleHandler(t, db, "A", "a", false)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/roles?page=0&pageSize=0", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Code int `json:"code"`
		Data struct {
			Items    []model.Role `json:"items"`
			Total    int64        `json:"total"`
			Page     int          `json:"page"`
			PageSize int          `json:"pageSize"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Data.Page != 1 || resp.Data.PageSize != 20 {
		t.Fatalf("expected normalized page=1 pageSize=20, got page=%d pageSize=%d", resp.Data.Page, resp.Data.PageSize)
	}
	if len(resp.Data.Items) != 2 {
		t.Fatalf("expected 2 roles, got %d", len(resp.Data.Items))
	}
	if resp.Data.Items[0].Code != "b" || resp.Data.Items[1].Code != "a" {
		t.Fatalf("expected sort asc then id asc order, got %+v", resp.Data.Items)
	}
	if resp.Data.Total != 2 {
		t.Fatalf("expected total 2, got %d", resp.Data.Total)
	}
}

func TestRoleHandlerDelete_ReturnsBadRequestWhenUsersAssigned(t *testing.T) {
	db := newTestDBForRoleHandler(t)
	h, _, _ := newRoleHandlerForTest(t, db)
	r := setupRoleRouter(h)
	role := seedRoleForRoleHandler(t, db, "Editor", "editor", false)
	seedUserForRoleHandler(t, db, "alice", role.ID)

	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/roles/%d", role.ID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRoleHandlerSetPermissions_PersistsMenuAndAPIPolicies(t *testing.T) {
	db := newTestDBForRoleHandler(t)
	h, _, casbinSvc := newRoleHandlerForTest(t, db)
	r := setupRoleRouter(h)
	role := seedRoleForRoleHandler(t, db, "Editor", "editor", false)
	dir := seedMenuForRoleHandler(t, db, &model.Menu{Name: "System", Type: model.MenuTypeDirectory, Permission: "system", Sort: 1})
	page := seedMenuForRoleHandler(t, db, &model.Menu{Name: "Roles", Type: model.MenuTypeMenu, ParentID: &dir.ID, Permission: "system:role:list", Sort: 1})

	body := fmt.Sprintf(`{"menuIds":[%d],"apiPolicies":[{"path":"/api/v1/roles","method":"GET"}]}`, page.ID)
	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/v1/roles/%d/permissions", role.ID), bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	policies := casbinSvc.GetPoliciesForRole(role.Code)
	if len(policies) != 2 {
		t.Fatalf("expected 2 policies, got %v", policies)
	}

	hasMenu := false
	hasAPI := false
	for _, policy := range policies {
		if policy[1] == "system:role:list" && policy[2] == "read" {
			hasMenu = true
		}
		if policy[1] == "/api/v1/roles" && policy[2] == "GET" {
			hasAPI = true
		}
	}
	if !hasMenu || !hasAPI {
		t.Fatalf("expected menu and api policies, got %v", policies)
	}
}

func TestRoleHandlerGetPermissions_ReturnsEmptyArraysWhenRoleHasNoPolicies(t *testing.T) {
	db := newTestDBForRoleHandler(t)
	h, _, _ := newRoleHandlerForTest(t, db)
	r := setupRoleRouter(h)
	role := seedRoleForRoleHandler(t, db, "Editor", "editor", false)

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/roles/%d/permissions", role.ID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Code int `json:"code"`
		Data struct {
			MenuPermissions []string `json:"menuPermissions"`
			APIPolicies     []struct {
				Path   string `json:"path"`
				Method string `json:"method"`
			} `json:"apiPolicies"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Data.MenuPermissions == nil {
		t.Fatal("expected empty menuPermissions array, got nil")
	}
	if resp.Data.APIPolicies == nil {
		t.Fatal("expected empty apiPolicies array, got nil")
	}
	if len(resp.Data.MenuPermissions) != 0 || len(resp.Data.APIPolicies) != 0 {
		t.Fatalf("expected empty permissions, got %+v", resp.Data)
	}
}

func TestRoleHandlerCreate_ReturnsBadRequestOnDuplicateCode(t *testing.T) {
	db := newTestDBForRoleHandler(t)
	h, _, _ := newRoleHandlerForTest(t, db)
	r := setupRoleRouter(h)
	seedRoleForRoleHandler(t, db, "Editor", "editor", false)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/roles", bytes.NewBufferString(`{"name":"Editor 2","code":"editor"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRoleHandlerUpdateDataScope_SavesCustomDeptIDs(t *testing.T) {
	db := newTestDBForRoleHandler(t)
	h, _, _ := newRoleHandlerForTest(t, db)
	r := setupRoleRouter(h)
	role := seedRoleForRoleHandler(t, db, "Editor", "editor", false)

	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/v1/roles/%d/data-scope", role.ID), bytes.NewBufferString(`{"dataScope":"custom","deptIds":[10,20]}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var scopes []model.RoleDeptScope
	if err := db.Where("role_id = ?", role.ID).Order("department_id ASC").Find(&scopes).Error; err != nil {
		t.Fatalf("query role dept scopes: %v", err)
	}
	if len(scopes) != 2 || scopes[0].DepartmentID != 10 || scopes[1].DepartmentID != 20 {
		t.Fatalf("expected custom dept ids [10 20], got %+v", scopes)
	}
}

func TestRoleHandlerUpdateDataScope_RejectsInvalidScope(t *testing.T) {
	db := newTestDBForRoleHandler(t)
	h, _, _ := newRoleHandlerForTest(t, db)
	r := setupRoleRouter(h)
	role := seedRoleForRoleHandler(t, db, "Editor", "editor", false)

	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/v1/roles/%d/data-scope", role.ID), bytes.NewBufferString(`{"dataScope":"invalid"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
	if !bytes.Contains(w.Body.Bytes(), []byte("invalid data scope value")) {
		t.Fatalf("expected invalid data scope message, got %s", w.Body.String())
	}
}

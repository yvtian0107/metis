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

func newTestDBForMenuHandler(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	gdb, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := gdb.AutoMigrate(&model.Menu{}); err != nil {
		t.Fatalf("migrate test db: %v", err)
	}
	return gdb
}

func newMenuHandlerForTest(t *testing.T, db *gorm.DB) (*MenuHandler, *service.MenuService, *service.CasbinService) {
	t.Helper()
	wrapped := &database.DB{DB: db}
	injector := do.New()
	do.ProvideValue(injector, wrapped)

	enforcer, err := casbinpkg.NewEnforcerWithDB(db)
	if err != nil {
		t.Fatalf("create casbin enforcer: %v", err)
	}
	do.ProvideValue(injector, enforcer)

	do.Provide(injector, repository.NewMenu)
	do.Provide(injector, service.NewCasbin)
	do.Provide(injector, service.NewMenu)

	menuSvc := do.MustInvoke[*service.MenuService](injector)
	casbinSvc := do.MustInvoke[*service.CasbinService](injector)
	return &MenuHandler{menuSvc: menuSvc}, menuSvc, casbinSvc
}

func setupMenuRouter(h *MenuHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("userId", uint(9999))
		c.Set("userRole", "editor")
		c.Next()
	})
	r.GET("/api/v1/menus/tree", h.GetTree)
	r.GET("/api/v1/menus/user-tree", h.GetUserTree)
	r.POST("/api/v1/menus", h.Create)
	r.PUT("/api/v1/menus/:id", h.Update)
	r.PUT("/api/v1/menus/sort", h.Reorder)
	r.DELETE("/api/v1/menus/:id", h.Delete)
	return r
}

func seedMenuForMenuHandler(t *testing.T, db *gorm.DB, menu *model.Menu) *model.Menu {
	t.Helper()
	if err := db.Create(menu).Error; err != nil {
		t.Fatalf("seed menu: %v", err)
	}
	return menu
}

func TestMenuHandlerGetTree_ReturnsUnifiedResponse(t *testing.T) {
	db := newTestDBForMenuHandler(t)
	h, _, _ := newMenuHandlerForTest(t, db)
	r := setupMenuRouter(h)

	seedMenuForMenuHandler(t, db, &model.Menu{Name: "System", Type: model.MenuTypeDirectory, Permission: "test:menu:get-tree", Sort: 1})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/menus/tree", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Code    int          `json:"code"`
		Message string       `json:"message"`
		Data    []model.Menu `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Code != 0 {
		t.Fatalf("expected response code 0, got %d", resp.Code)
	}
	if len(resp.Data) != 1 || resp.Data[0].Name != "System" {
		t.Fatalf("expected tree with System root, got %+v", resp.Data)
	}
}

func TestMenuHandlerGetUserTree_ReturnsMenusAndPermissions(t *testing.T) {
	db := newTestDBForMenuHandler(t)
	h, _, casbinSvc := newMenuHandlerForTest(t, db)
	r := setupMenuRouter(h)

	dir := seedMenuForMenuHandler(t, db, &model.Menu{Name: "System", Type: model.MenuTypeDirectory, Permission: "test:menu:user-tree-dir", Sort: 1})
	seedMenuForMenuHandler(t, db, &model.Menu{Name: "Users", Type: model.MenuTypeMenu, ParentID: &dir.ID, Path: "/users", Permission: "system:user:list", Sort: 1})
	if err := casbinSvc.SetPoliciesForRole("editor", [][]string{{"editor", "system:user:list", "GET"}}); err != nil {
		t.Fatalf("set policies: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/menus/user-tree", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Code    int `json:"code"`
		Data    struct {
			Menus       []model.Menu `json:"menus"`
			Permissions []string     `json:"permissions"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if len(resp.Data.Menus) != 1 || len(resp.Data.Menus[0].Children) != 1 {
		t.Fatalf("expected filtered menus in response, got %+v", resp.Data.Menus)
	}
	if len(resp.Data.Permissions) != 1 || resp.Data.Permissions[0] != "system:user:list" {
		t.Fatalf("expected system:user:list permission, got %+v", resp.Data.Permissions)
	}
}

func TestMenuHandlerCreate_Success(t *testing.T) {
	db := newTestDBForMenuHandler(t)
	h, _, _ := newMenuHandlerForTest(t, db)
	r := setupMenuRouter(h)

	body := `{"name":"菜单管理","type":"menu","path":"/menus","permission":"system:menu:list","sort":1}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/menus", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Code int        `json:"code"`
		Data model.Menu `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Code != 0 {
		t.Fatalf("expected response code 0, got %d", resp.Code)
	}
	if resp.Data.ID == 0 || resp.Data.Permission != "system:menu:list" {
		t.Fatalf("expected created menu in response, got %+v", resp.Data)
	}
}

func TestMenuHandlerCreate_MissingName_ReturnsBadRequest(t *testing.T) {
	db := newTestDBForMenuHandler(t)
	h, _, _ := newMenuHandlerForTest(t, db)
	r := setupMenuRouter(h)

	body := `{"type":"menu","path":"/menus"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/menus", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestMenuHandlerCreate_InvalidType_ReturnsBadRequest(t *testing.T) {
	db := newTestDBForMenuHandler(t)
	h, _, _ := newMenuHandlerForTest(t, db)
	r := setupMenuRouter(h)

	body := `{"name":"非法菜单","type":"invalid","permission":"test:menu:invalid-type"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/menus", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid type, got %d: %s", w.Code, w.Body.String())
	}
}

func TestMenuHandlerCreate_DuplicatePermission_ReturnsBadRequest(t *testing.T) {
	db := newTestDBForMenuHandler(t)
	h, _, _ := newMenuHandlerForTest(t, db)
	r := setupMenuRouter(h)

	seedMenuForMenuHandler(t, db, &model.Menu{Name: "旧菜单", Type: model.MenuTypeMenu, Permission: "system:menu:duplicate", Sort: 1})
	body := `{"name":"新菜单","type":"menu","path":"/menus/new","permission":"system:menu:duplicate"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/menus", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for duplicate permission, got %d: %s", w.Code, w.Body.String())
	}
}

func TestMenuHandlerCreate_ButtonParent_ReturnsBadRequest(t *testing.T) {
	db := newTestDBForMenuHandler(t)
	h, _, _ := newMenuHandlerForTest(t, db)
	r := setupMenuRouter(h)

	button := seedMenuForMenuHandler(t, db, &model.Menu{Name: "按钮权限", Type: model.MenuTypeButton, Permission: "test:menu:button-parent", Sort: 1})
	body := fmt.Sprintf(`{"parentId":%d,"name":"子菜单","type":"menu","path":"/child","permission":"test:menu:child-under-button"}`, button.ID)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/menus", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for button parent, got %d: %s", w.Code, w.Body.String())
	}
}

func TestMenuHandlerUpdate_NotFound(t *testing.T) {
	db := newTestDBForMenuHandler(t)
	h, _, _ := newMenuHandlerForTest(t, db)
	r := setupMenuRouter(h)

	body := `{"name":"新名称"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/menus/999", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestMenuHandlerUpdate_DescendantParent_ReturnsBadRequest(t *testing.T) {
	db := newTestDBForMenuHandler(t)
	h, _, _ := newMenuHandlerForTest(t, db)
	r := setupMenuRouter(h)

	root := seedMenuForMenuHandler(t, db, &model.Menu{Name: "根目录", Type: model.MenuTypeDirectory, Permission: "test:menu:update-cycle-root", Sort: 1})
	child := seedMenuForMenuHandler(t, db, &model.Menu{Name: "子菜单", Type: model.MenuTypeMenu, ParentID: &root.ID, Permission: "test:menu:update-cycle-child", Sort: 1})
	body := fmt.Sprintf(`{"parentId":%d}`, child.ID)
	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/v1/menus/%d", root.ID), bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for descendant parent, got %d: %s", w.Code, w.Body.String())
	}
}

func TestMenuHandlerUpdate_InvalidType_ReturnsBadRequest(t *testing.T) {
	db := newTestDBForMenuHandler(t)
	h, _, _ := newMenuHandlerForTest(t, db)
	r := setupMenuRouter(h)

	menu := seedMenuForMenuHandler(t, db, &model.Menu{Name: "菜单", Type: model.MenuTypeMenu, Permission: "test:menu:update-invalid-type", Sort: 1})
	body := `{"type":"invalid"}`
	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/v1/menus/%d", menu.ID), bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid update type, got %d: %s", w.Code, w.Body.String())
	}
}

func TestMenuHandlerUpdate_DuplicatePermission_ReturnsBadRequest(t *testing.T) {
	db := newTestDBForMenuHandler(t)
	h, _, _ := newMenuHandlerForTest(t, db)
	r := setupMenuRouter(h)

	seedMenuForMenuHandler(t, db, &model.Menu{Name: "菜单A", Type: model.MenuTypeMenu, Permission: "test:menu:update-duplicate-a", Sort: 1})
	menuB := seedMenuForMenuHandler(t, db, &model.Menu{Name: "菜单B", Type: model.MenuTypeMenu, Permission: "test:menu:update-duplicate-b", Sort: 2})
	body := `{"permission":"test:menu:update-duplicate-a"}`
	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/v1/menus/%d", menuB.ID), bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for duplicate update permission, got %d: %s", w.Code, w.Body.String())
	}
}

func TestMenuHandlerUpdate_ButtonParent_ReturnsBadRequest(t *testing.T) {
	db := newTestDBForMenuHandler(t)
	h, _, _ := newMenuHandlerForTest(t, db)
	r := setupMenuRouter(h)

	button := seedMenuForMenuHandler(t, db, &model.Menu{Name: "按钮", Type: model.MenuTypeButton, Permission: "test:menu:update-button-parent", Sort: 1})
	menu := seedMenuForMenuHandler(t, db, &model.Menu{Name: "菜单", Type: model.MenuTypeMenu, Permission: "test:menu:update-button-child", Sort: 2})
	body := fmt.Sprintf(`{"parentId":%d}`, button.ID)
	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/v1/menus/%d", menu.ID), bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for button parent update, got %d: %s", w.Code, w.Body.String())
	}
}

func TestMenuHandlerUpdate_ClearParent_Success(t *testing.T) {
	db := newTestDBForMenuHandler(t)
	h, _, _ := newMenuHandlerForTest(t, db)
	r := setupMenuRouter(h)

	parent := seedMenuForMenuHandler(t, db, &model.Menu{Name: "父目录", Type: model.MenuTypeDirectory, Permission: "test:menu:update-clear-parent-parent", Sort: 1})
	menu := seedMenuForMenuHandler(t, db, &model.Menu{Name: "子菜单", Type: model.MenuTypeMenu, ParentID: &parent.ID, Permission: "test:menu:update-clear-parent-child", Sort: 2})
	body := `{"parentId":null}`
	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/v1/menus/%d", menu.ID), bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for clear parent, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Code int        `json:"code"`
		Data model.Menu `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Data.ParentID != nil {
		t.Fatalf("expected response parentID nil, got %v", resp.Data.ParentID)
	}
}

func TestMenuHandlerReorder_Success(t *testing.T) {
	db := newTestDBForMenuHandler(t)
	h, _, _ := newMenuHandlerForTest(t, db)
	r := setupMenuRouter(h)

	m1 := seedMenuForMenuHandler(t, db, &model.Menu{Name: "菜单1", Type: model.MenuTypeMenu, Permission: "test:menu:reorder-1", Sort: 1})
	m2 := seedMenuForMenuHandler(t, db, &model.Menu{Name: "菜单2", Type: model.MenuTypeMenu, Permission: "test:menu:reorder-2", Sort: 2})
	body := fmt.Sprintf(`{"items":[{"id":%d,"sort":20},{"id":%d,"sort":10}]}`, m1.ID, m2.ID)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/menus/sort", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestMenuHandlerReorder_EmptyItems_ReturnsBadRequest(t *testing.T) {
	db := newTestDBForMenuHandler(t)
	h, _, _ := newMenuHandlerForTest(t, db)
	r := setupMenuRouter(h)

	body := `{"items":[]}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/menus/sort", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty items, got %d: %s", w.Code, w.Body.String())
	}
}

func TestMenuHandlerReorder_UnknownID_ReturnsBadRequest(t *testing.T) {
	db := newTestDBForMenuHandler(t)
	h, _, _ := newMenuHandlerForTest(t, db)
	r := setupMenuRouter(h)

	body := `{"items":[{"id":999,"sort":1}]}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/menus/sort", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unknown reorder id, got %d: %s", w.Code, w.Body.String())
	}
}

func TestMenuHandlerDelete_LeafMenu_Success(t *testing.T) {
	db := newTestDBForMenuHandler(t)
	h, _, _ := newMenuHandlerForTest(t, db)
	r := setupMenuRouter(h)

	menu := seedMenuForMenuHandler(t, db, &model.Menu{Name: "叶子菜单", Type: model.MenuTypeMenu, Permission: "test:menu:delete-leaf", Sort: 1})
	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/menus/%d", menu.ID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestMenuHandlerDelete_MenuWithChildren_ReturnsBadRequest(t *testing.T) {
	db := newTestDBForMenuHandler(t)
	h, _, _ := newMenuHandlerForTest(t, db)
	r := setupMenuRouter(h)

	parent := seedMenuForMenuHandler(t, db, &model.Menu{Name: "父菜单", Type: model.MenuTypeDirectory, Permission: "test:menu:delete-parent", Sort: 1})
	seedMenuForMenuHandler(t, db, &model.Menu{Name: "子菜单", Type: model.MenuTypeMenu, ParentID: &parent.ID, Permission: "test:menu:delete-child", Sort: 1})
	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/menus/%d", parent.ID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}
package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/samber/do/v2"
	"gorm.io/gorm"

	"metis/internal/database"
	"metis/internal/model"
	"metis/internal/repository"
	"metis/internal/service"
)

func newTestDBForUserHandler(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	gdb, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := gdb.AutoMigrate(
		&model.User{},
		&model.Role{},
		&model.SystemConfig{},
		&model.RefreshToken{},
		&model.UserConnection{},
	); err != nil {
		t.Fatalf("migrate test db: %v", err)
	}
	return gdb
}

func newUserHandlerForTest(t *testing.T, db *gorm.DB) (*UserHandler, *service.UserService) {
	t.Helper()
	wrapped := &database.DB{DB: db}
	injector := do.New()
	do.ProvideValue(injector, wrapped)
	do.Provide(injector, repository.NewUser)
	do.Provide(injector, repository.NewRefreshToken)
	do.Provide(injector, repository.NewUserConnection)
	do.Provide(injector, repository.NewSysConfig)
	do.Provide(injector, service.NewSettings)
	do.Provide(injector, service.NewUser)

	userSvc := do.MustInvoke[*service.UserService](injector)
	connRepo := do.MustInvoke[*repository.UserConnectionRepo](injector)

	return &UserHandler{userSvc: userSvc, connRepo: connRepo}, userSvc
}

func setupUserRouter(h *UserHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("userId", uint(9999))
		c.Next()
	})
	r.GET("/api/v1/users", h.List)
	r.POST("/api/v1/users", h.Create)
	r.PUT("/api/v1/users/:id", h.Update)
	return r
}

func seedRoleForUserHandler(t *testing.T, db *gorm.DB, code string) *model.Role {
	t.Helper()
	role := &model.Role{Name: code, Code: code}
	if err := db.Create(role).Error; err != nil {
		t.Fatalf("seed role: %v", err)
	}
	return role
}

func setPasswordPolicyForUserHandler(t *testing.T, db *gorm.DB, minLen int, upper, lower, number, special bool) {
	t.Helper()
	configs := []model.SystemConfig{
		{Key: "security.password_min_length", Value: strconv.Itoa(minLen)},
		{Key: "security.password_require_upper", Value: strconv.FormatBool(upper)},
		{Key: "security.password_require_lower", Value: strconv.FormatBool(lower)},
		{Key: "security.password_require_number", Value: strconv.FormatBool(number)},
		{Key: "security.password_require_special", Value: strconv.FormatBool(special)},
	}
	for _, cfg := range configs {
		if err := db.Save(&cfg).Error; err != nil {
			t.Fatalf("set password policy: %v", err)
		}
	}
}

func TestUserHandlerCreate_SetsManagerWhenManagerIDProvided(t *testing.T) {
	db := newTestDBForUserHandler(t)
	h, userSvc := newUserHandlerForTest(t, db)
	r := setupUserRouter(h)
	role := seedRoleForUserHandler(t, db, "test-role")
	manager, err := userSvc.Create("manager", "Password123!", "manager@example.com", "", role.ID)
	if err != nil {
		t.Fatalf("create manager: %v", err)
	}

	body := fmt.Sprintf(`{"username":"alice","password":"Password123!","email":"alice@example.com","phone":"1234567890","roleId":%d,"managerId":%d}`,
		role.ID,
		manager.ID,
	)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Code    int                `json:"code"`
		Message string             `json:"message"`
		Data    model.UserResponse `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Code != 0 {
		t.Fatalf("expected response code 0, got %d", resp.Code)
	}
	if resp.Data.ManagerID == nil {
		t.Fatal("expected managerId to be set in create response")
	}
	if *resp.Data.ManagerID != manager.ID {
		t.Fatalf("expected managerId %d, got %d", manager.ID, *resp.Data.ManagerID)
	}
}

func TestUserHandlerCreate_ReturnsBadRequestOnPasswordPolicyViolation(t *testing.T) {
	db := newTestDBForUserHandler(t)
	h, _ := newUserHandlerForTest(t, db)
	r := setupUserRouter(h)
	role := seedRoleForUserHandler(t, db, "test-role")
	setPasswordPolicyForUserHandler(t, db, 16, true, true, true, true)

	body := fmt.Sprintf(`{"username":"alice","password":"short","email":"alice@example.com","roleId":%d}`,
		role.ID,
	)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUserHandlerUpdate_ReturnsBadRequestOnCircularManagerChain(t *testing.T) {
	db := newTestDBForUserHandler(t)
	h, userSvc := newUserHandlerForTest(t, db)
	r := setupUserRouter(h)
	role := seedRoleForUserHandler(t, db, "test-role")
	userA, err := userSvc.Create("user-a", "Password123!", "user-a@example.com", "", role.ID)
	if err != nil {
		t.Fatalf("create user-a: %v", err)
	}
	userB, err := userSvc.CreateWithParams(service.CreateUserParams{
		Username:  "user-b",
		Password:  "Password123!",
		Email:     "user-b@example.com",
		RoleID:    role.ID,
		ManagerID: &userA.ID,
	})
	if err != nil {
		t.Fatalf("create user-b: %v", err)
	}

	body := fmt.Sprintf(`{"managerId":%d}`, userB.ID)
	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/v1/users/%d", userA.ID), bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUserHandlerList_ReturnsManagerAndPagination(t *testing.T) {
	db := newTestDBForUserHandler(t)
	h, userSvc := newUserHandlerForTest(t, db)
	r := setupUserRouter(h)
	role := seedRoleForUserHandler(t, db, "test-role")
	manager, err := userSvc.Create("manager", "Password123!", "manager@example.com", "", role.ID)
	if err != nil {
		t.Fatalf("create manager: %v", err)
	}
	if _, err := userSvc.CreateWithParams(service.CreateUserParams{
		Username:  "alice",
		Password:  "Password123!",
		Email:     "alice@example.com",
		Phone:     "1234567890",
		RoleID:    role.ID,
		ManagerID: &manager.ID,
	}); err != nil {
		t.Fatalf("create managed user: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users?page=1&pageSize=20", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			Items []model.UserResponse `json:"items"`
			Total int64                `json:"total"`
			Page  int                  `json:"page"`
			Size  int                  `json:"pageSize"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Code != 0 {
		t.Fatalf("expected response code 0, got %d", resp.Code)
	}
	if resp.Data.Total != 2 {
		t.Fatalf("expected total 2, got %d", resp.Data.Total)
	}
	if resp.Data.Page != 1 || resp.Data.Size != 20 {
		t.Fatalf("expected page=1 pageSize=20, got page=%d pageSize=%d", resp.Data.Page, resp.Data.Size)
	}
	if len(resp.Data.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(resp.Data.Items))
	}
	if resp.Data.Items[0].Manager == nil || resp.Data.Items[0].Manager.ID != manager.ID {
		t.Fatalf("expected manager %d in first item, got %+v", manager.ID, resp.Data.Items[0].Manager)
	}
	if !resp.Data.Items[0].HasPassword {
		t.Fatal("expected hasPassword=true for created user")
	}
}
package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/samber/do/v2"
	"gorm.io/gorm"

	"metis/internal/database"
	"metis/internal/model"
	"metis/internal/pkg/token"
	"metis/internal/repository"
	"metis/internal/service"
)

func newTestDBForSessionHandler(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	gdb, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := gdb.AutoMigrate(
		&model.Role{},
		&model.User{},
		&model.RefreshToken{},
	); err != nil {
		t.Fatalf("migrate test db: %v", err)
	}
	return gdb
}

func newSessionHandlerForTest(t *testing.T, db *gorm.DB) (*SessionHandler, *service.SessionService) {
	t.Helper()
	wrapped := &database.DB{DB: db}
	injector := do.New()
	do.ProvideValue(injector, wrapped)
	do.ProvideValue(injector, token.NewBlacklist())
	do.Provide(injector, repository.NewRefreshToken)
	do.Provide(injector, service.NewSession)

	sessionSvc := do.MustInvoke[*service.SessionService](injector)
	return &SessionHandler{sessionSvc: sessionSvc}, sessionSvc
}

func setupSessionRouter(h *SessionHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("userId", uint(9999))
		c.Set("tokenJTI", "jti-current")
		c.Next()
	})
	r.GET("/api/v1/sessions", h.List)
	r.DELETE("/api/v1/sessions/:id", h.Kick)
	return r
}

func seedUserForSessionHandler(t *testing.T, db *gorm.DB, username string) *model.User {
	t.Helper()
	user := &model.User{Username: username}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return user
}

func seedRefreshTokenForSessionHandler(t *testing.T, db *gorm.DB, userID uint, tokenValue, jti string, lastSeenAt time.Time, revoked bool, expiresAt time.Time) *model.RefreshToken {
	t.Helper()
	rt := &model.RefreshToken{
		Token:          tokenValue,
		UserID:         userID,
		Revoked:        revoked,
		ExpiresAt:      expiresAt,
		LastSeenAt:     lastSeenAt,
		AccessTokenJTI: jti,
	}
	if err := db.Create(rt).Error; err != nil {
		t.Fatalf("seed refresh token: %v", err)
	}
	return rt
}

func TestSessionHandlerList_ReturnsPaginationMetadataAndCurrentSession(t *testing.T) {
	db := newTestDBForSessionHandler(t)
	h, _ := newSessionHandlerForTest(t, db)
	r := setupSessionRouter(h)
	user := seedUserForSessionHandler(t, db, "alice")
	future := time.Now().Add(24 * time.Hour)
	seedRefreshTokenForSessionHandler(t, db, user.ID, "rt-1", "jti-current", time.Now().Add(-1*time.Minute), false, future)
	seedRefreshTokenForSessionHandler(t, db, user.ID, "rt-2", "jti-other", time.Now().Add(-2*time.Minute), false, future)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions?page=1&pageSize=20", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Code int `json:"code"`
		Data struct {
			Items []struct {
				ID        uint   `json:"id"`
				Username  string `json:"username"`
				IsCurrent bool   `json:"isCurrent"`
			} `json:"items"`
			Total    int64 `json:"total"`
			Page     int   `json:"page"`
			PageSize int   `json:"pageSize"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Code != 0 {
		t.Fatalf("expected response code 0, got %d", resp.Code)
	}
	if resp.Data.Page != 1 || resp.Data.PageSize != 20 {
		t.Fatalf("expected page=1 pageSize=20, got page=%d pageSize=%d", resp.Data.Page, resp.Data.PageSize)
	}
	if resp.Data.Total != 2 || len(resp.Data.Items) != 2 {
		t.Fatalf("expected total=2 and 2 items, got total=%d len=%d", resp.Data.Total, len(resp.Data.Items))
	}
	currentCount := 0
	for _, item := range resp.Data.Items {
		if item.IsCurrent {
			currentCount++
		}
	}
	if currentCount != 1 {
		t.Fatalf("expected exactly 1 current session, got %d", currentCount)
	}
}

func TestSessionHandlerList_NormalizesInvalidPaginationAndSupportsEmptyList(t *testing.T) {
	db := newTestDBForSessionHandler(t)
	h, _ := newSessionHandlerForTest(t, db)
	r := setupSessionRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions?page=0&pageSize=101", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Code int `json:"code"`
		Data struct {
			Items    []any `json:"items"`
			Total    int64 `json:"total"`
			Page     int   `json:"page"`
			PageSize int   `json:"pageSize"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Data.Page != 1 || resp.Data.PageSize != 20 {
		t.Fatalf("expected normalized page=1 pageSize=20, got page=%d pageSize=%d", resp.Data.Page, resp.Data.PageSize)
	}
	if resp.Data.Total != 0 || len(resp.Data.Items) != 0 {
		t.Fatalf("expected empty result, got total=%d len=%d", resp.Data.Total, len(resp.Data.Items))
	}
}

func TestSessionHandlerKick_SuccessRevokesSession(t *testing.T) {
	db := newTestDBForSessionHandler(t)
	h, _ := newSessionHandlerForTest(t, db)
	r := setupSessionRouter(h)
	user := seedUserForSessionHandler(t, db, "bob")
	rt := seedRefreshTokenForSessionHandler(t, db, user.ID, "rt-kick", "jti-kick", time.Now(), false, time.Now().Add(24*time.Hour))

	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/sessions/%d", rt.ID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var updated model.RefreshToken
	if err := db.First(&updated, rt.ID).Error; err != nil {
		t.Fatalf("reload refresh token: %v", err)
	}
	if !updated.Revoked {
		t.Fatal("expected session refresh token to be revoked")
	}
}

func TestSessionHandlerKick_SelfKickReturnsBadRequest(t *testing.T) {
	db := newTestDBForSessionHandler(t)
	h, _ := newSessionHandlerForTest(t, db)
	r := setupSessionRouter(h)
	user := seedUserForSessionHandler(t, db, "carol")
	rt := seedRefreshTokenForSessionHandler(t, db, user.ID, "rt-self", "jti-current", time.Now(), false, time.Now().Add(24*time.Hour))

	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/sessions/%d", rt.ID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSessionHandlerKick_InvalidSessionIDReturnsBadRequest(t *testing.T) {
	db := newTestDBForSessionHandler(t)
	h, _ := newSessionHandlerForTest(t, db)
	r := setupSessionRouter(h)

	for _, path := range []string{"/api/v1/sessions/abc", "/api/v1/sessions/0"} {
		req := httptest.NewRequest(http.MethodDelete, path, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for %s, got %d: %s", path, w.Code, w.Body.String())
		}
	}
}

func TestSessionHandlerKick_NonActiveSessionReturnsNotFound(t *testing.T) {
	db := newTestDBForSessionHandler(t)
	h, _ := newSessionHandlerForTest(t, db)
	r := setupSessionRouter(h)
	user := seedUserForSessionHandler(t, db, "dora")
	revoked := seedRefreshTokenForSessionHandler(t, db, user.ID, "rt-revoked", "jti-revoked", time.Now(), true, time.Now().Add(24*time.Hour))
	expired := seedRefreshTokenForSessionHandler(t, db, user.ID, "rt-expired", "jti-expired", time.Now(), false, time.Now().Add(-24*time.Hour))

	for _, id := range []uint{revoked.ID, expired.ID} {
		req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/sessions/%d", id), nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Fatalf("expected 404 for session %d, got %d: %s", id, w.Code, w.Body.String())
		}
	}
}
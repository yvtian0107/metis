package service

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/samber/do/v2"
	"gorm.io/gorm"

	"metis/internal/database"
	"metis/internal/model"
	"metis/internal/pkg/token"
	"metis/internal/repository"
)

func newTestDBForSession(t *testing.T) *gorm.DB {
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

func newSessionServiceForTest(t *testing.T, db *gorm.DB) *SessionService {
	t.Helper()
	wrapped := &database.DB{DB: db}
	injector := do.New()
	do.ProvideValue(injector, wrapped)
	do.ProvideValue(injector, token.NewBlacklist())
	do.Provide(injector, repository.NewRefreshToken)
	do.Provide(injector, NewSession)

	return do.MustInvoke[*SessionService](injector)
}

func seedUserForSessionTest(t *testing.T, db *gorm.DB, username string) *model.User {
	t.Helper()
	user := &model.User{Username: username}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return user
}

func seedRefreshToken(t *testing.T, db *gorm.DB, userID uint, tok string, revoked bool, expiresAt time.Time, jti string) *model.RefreshToken {
	t.Helper()
	rt := &model.RefreshToken{
		Token:          tok,
		UserID:         userID,
		Revoked:        revoked,
		ExpiresAt:      expiresAt,
		LastSeenAt:     time.Now(),
		AccessTokenJTI: jti,
	}
	if err := db.Create(rt).Error; err != nil {
		t.Fatalf("seed refresh token: %v", err)
	}
	return rt
}

// 1. List Sessions

func TestSessionServiceListSessions_Pagination(t *testing.T) {
	db := newTestDBForSession(t)
	svc := newSessionServiceForTest(t, db)
	user := seedUserForSessionTest(t, db, "alice")
	future := time.Now().Add(24 * time.Hour)

	seedRefreshToken(t, db, user.ID, "rt1", false, future, "jti1")
	seedRefreshToken(t, db, user.ID, "rt2", false, future, "jti2")
	seedRefreshToken(t, db, user.ID, "rt3", false, future, "jti3")

	result, err := svc.ListSessions(1, 2, "")
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	if len(result.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(result.Items))
	}
	if result.Total != 3 {
		t.Fatalf("expected total 3, got %d", result.Total)
	}
}

func TestSessionServiceListSessions_MarksCurrentJTI(t *testing.T) {
	db := newTestDBForSession(t)
	svc := newSessionServiceForTest(t, db)
	user := seedUserForSessionTest(t, db, "bob")
	future := time.Now().Add(24 * time.Hour)

	seedRefreshToken(t, db, user.ID, "rt1", false, future, "jti-a")
	seedRefreshToken(t, db, user.ID, "rt2", false, future, "jti-b")

	result, err := svc.ListSessions(1, 10, "jti-a")
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	if len(result.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(result.Items))
	}

	var currentCount int
	for _, item := range result.Items {
		if item.IsCurrent {
			currentCount++
		}
	}
	if currentCount != 1 {
		t.Fatalf("expected exactly 1 current session, got %d", currentCount)
	}
}

func TestSessionServiceListSessions_ExcludesRevokedAndExpired(t *testing.T) {
	db := newTestDBForSession(t)
	svc := newSessionServiceForTest(t, db)
	user := seedUserForSessionTest(t, db, "carol")
	future := time.Now().Add(24 * time.Hour)
	past := time.Now().Add(-24 * time.Hour)

	active := seedRefreshToken(t, db, user.ID, "rt-active", false, future, "jti-active")
	_ = seedRefreshToken(t, db, user.ID, "rt-revoked", true, future, "jti-revoked")
	_ = seedRefreshToken(t, db, user.ID, "rt-expired", false, past, "jti-expired")

	result, err := svc.ListSessions(1, 10, "")
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	if result.Total != 1 {
		t.Fatalf("expected total 1, got %d", result.Total)
	}
	if len(result.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(result.Items))
	}
	if result.Items[0].ID != active.ID {
		t.Fatalf("expected active session id %d, got %d", active.ID, result.Items[0].ID)
	}
}

func TestSessionServiceListSessions_Empty(t *testing.T) {
	db := newTestDBForSession(t)
	svc := newSessionServiceForTest(t, db)

	result, err := svc.ListSessions(1, 10, "")
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	if len(result.Items) != 0 {
		t.Fatalf("expected 0 items, got %d", len(result.Items))
	}
	if result.Total != 0 {
		t.Fatalf("expected total 0, got %d", result.Total)
	}
}

// 2. Kick Session

func TestSessionServiceKickSession_Success(t *testing.T) {
	db := newTestDBForSession(t)
	svc := newSessionServiceForTest(t, db)
	user := seedUserForSessionTest(t, db, "dave")
	future := time.Now().Add(24 * time.Hour)
	rt := seedRefreshToken(t, db, user.ID, "rt-kick", false, future, "jti-kick")

	if err := svc.KickSession(rt.ID, "different-jti"); err != nil {
		t.Fatalf("kick session: %v", err)
	}

	updated, err := svc.refreshTokenRepo.FindByID(rt.ID)
	if err != nil {
		t.Fatalf("find by id: %v", err)
	}
	if !updated.Revoked {
		t.Fatal("expected token to be revoked")
	}
	if !svc.blacklist.IsBlocked("jti-kick") {
		t.Fatal("expected jti to be blacklisted")
	}
}

func TestSessionServiceKickSession_PreventsSelfKick(t *testing.T) {
	db := newTestDBForSession(t)
	svc := newSessionServiceForTest(t, db)
	user := seedUserForSessionTest(t, db, "eve")
	future := time.Now().Add(24 * time.Hour)
	rt := seedRefreshToken(t, db, user.ID, "rt-self", false, future, "jti-self")

	err := svc.KickSession(rt.ID, "jti-self")
	if !errors.Is(err, ErrCannotKickSelf) {
		t.Fatalf("expected ErrCannotKickSelf, got %v", err)
	}
}

func TestSessionServiceKickSession_NotFound(t *testing.T) {
	db := newTestDBForSession(t)
	svc := newSessionServiceForTest(t, db)

	err := svc.KickSession(999, "jti-x")
	if !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("expected ErrSessionNotFound, got %v", err)
	}
}

func TestSessionServiceKickSession_AlreadyRevoked(t *testing.T) {
	db := newTestDBForSession(t)
	svc := newSessionServiceForTest(t, db)
	user := seedUserForSessionTest(t, db, "frank")
	future := time.Now().Add(24 * time.Hour)
	rt := seedRefreshToken(t, db, user.ID, "rt-revoked", true, future, "jti-revoked")

	err := svc.KickSession(rt.ID, "jti-x")
	if !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("expected ErrSessionNotFound, got %v", err)
	}
}

func TestSessionServiceKickSession_ExpiredSessionReturnsNotFound(t *testing.T) {
	db := newTestDBForSession(t)
	svc := newSessionServiceForTest(t, db)
	user := seedUserForSessionTest(t, db, "henry")
	past := time.Now().Add(-24 * time.Hour)
	rt := seedRefreshToken(t, db, user.ID, "rt-expired", false, past, "jti-expired")

	err := svc.KickSession(rt.ID, "other-jti")
	if !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("expected ErrSessionNotFound, got %v", err)
	}
	if svc.blacklist.IsBlocked("jti-expired") {
		t.Fatal("expected expired session jti not to be blacklisted")
	}

	updated, findErr := svc.refreshTokenRepo.FindByID(rt.ID)
	if findErr != nil {
		t.Fatalf("find by id: %v", findErr)
	}
	if updated.Revoked {
		t.Fatal("expected expired session to remain unchanged")
	}
}

func TestSessionServiceKickSession_EmptyJTI(t *testing.T) {
	db := newTestDBForSession(t)
	svc := newSessionServiceForTest(t, db)
	user := seedUserForSessionTest(t, db, "grace")
	future := time.Now().Add(24 * time.Hour)
	rt := seedRefreshToken(t, db, user.ID, "rt-empty", false, future, "")

	beforeCount := svc.blacklist.Count()
	if err := svc.KickSession(rt.ID, "jti-x"); err != nil {
		t.Fatalf("kick session: %v", err)
	}
	if svc.blacklist.Count() != beforeCount {
		t.Fatal("expected blacklist count unchanged for empty jti")
	}

	updated, err := svc.refreshTokenRepo.FindByID(rt.ID)
	if err != nil {
		t.Fatalf("find by id: %v", err)
	}
	if !updated.Revoked {
		t.Fatal("expected token to be revoked")
	}
}

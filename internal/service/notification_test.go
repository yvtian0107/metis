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
	"metis/internal/repository"
)

func newTestDBForNotification(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	gdb, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := gdb.AutoMigrate(
		&model.User{},
		&model.Notification{},
		&model.NotificationRead{},
	); err != nil {
		t.Fatalf("migrate test db: %v", err)
	}
	return gdb
}

func newNotificationServiceForTest(t *testing.T, db *gorm.DB) *NotificationService {
	t.Helper()
	injector := do.New()
	do.ProvideValue(injector, &database.DB{DB: db})
	do.Provide(injector, repository.NewNotification)
	do.Provide(injector, NewNotification)
	return do.MustInvoke[*NotificationService](injector)
}

func seedUserForNotification(t *testing.T, db *gorm.DB, username string) *model.User {
	t.Helper()
	u := &model.User{Username: username}
	if err := db.Create(u).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return u
}

func seedAnnouncement(t *testing.T, db *gorm.DB, title, content string, createdBy *uint) *model.Notification {
	t.Helper()
	n := &model.Notification{
		Type:       model.NotificationTypeAnnouncement,
		Source:     "announcement",
		Title:      title,
		Content:    content,
		TargetType: model.NotificationTargetAll,
		CreatedBy:  createdBy,
	}
	if err := db.Create(n).Error; err != nil {
		t.Fatalf("seed announcement: %v", err)
	}
	return n
}

func seedNotificationRead(t *testing.T, db *gorm.DB, notificationID, userID uint) {
	t.Helper()
	nr := &model.NotificationRead{
		NotificationID: notificationID,
		UserID:         userID,
		ReadAt:         time.Now(),
	}
	if err := db.Create(nr).Error; err != nil {
		t.Fatalf("seed notification read: %v", err)
	}
}

func TestNotificationServiceListAnnouncements_Pagination(t *testing.T) {
	db := newTestDBForNotification(t)
	svc := newNotificationServiceForTest(t, db)

	for i := 0; i < 25; i++ {
		seedAnnouncement(t, db, fmt.Sprintf("Announcement %d", i), "content", nil)
	}

	items, total, err := svc.ListAnnouncements(repository.ListParams{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("list announcements: %v", err)
	}
	if total != 25 {
		t.Fatalf("expected total 25, got %d", total)
	}
	if len(items) != 10 {
		t.Fatalf("expected 10 items, got %d", len(items))
	}
}

func TestNotificationServiceListAnnouncements_KeywordFilter(t *testing.T) {
	db := newTestDBForNotification(t)
	svc := newNotificationServiceForTest(t, db)

	seedAnnouncement(t, db, "Welcome to Metis", "content", nil)
	seedAnnouncement(t, db, "System Maintenance", "content", nil)
	seedAnnouncement(t, db, "Holiday Notice", "content", nil)

	items, total, err := svc.ListAnnouncements(repository.ListParams{Keyword: "Metis", Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("list announcements: %v", err)
	}
	if total != 1 {
		t.Fatalf("expected total 1, got %d", total)
	}
	if len(items) != 1 || items[0].Title != "Welcome to Metis" {
		t.Fatalf("unexpected items: %+v", items)
	}
}

func TestNotificationServiceListAnnouncements_CreatorUsername(t *testing.T) {
	db := newTestDBForNotification(t)
	svc := newNotificationServiceForTest(t, db)

	u := seedUserForNotification(t, db, "admin")
	seedAnnouncement(t, db, "Welcome", "content", &u.ID)

	items, _, err := svc.ListAnnouncements(repository.ListParams{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("list announcements: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].CreatorUsername != "admin" {
		t.Fatalf("expected creator username admin, got %s", items[0].CreatorUsername)
	}
}

func TestNotificationServiceCreateAnnouncement_StoresCorrectFields(t *testing.T) {
	db := newTestDBForNotification(t)
	svc := newNotificationServiceForTest(t, db)

	u := seedUserForNotification(t, db, "creator")
	n, err := svc.CreateAnnouncement("Title", "Body", u.ID)
	if err != nil {
		t.Fatalf("create announcement: %v", err)
	}
	if n.Type != model.NotificationTypeAnnouncement {
		t.Fatalf("expected type %s, got %s", model.NotificationTypeAnnouncement, n.Type)
	}
	if n.Source != "announcement" {
		t.Fatalf("expected source announcement, got %s", n.Source)
	}
	if n.Title != "Title" {
		t.Fatalf("expected title Title, got %s", n.Title)
	}
	if n.Content != "Body" {
		t.Fatalf("expected content Body, got %s", n.Content)
	}
	if n.TargetType != model.NotificationTargetAll {
		t.Fatalf("expected target_type %s, got %s", model.NotificationTargetAll, n.TargetType)
	}
	if n.TargetID != nil {
		t.Fatal("expected target_id nil")
	}
	if n.CreatedBy == nil || *n.CreatedBy != u.ID {
		t.Fatalf("expected created_by %d, got %v", u.ID, n.CreatedBy)
	}
}

func TestNotificationServiceUpdateAnnouncement_Success(t *testing.T) {
	db := newTestDBForNotification(t)
	svc := newNotificationServiceForTest(t, db)

	n := seedAnnouncement(t, db, "Old Title", "Old Content", nil)
	updated, err := svc.UpdateAnnouncement(n.ID, "New Title", "New Content")
	if err != nil {
		t.Fatalf("update announcement: %v", err)
	}
	if updated.Title != "New Title" {
		t.Fatalf("expected title New Title, got %s", updated.Title)
	}
	if updated.Content != "New Content" {
		t.Fatalf("expected content New Content, got %s", updated.Content)
	}

	stored, err := svc.notifRepo.FindByID(n.ID)
	if err != nil {
		t.Fatalf("find announcement: %v", err)
	}
	if stored.Title != "New Title" || stored.Content != "New Content" {
		t.Fatalf("stored announcement not updated")
	}
}

func TestNotificationServiceUpdateAnnouncement_NotFound(t *testing.T) {
	db := newTestDBForNotification(t)
	svc := newNotificationServiceForTest(t, db)

	_, err := svc.UpdateAnnouncement(9999, "Title", "Content")
	if !errors.Is(err, ErrNotificationNotFound) {
		t.Fatalf("expected ErrNotificationNotFound, got %v", err)
	}
}

func TestNotificationServiceDeleteAnnouncement_Success(t *testing.T) {
	db := newTestDBForNotification(t)
	svc := newNotificationServiceForTest(t, db)

	u := seedUserForNotification(t, db, "reader")
	n := seedAnnouncement(t, db, "To Delete", "content", nil)
	seedNotificationRead(t, db, n.ID, u.ID)

	if err := svc.DeleteAnnouncement(n.ID); err != nil {
		t.Fatalf("delete announcement: %v", err)
	}

	var count int64
	if err := db.Model(&model.Notification{}).Where("id = ?", n.ID).Count(&count).Error; err != nil {
		t.Fatalf("count notifications: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected notification deleted, count=%d", count)
	}

	if err := db.Model(&model.NotificationRead{}).Where("notification_id = ?", n.ID).Count(&count).Error; err != nil {
		t.Fatalf("count notification reads: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected read records deleted, count=%d", count)
	}
}

func TestNotificationServiceDeleteAnnouncement_NotFound(t *testing.T) {
	db := newTestDBForNotification(t)
	svc := newNotificationServiceForTest(t, db)

	if err := svc.DeleteAnnouncement(9999); !errors.Is(err, ErrNotificationNotFound) {
		t.Fatalf("expected ErrNotificationNotFound, got %v", err)
	}
}

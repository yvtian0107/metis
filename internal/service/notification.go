package service

import (
	"errors"

	"github.com/samber/do/v2"
	"gorm.io/gorm"

	"metis/internal/model"
	"metis/internal/repository"
)

var (
	ErrNotificationNotFound = errors.New("error.notification.not_found")
)

type NotificationService struct {
	notifRepo *repository.NotificationRepo
}

func NewNotification(i do.Injector) (*NotificationService, error) {
	notifRepo := do.MustInvoke[*repository.NotificationRepo](i)
	return &NotificationService{notifRepo: notifRepo}, nil
}

// Send creates a notification. This is the unified interface for all modules.
func (s *NotificationService) Send(notifType, source, title, content, targetType string, targetID *uint, createdBy *uint) (*model.Notification, error) {
	n := &model.Notification{
		Type:       notifType,
		Source:     source,
		Title:      title,
		Content:    content,
		TargetType: targetType,
		TargetID:   targetID,
		CreatedBy:  createdBy,
	}
	if err := s.notifRepo.Create(n); err != nil {
		return nil, err
	}
	return n, nil
}

// ListForUser returns paginated notifications for a user.
func (s *NotificationService) ListForUser(userID uint, params repository.ListParams) ([]model.NotificationResponse, int64, error) {
	return s.notifRepo.ListForUser(userID, params)
}

// GetUnreadCount returns the unread notification count for a user.
func (s *NotificationService) GetUnreadCount(userID uint) (int64, error) {
	return s.notifRepo.CountUnreadForUser(userID)
}

// MarkAsRead marks a single notification as read for a user.
func (s *NotificationService) MarkAsRead(notificationID, userID uint) error {
	return s.notifRepo.MarkAsRead(notificationID, userID)
}

// MarkAllAsRead marks all unread notifications as read for a user.
func (s *NotificationService) MarkAllAsRead(userID uint) error {
	return s.notifRepo.MarkAllAsRead(userID)
}

// ListAnnouncements returns paginated announcements for the management page.
func (s *NotificationService) ListAnnouncements(params repository.ListParams) ([]model.AnnouncementResponse, int64, error) {
	return s.notifRepo.ListAnnouncements(params)
}

// CreateAnnouncement creates an announcement and sends it as a broadcast notification.
func (s *NotificationService) CreateAnnouncement(title, content string, createdBy uint) (*model.Notification, error) {
	return s.Send(
		model.NotificationTypeAnnouncement,
		"announcement",
		title, content,
		model.NotificationTargetAll, nil, &createdBy,
	)
}

// UpdateAnnouncement updates an announcement's title and content.
func (s *NotificationService) UpdateAnnouncement(id uint, title, content string) (*model.Notification, error) {
	n, err := s.notifRepo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotificationNotFound
		}
		return nil, err
	}

	n.Title = title
	n.Content = content
	if err := s.notifRepo.Update(n); err != nil {
		return nil, err
	}
	return n, nil
}

// DeleteAnnouncement deletes an announcement and its associated read records.
func (s *NotificationService) DeleteAnnouncement(id uint) error {
	if err := s.notifRepo.Delete(id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotificationNotFound
		}
		return err
	}
	return nil
}

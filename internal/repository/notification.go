package repository

import (
	"time"

	"github.com/samber/do/v2"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"metis/internal/database"
	"metis/internal/model"
)

type NotificationRepo struct {
	db *database.DB
}

func NewNotification(i do.Injector) (*NotificationRepo, error) {
	db := do.MustInvoke[*database.DB](i)
	return &NotificationRepo{db: db}, nil
}

func (r *NotificationRepo) Create(n *model.Notification) error {
	return r.db.Create(n).Error
}

func (r *NotificationRepo) FindByID(id uint) (*model.Notification, error) {
	var n model.Notification
	if err := r.db.First(&n, id).Error; err != nil {
		return nil, err
	}
	return &n, nil
}

func (r *NotificationRepo) Update(n *model.Notification) error {
	return r.db.Save(n).Error
}

func (r *NotificationRepo) Delete(id uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		// Clean up read records first
		if err := tx.Where("notification_id = ?", id).Delete(&model.NotificationRead{}).Error; err != nil {
			return err
		}
		result := tx.Delete(&model.Notification{}, id)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
}

// ListForUser returns notifications visible to a user (broadcast + targeted), with isRead status.
func (r *NotificationRepo) ListForUser(userID uint, params ListParams) ([]model.NotificationResponse, int64, error) {
	baseQuery := r.db.Model(&model.Notification{}).
		Where("target_type = ? OR (target_type = ? AND target_id = ?)",
			model.NotificationTargetAll, model.NotificationTargetUser, userID)

	if params.Keyword != "" {
		like := "%" + params.Keyword + "%"
		baseQuery = baseQuery.Where("title LIKE ?", like)
	}

	var total int64
	if err := baseQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if params.Page < 1 {
		params.Page = 1
	}
	if params.PageSize < 1 {
		params.PageSize = 20
	}
	offset := (params.Page - 1) * params.PageSize

	type row struct {
		model.Notification
		ReadAt *time.Time
	}

	var rows []row
	err := r.db.Table("notifications").
		Select("notifications.*, notification_reads.read_at").
		Joins("LEFT JOIN notification_reads ON notification_reads.notification_id = notifications.id AND notification_reads.user_id = ?", userID).
		Where("notifications.deleted_at IS NULL").
		Where("notifications.target_type = ? OR (notifications.target_type = ? AND notifications.target_id = ?)",
			model.NotificationTargetAll, model.NotificationTargetUser, userID).
		Order("notifications.created_at DESC").
		Offset(offset).Limit(params.PageSize).
		Find(&rows).Error
	if err != nil {
		return nil, 0, err
	}

	items := make([]model.NotificationResponse, len(rows))
	for i, row := range rows {
		items[i] = model.NotificationResponse{
			ID:        row.ID,
			Type:      row.Type,
			Source:    row.Source,
			Title:     row.Title,
			Content:   row.Content,
			CreatedAt: row.CreatedAt,
			IsRead:    row.ReadAt != nil,
		}
	}

	return items, total, nil
}

// CountUnreadForUser returns the number of unread notifications for a user.
func (r *NotificationRepo) CountUnreadForUser(userID uint) (int64, error) {
	var count int64
	err := r.db.Table("notifications").
		Joins("LEFT JOIN notification_reads ON notification_reads.notification_id = notifications.id AND notification_reads.user_id = ?", userID).
		Where("notifications.deleted_at IS NULL").
		Where("notifications.target_type = ? OR (notifications.target_type = ? AND notifications.target_id = ?)",
			model.NotificationTargetAll, model.NotificationTargetUser, userID).
		Where("notification_reads.id IS NULL").
		Count(&count).Error
	return count, err
}

// MarkAsRead marks a single notification as read for a user (idempotent).
func (r *NotificationRepo) MarkAsRead(notificationID, userID uint) error {
	nr := model.NotificationRead{
		NotificationID: notificationID,
		UserID:         userID,
		ReadAt:         time.Now(),
	}
	return r.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&nr).Error
}

// MarkAllAsRead marks all unread notifications as read for a user.
func (r *NotificationRepo) MarkAllAsRead(userID uint) error {
	// Find all unread notification IDs for this user
	var ids []uint
	err := r.db.Table("notifications").
		Select("notifications.id").
		Joins("LEFT JOIN notification_reads ON notification_reads.notification_id = notifications.id AND notification_reads.user_id = ?", userID).
		Where("notifications.deleted_at IS NULL").
		Where("notifications.target_type = ? OR (notifications.target_type = ? AND notifications.target_id = ?)",
			model.NotificationTargetAll, model.NotificationTargetUser, userID).
		Where("notification_reads.id IS NULL").
		Find(&ids).Error
	if err != nil {
		return err
	}

	if len(ids) == 0 {
		return nil
	}

	now := time.Now()
	records := make([]model.NotificationRead, len(ids))
	for i, id := range ids {
		records[i] = model.NotificationRead{
			NotificationID: id,
			UserID:         userID,
			ReadAt:         now,
		}
	}

	return r.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&records).Error
}

// ListAnnouncements returns announcements with creator username for the management page.
func (r *NotificationRepo) ListAnnouncements(params ListParams) ([]model.AnnouncementResponse, int64, error) {
	baseQuery := r.db.Model(&model.Notification{}).Where("type = ?", model.NotificationTypeAnnouncement)

	if params.Keyword != "" {
		like := "%" + params.Keyword + "%"
		baseQuery = baseQuery.Where("title LIKE ?", like)
	}

	var total int64
	if err := baseQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if params.Page < 1 {
		params.Page = 1
	}
	if params.PageSize < 1 {
		params.PageSize = 20
	}
	offset := (params.Page - 1) * params.PageSize

	type row struct {
		ID             uint
		Title          string
		Content        string
		CreatedAt      time.Time
		UpdatedAt      time.Time
		CreatorUsername string
	}

	var rows []row
	query := r.db.Table("notifications").
		Select("notifications.id, notifications.title, notifications.content, notifications.created_at, notifications.updated_at, users.username AS creator_username").
		Joins("LEFT JOIN users ON users.id = notifications.created_by").
		Where("notifications.type = ?", model.NotificationTypeAnnouncement).
		Where("notifications.deleted_at IS NULL")

	if params.Keyword != "" {
		like := "%" + params.Keyword + "%"
		query = query.Where("notifications.title LIKE ?", like)
	}

	err := query.Order("notifications.created_at DESC").
		Offset(offset).Limit(params.PageSize).
		Find(&rows).Error
	if err != nil {
		return nil, 0, err
	}

	items := make([]model.AnnouncementResponse, len(rows))
	for i, row := range rows {
		items[i] = model.AnnouncementResponse{
			ID:             row.ID,
			Title:          row.Title,
			Content:        row.Content,
			CreatedAt:      row.CreatedAt,
			UpdatedAt:      row.UpdatedAt,
			CreatorUsername: row.CreatorUsername,
		}
	}

	return items, total, nil
}

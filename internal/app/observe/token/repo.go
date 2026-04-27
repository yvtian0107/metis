package token

import (
	. "metis/internal/app/observe/domain"
	"time"

	"github.com/samber/do/v2"

	"metis/internal/database"
)

type IntegrationTokenRepo struct {
	db *database.DB
}

func NewIntegrationTokenRepo(i do.Injector) (*IntegrationTokenRepo, error) {
	db := do.MustInvoke[*database.DB](i)
	return &IntegrationTokenRepo{db: db}, nil
}

func (r *IntegrationTokenRepo) Create(t *IntegrationToken) error {
	return r.db.Create(t).Error
}

// ListByUserID returns all non-revoked tokens for a user.
func (r *IntegrationTokenRepo) ListByUserID(userID uint) ([]IntegrationToken, error) {
	var tokens []IntegrationToken
	err := r.db.Where("user_id = ? AND revoked_at IS NULL", userID).
		Order("created_at DESC").
		Find(&tokens).Error
	return tokens, err
}

// CountByUserID returns the number of active (non-revoked) tokens for a user.
func (r *IntegrationTokenRepo) CountByUserID(userID uint) (int64, error) {
	var count int64
	err := r.db.Model(&IntegrationToken{}).
		Where("user_id = ? AND revoked_at IS NULL", userID).
		Count(&count).Error
	return count, err
}

// FindByPrefix returns non-revoked tokens matching the given prefix (for auth lookup).
func (r *IntegrationTokenRepo) FindByPrefix(prefix string) ([]IntegrationToken, error) {
	var tokens []IntegrationToken
	err := r.db.Where("token_prefix = ? AND revoked_at IS NULL", prefix).
		Find(&tokens).Error
	return tokens, err
}

// FindByIDAndUserID returns a token by ID, scoped to a specific user.
func (r *IntegrationTokenRepo) FindByIDAndUserID(id, userID uint) (*IntegrationToken, error) {
	var t IntegrationToken
	err := r.db.Where("id = ? AND user_id = ? AND revoked_at IS NULL", id, userID).
		First(&t).Error
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// Revoke sets revoked_at on a token.
func (r *IntegrationTokenRepo) Revoke(id uint) error {
	now := time.Now()
	return r.db.Model(&IntegrationToken{}).
		Where("id = ?", id).
		Update("revoked_at", now).Error
}

// UpdateLastUsed sets last_used_at to now.
func (r *IntegrationTokenRepo) UpdateLastUsed(id uint) error {
	now := time.Now()
	return r.db.Model(&IntegrationToken{}).
		Where("id = ?", id).
		Update("last_used_at", now).Error
}

package repository

import (
	"time"

	"github.com/samber/do/v2"
	"gorm.io/gorm"

	"metis/internal/database"
	"metis/internal/model"
)

type UserRepo struct {
	db *database.DB
}

func NewUser(i do.Injector) (*UserRepo, error) {
	db := do.MustInvoke[*database.DB](i)
	return &UserRepo{db: db}, nil
}

func (r *UserRepo) FindByUsername(username string) (*model.User, error) {
	var user model.User
	if err := r.db.Preload("Role").Where("username = ?", username).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepo) FindByID(id uint) (*model.User, error) {
	var user model.User
	if err := r.db.Preload("Role").First(&user, id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// FindByIDWithManager loads a user with Role and direct Manager (one level).
func (r *UserRepo) FindByIDWithManager(id uint) (*model.User, error) {
	var user model.User
	if err := r.db.Preload("Role").Preload("Manager").First(&user, id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepo) FindByEmail(email string) (*model.User, error) {
	var user model.User
	if err := r.db.Preload("Role").Where("email = ? AND email != ''", email).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// ListParams holds query parameters for listing users.
type ListParams struct {
	Keyword   string
	IsActive  *bool
	Page      int
	PageSize  int
	DeptScope *[]uint // nil = no filter; &[]uint{} = self only (unused for users); &[]uint{1,2} = dept filter
}

// ListResult holds the paginated result.
type ListResult struct {
	Items []model.User `json:"items"`
	Total int64        `json:"total"`
}

func (r *UserRepo) List(params ListParams) (*ListResult, error) {
	query := r.db.Model(&model.User{})

	if params.Keyword != "" {
		like := "%" + params.Keyword + "%"
		query = query.Where("username LIKE ? OR email LIKE ? OR phone LIKE ?", like, like, like)
	}
	if params.IsActive != nil {
		query = query.Where("is_active = ?", *params.IsActive)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}

	if params.Page < 1 {
		params.Page = 1
	}
	if params.PageSize < 1 {
		params.PageSize = 20
	}

	var users []model.User
	offset := (params.Page - 1) * params.PageSize
	if err := query.Preload("Role").Preload("Manager").Offset(offset).Limit(params.PageSize).Order("id DESC").Find(&users).Error; err != nil {
		return nil, err
	}

	return &ListResult{Items: users, Total: total}, nil
}

func (r *UserRepo) Create(user *model.User) error {
	return r.db.Create(user).Error
}

func (r *UserRepo) Update(user *model.User) error {
	return r.db.Save(user).Error
}

func (r *UserRepo) Delete(id uint) error {
	result := r.db.Delete(&model.User{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *UserRepo) ExistsByUsername(username string) (bool, error) {
	var count int64
	if err := r.db.Model(&model.User{}).Where("username = ?", username).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// IncrementFailedAttempts atomically increments the failed login counter.
func (r *UserRepo) IncrementFailedAttempts(userID uint) error {
	return r.db.Model(&model.User{}).Where("id = ?", userID).
		UpdateColumn("failed_login_attempts", gorm.Expr("failed_login_attempts + 1")).Error
}

// LockUser sets LockedUntil to now + duration and persists it.
func (r *UserRepo) LockUser(userID uint, duration time.Duration) error {
	until := time.Now().Add(duration)
	return r.db.Model(&model.User{}).Where("id = ?", userID).
		Updates(map[string]any{"locked_until": until}).Error
}

// UnlockUser resets FailedLoginAttempts and LockedUntil.
func (r *UserRepo) UnlockUser(userID uint) error {
	return r.db.Model(&model.User{}).Where("id = ?", userID).
		Updates(map[string]any{"failed_login_attempts": 0, "locked_until": nil}).Error
}

// ResetFailedAttempts clears login failure tracking on successful login.
func (r *UserRepo) ResetFailedAttempts(userID uint) error {
	return r.db.Model(&model.User{}).Where("id = ?", userID).
		Updates(map[string]any{"failed_login_attempts": 0, "locked_until": nil}).Error
}

// GetFailedAttempts returns the current failed login attempt count.
func (r *UserRepo) GetFailedAttempts(userID uint) (int, error) {
	var user model.User
	if err := r.db.Select("failed_login_attempts").First(&user, userID).Error; err != nil {
		return 0, err
	}
	return user.FailedLoginAttempts, nil
}

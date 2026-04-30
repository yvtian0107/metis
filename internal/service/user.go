package service

import (
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/samber/do/v2"
	"gorm.io/gorm"

	"metis/internal/model"
	"metis/internal/pkg/token"
	"metis/internal/repository"
)

var (
	ErrUsernameExists      = errors.New("error.user.username_exists")
	ErrUserNotFound        = errors.New("error.user.not_found")
	ErrCannotSelf          = errors.New("error.user.cannot_self")
	ErrPasswordViolation   = errors.New("error.user.password_violation")
	ErrCircularManagerChain = errors.New("error.user.circular_manager_chain")
)

type UserService struct {
	userRepo         *repository.UserRepo
	refreshTokenRepo *repository.RefreshTokenRepo
	settingsSvc      *SettingsService
}

func NewUser(i do.Injector) (*UserService, error) {
	userRepo := do.MustInvoke[*repository.UserRepo](i)
	refreshTokenRepo := do.MustInvoke[*repository.RefreshTokenRepo](i)
	settingsSvc := do.MustInvoke[*SettingsService](i)
	return &UserService{
		userRepo:         userRepo,
		refreshTokenRepo: refreshTokenRepo,
		settingsSvc:      settingsSvc,
	}, nil
}

func (s *UserService) List(params repository.ListParams) (*repository.ListResult, error) {
	return s.userRepo.List(params)
}

func (s *UserService) GetByID(id uint) (*model.User, error) {
	user, err := s.userRepo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return user, nil
}

func (s *UserService) GetByIDWithManager(id uint) (*model.User, error) {
	user, err := s.userRepo.FindByIDWithManager(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return user, nil
}

type CreateUserParams struct {
	Username  string
	Password  string
	Email     string
	Phone     string
	RoleID    uint
	ManagerID *uint
}

func (s *UserService) Create(username, password, email, phone string, roleID uint) (*model.User, error) {
	return s.CreateWithParams(CreateUserParams{
		Username: username,
		Password: password,
		Email:    email,
		Phone:    phone,
		RoleID:   roleID,
	})
}

func (s *UserService) CreateWithParams(params CreateUserParams) (*model.User, error) {
	// Validate password policy
	if violations := token.ValidatePassword(params.Password, s.settingsSvc.GetPasswordPolicy()); len(violations) > 0 {
		return nil, fmt.Errorf("%w: %s", ErrPasswordViolation, violations[0])
	}

	exists, err := s.userRepo.ExistsByUsername(params.Username)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrUsernameExists
	}

	hashed, err := token.HashPassword(params.Password)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	user := &model.User{
		Username:          params.Username,
		Password:          hashed,
		Email:             params.Email,
		Phone:             params.Phone,
		RoleID:            params.RoleID,
		ManagerID:         params.ManagerID,
		IsActive:          true,
		PasswordChangedAt: &now,
	}

	if err := s.userRepo.Create(user); err != nil {
		return nil, err
	}
	// Reload to get Role + Manager associations.
	return s.userRepo.FindByIDWithManager(user.ID)
}

type UpdateUserParams struct {
	Email     *string
	Phone     *string
	Avatar    *string
	Locale    *string
	Timezone  *string
	RoleID    *uint
	IsActive  *bool
	ManagerID *uint // nil = no change; use SetManagerID to clear
}

// SetManagerID is a sentinel to distinguish "not set" from "clear to null".
// Pass ManagerID = nil to not touch the field; pass with ClearManager=true to set null.
type UpdateUserParamsExt struct {
	UpdateUserParams
	ClearManager bool
}

func (s *UserService) Update(id, currentUserID uint, params UpdateUserParams) (*model.User, error) {
	user, err := s.userRepo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	// Cannot change own role
	if params.RoleID != nil && id == currentUserID {
		return nil, ErrCannotSelf
	}

	if params.Email != nil {
		user.Email = *params.Email
	}
	if params.Phone != nil {
		user.Phone = *params.Phone
	}
	if params.Avatar != nil {
		user.Avatar = *params.Avatar
	}
	if params.Locale != nil {
		user.Locale = *params.Locale
	}
	if params.Timezone != nil {
		user.Timezone = *params.Timezone
	}
	if params.RoleID != nil {
		user.RoleID = *params.RoleID
	}
	if params.IsActive != nil {
		user.IsActive = *params.IsActive
	}
	if params.ManagerID != nil {
		if err := s.checkManagerCycle(id, *params.ManagerID); err != nil {
			return nil, err
		}
		user.ManagerID = params.ManagerID
	}

	if err := s.userRepo.Update(user); err != nil {
		return nil, err
	}
	// Reload to get updated Role + Manager associations
	return s.userRepo.FindByIDWithManager(user.ID)
}

// ClearManager sets a user's managerID to null.
func (s *UserService) ClearManager(id uint) (*model.User, error) {
	user, err := s.userRepo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	user.ManagerID = nil
	if err := s.userRepo.Update(user); err != nil {
		return nil, err
	}
	return s.userRepo.FindByIDWithManager(user.ID)
}

// checkManagerCycle prevents circular manager chains (BFS, max depth 10).
func (s *UserService) checkManagerCycle(userID, newManagerID uint) error {
	if userID == newManagerID {
		return ErrCircularManagerChain
	}
	const maxDepth = 10
	visited := map[uint]bool{userID: true}
	current := newManagerID
	for i := 0; i < maxDepth; i++ {
		if visited[current] {
			return ErrCircularManagerChain
		}
		mgr, err := s.userRepo.FindByID(current)
		if err != nil {
			return nil // manager not found → no cycle
		}
		if mgr.ManagerID == nil {
			return nil // reached root
		}
		visited[current] = true
		current = *mgr.ManagerID
	}
	return nil
}

// GetManagerChain returns the ordered list of managers from direct manager up to root (max 10).
func (s *UserService) GetManagerChain(userID uint) ([]*model.User, error) {
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	var chain []*model.User
	current := user.ManagerID
	visited := map[uint]bool{userID: true}
	for current != nil && len(chain) < 10 {
		if visited[*current] {
			break // cycle guard
		}
		mgr, err := s.userRepo.FindByID(*current)
		if err != nil {
			break
		}
		chain = append(chain, mgr)
		visited[*current] = true
		current = mgr.ManagerID
	}
	return chain, nil
}

func (s *UserService) Delete(id, currentUserID uint) error {
	if id == currentUserID {
		return ErrCannotSelf
	}

	if err := s.userRepo.Delete(id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrUserNotFound
		}
		return err
	}

	// Revoke all refresh tokens for deleted user
	return s.refreshTokenRepo.RevokeAllForUser(id)
}

func (s *UserService) ResetPassword(id uint, newPassword string) error {
	// Validate password policy
	if violations := token.ValidatePassword(newPassword, s.settingsSvc.GetPasswordPolicy()); len(violations) > 0 {
		return fmt.Errorf("%w: %s", ErrPasswordViolation, violations[0])
	}

	user, err := s.userRepo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrUserNotFound
		}
		return err
	}

	hashed, err := token.HashPassword(newPassword)
	if err != nil {
		return err
	}
	now := time.Now()
	user.Password = hashed
	user.PasswordChangedAt = &now
	user.ForcePasswordReset = false
	if err := s.userRepo.Update(user); err != nil {
		return err
	}

	return s.refreshTokenRepo.RevokeAllForUser(id)
}

func (s *UserService) Activate(id uint) (*model.User, error) {
	user, err := s.userRepo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	user.IsActive = true
	if err := s.userRepo.Update(user); err != nil {
		return nil, err
	}
	// Reload to get updated Role association
	return s.userRepo.FindByID(user.ID)
}

func (s *UserService) UnlockUser(id uint) error {
	_, err := s.userRepo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrUserNotFound
		}
		return err
	}
	return s.userRepo.UnlockUser(id)
}

func (s *UserService) Deactivate(id, currentUserID uint) (*model.User, error) {
	if id == currentUserID {
		return nil, ErrCannotSelf
	}

	user, err := s.userRepo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	user.IsActive = false
	if err := s.userRepo.Update(user); err != nil {
		return nil, err
	}

	if err := s.refreshTokenRepo.RevokeAllForUser(id); err != nil {
		slog.Error("failed to revoke tokens on user deactivation", "userId", id, "error", err)
	}
	// Reload to get updated Role association
	return s.userRepo.FindByID(id)
}

// UpdateProfile updates only profile fields (locale, timezone) for self-service.
func (s *UserService) UpdateProfile(user *model.User) error {
	return s.userRepo.Update(user)
}

package model

import "time"

const (
	RoleAdmin = "admin"
	RoleUser  = "user"
)

type User struct {
	BaseModel
	Username            string     `json:"username" gorm:"uniqueIndex;size:64;not null"`
	Password            string     `json:"-" gorm:"size:255"`
	Email               string     `json:"email" gorm:"size:255"`
	Phone               string     `json:"phone" gorm:"size:32"`
	Avatar              string     `json:"avatar" gorm:"size:512"`
	Locale              string     `json:"locale" gorm:"size:10"`
	Timezone            string     `json:"timezone" gorm:"size:50"`
	RoleID              uint       `json:"roleId" gorm:"not null;default:0"`
	Role                Role       `json:"role" gorm:"foreignKey:RoleID"`
	IsActive            bool       `json:"isActive" gorm:"not null;default:true"`
	PasswordChangedAt   *time.Time `json:"passwordChangedAt,omitempty" gorm:"default:null"`
	ForcePasswordReset  bool       `json:"forcePasswordReset" gorm:"not null;default:false"`
	FailedLoginAttempts int        `json:"-" gorm:"not null;default:0"`
	LockedUntil         *time.Time `json:"-" gorm:"default:null"`
	TwoFactorEnabled    bool       `json:"twoFactorEnabled" gorm:"not null;default:false"`
}

// HasPassword returns true if the user has a password set (not an OAuth-only user).
func (u *User) HasPassword() bool {
	return u.Password != ""
}

// UserResponse is the safe representation excluding password.
type UserResponse struct {
	ID                  uint                     `json:"id"`
	Username            string                   `json:"username"`
	Email               string                   `json:"email"`
	Phone               string                   `json:"phone"`
	Avatar              string                   `json:"avatar"`
	Locale              string                   `json:"locale"`
	Timezone            string                   `json:"timezone"`
	Role                RoleResponse             `json:"role"`
	IsActive            bool                     `json:"isActive"`
	HasPassword         bool                     `json:"hasPassword"`
	TwoFactorEnabled    bool                     `json:"twoFactorEnabled"`
	PasswordChangedAt   *time.Time               `json:"passwordChangedAt,omitempty"`
	ForcePasswordReset  bool                     `json:"forcePasswordReset"`
	FailedLoginAttempts int                      `json:"failedLoginAttempts"`
	LockedUntil         *time.Time               `json:"lockedUntil,omitempty"`
	Connections         []UserConnectionResponse `json:"connections,omitempty"`
	CreatedAt           time.Time                `json:"createdAt"`
	UpdatedAt           time.Time                `json:"updatedAt"`
}

func (u *User) ToResponse() UserResponse {
	return UserResponse{
		ID:       u.ID,
		Username: u.Username,
		Email:    u.Email,
		Phone:    u.Phone,
		Avatar:   u.Avatar,
		Locale:   u.Locale,
		Timezone: u.Timezone,
		Role: RoleResponse{
			ID:   u.Role.ID,
			Name: u.Role.Name,
			Code: u.Role.Code,
		},
		IsActive:            u.IsActive,
		HasPassword:         u.HasPassword(),
		TwoFactorEnabled:    u.TwoFactorEnabled,
		PasswordChangedAt:   u.PasswordChangedAt,
		ForcePasswordReset:  u.ForcePasswordReset,
		FailedLoginAttempts: u.FailedLoginAttempts,
		LockedUntil:         u.LockedUntil,
		CreatedAt:           u.CreatedAt,
		UpdatedAt:           u.UpdatedAt,
	}
}

// IsLocked returns true if the account is currently locked.
func (u *User) IsLocked() bool {
	return u.LockedUntil != nil && u.LockedUntil.After(time.Now())
}

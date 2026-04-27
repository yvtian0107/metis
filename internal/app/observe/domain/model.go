package domain

import "time"

// IntegrationToken represents a user-owned token used to authenticate
// OTLP data ingestion via Traefik ForwardAuth.
type IntegrationToken struct {
	ID          uint       `json:"id"          gorm:"primaryKey"`
	UserID      uint       `json:"userId"      gorm:"not null;index"`
	OrgID       *uint      `json:"orgId"       gorm:"index"` // reserved for org-level tokens
	Scope       string     `json:"scope"       gorm:"not null;default:'personal';size:32"`
	Name        string     `json:"name"        gorm:"not null;size:100"`
	TokenHash   string     `json:"-"           gorm:"not null;size:255"`
	TokenPrefix string     `json:"-"           gorm:"not null;size:16;index"` // for auth lookup
	TokenPlain  string     `json:"-"           gorm:"not null;size:100"`      // stored for display
	LastUsedAt  *time.Time `json:"lastUsedAt"`
	RevokedAt   *time.Time `json:"revokedAt"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
}

// TokenResponse is returned to clients.
type TokenResponse struct {
	ID         uint       `json:"id"`
	Name       string     `json:"name"`
	Token      string     `json:"token"`
	Scope      string     `json:"scope"`
	LastUsedAt *time.Time `json:"lastUsedAt"`
	CreatedAt  time.Time  `json:"createdAt"`
}

func (t *IntegrationToken) ToResponse() TokenResponse {
	return TokenResponse{
		ID:         t.ID,
		Name:       t.Name,
		Token:      t.TokenPlain,
		Scope:      t.Scope,
		LastUsedAt: t.LastUsedAt,
		CreatedAt:  t.CreatedAt,
	}
}

// CreateTokenResponse is returned only once at creation time.
type CreateTokenResponse struct {
	TokenResponse
}

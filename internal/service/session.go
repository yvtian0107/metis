package service

import (
	"errors"
	"time"

	"github.com/samber/do/v2"

	"metis/internal/pkg/token"
	"metis/internal/repository"
)

var (
	ErrSessionNotFound = errors.New("error.session.not_found")
	ErrCannotKickSelf  = errors.New("error.session.cannot_kick_self")
)

type SessionService struct {
	refreshTokenRepo *repository.RefreshTokenRepo
	blacklist        *token.TokenBlacklist
}

func NewSession(i do.Injector) (*SessionService, error) {
	return &SessionService{
		refreshTokenRepo: do.MustInvoke[*repository.RefreshTokenRepo](i),
		blacklist:        do.MustInvoke[*token.TokenBlacklist](i),
	}, nil
}

type SessionItem struct {
	ID         uint      `json:"id"`
	UserID     uint      `json:"userId"`
	Username   string    `json:"username"`
	IPAddress  string    `json:"ipAddress"`
	UserAgent  string    `json:"userAgent"`
	LoginAt    time.Time `json:"loginAt"`
	LastSeenAt time.Time `json:"lastSeenAt"`
	IsCurrent  bool      `json:"isCurrent"`
}

type SessionListResult struct {
	Items []SessionItem `json:"items"`
	Total int64         `json:"total"`
}

func (s *SessionService) ListSessions(page, pageSize int, currentJTI string) (*SessionListResult, error) {
	sessions, total, err := s.refreshTokenRepo.GetActiveSessions(page, pageSize)
	if err != nil {
		return nil, err
	}

	items := make([]SessionItem, len(sessions))
	for i, sess := range sessions {
		items[i] = SessionItem{
			ID:         sess.ID,
			UserID:     sess.UserID,
			Username:   sess.Username,
			IPAddress:  sess.IPAddress,
			UserAgent:  sess.UserAgent,
			LoginAt:    sess.LoginAt,
			LastSeenAt: sess.LastSeenAt,
			IsCurrent:  sess.AccessTokenJTI == currentJTI,
		}
	}

	return &SessionListResult{Items: items, Total: total}, nil
}

func (s *SessionService) KickSession(sessionID uint, currentJTI string) error {
	rt, err := s.refreshTokenRepo.FindByID(sessionID)
	if err != nil {
		return ErrSessionNotFound
	}

	if rt.Revoked {
		return ErrSessionNotFound
	}
	if !rt.ExpiresAt.After(time.Now()) {
		return ErrSessionNotFound
	}

	// Prevent self-kick
	if rt.AccessTokenJTI == currentJTI {
		return ErrCannotKickSelf
	}

	// Revoke refresh token
	if err := s.refreshTokenRepo.RevokeByID(sessionID); err != nil {
		return err
	}

	// Blacklist the access token for immediate effect
	if rt.AccessTokenJTI != "" {
		s.blacklist.Add(rt.AccessTokenJTI, time.Now().Add(token.AccessTokenDuration))
	}

	return nil
}

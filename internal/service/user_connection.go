package service

import (
	"errors"
	"fmt"

	"github.com/samber/do/v2"

	"metis/internal/model"
	"metis/internal/repository"
)

var (
	ErrAlreadyBound        = errors.New("error.connection.already_bound")
	ErrExternalIDBound     = errors.New("error.connection.external_id_bound")
	ErrLastLoginMethod     = errors.New("error.connection.last_login_method")
	ErrConnectionNotFound  = errors.New("error.connection.not_found")
)

type UserConnectionService struct {
	connRepo *repository.UserConnectionRepo
	userRepo *repository.UserRepo
}

func NewUserConnection(i do.Injector) (*UserConnectionService, error) {
	return &UserConnectionService{
		connRepo: do.MustInvoke[*repository.UserConnectionRepo](i),
		userRepo: do.MustInvoke[*repository.UserRepo](i),
	}, nil
}

func (s *UserConnectionService) ListByUser(userID uint) ([]model.UserConnection, error) {
	return s.connRepo.FindByUserID(userID)
}

func (s *UserConnectionService) Bind(userID uint, provider, externalID, externalName, externalEmail, avatarURL string) error {
	// Check if already bound to this provider
	if _, err := s.connRepo.FindByUserAndProvider(userID, provider); err == nil {
		return ErrAlreadyBound
	}

	// Check if this external identity is already bound to another user
	if existing, err := s.connRepo.FindByProviderAndExternalID(provider, externalID); err == nil && existing.UserID != userID {
		return ErrExternalIDBound
	}

	conn := &model.UserConnection{
		UserID:        userID,
		Provider:      provider,
		ExternalID:    externalID,
		ExternalName:  externalName,
		ExternalEmail: externalEmail,
		AvatarURL:     avatarURL,
	}
	return s.connRepo.Create(conn)
}

func (s *UserConnectionService) Unbind(userID uint, provider string) error {
	conn, err := s.connRepo.FindByUserAndProvider(userID, provider)
	if err != nil {
		return ErrConnectionNotFound
	}

	// Check if this is the last login method
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return fmt.Errorf("find user: %w", err)
	}

	if !user.HasPassword() {
		count, err := s.connRepo.CountByUserID(userID)
		if err != nil {
			return fmt.Errorf("count connections: %w", err)
		}
		if count <= 1 {
			return ErrLastLoginMethod
		}
	}

	return s.connRepo.Delete(conn.ID)
}

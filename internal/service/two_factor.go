package service

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"

	"github.com/pquerna/otp/totp"
	"github.com/samber/do/v2"
	"gorm.io/gorm"

	"metis/internal/model"
	"metis/internal/repository"
)

var (
	ErrTwoFactorAlreadyEnabled = errors.New("error.2fa.already_enabled")
	ErrTwoFactorNotSetup       = errors.New("error.2fa.not_setup")
	ErrTwoFactorInvalidCode    = errors.New("error.2fa.invalid_code")
)

type TwoFactorService struct {
	tfRepo   *repository.TwoFactorSecretRepo
	userRepo *repository.UserRepo
}

func NewTwoFactor(i do.Injector) (*TwoFactorService, error) {
	return &TwoFactorService{
		tfRepo:   do.MustInvoke[*repository.TwoFactorSecretRepo](i),
		userRepo: do.MustInvoke[*repository.UserRepo](i),
	}, nil
}

// SetupResult holds the TOTP secret and provisioning URI for QR code.
type SetupResult struct {
	Secret string `json:"secret"`
	QRUri  string `json:"qrUri"`
}

// Setup generates a new TOTP secret for the user. Does not persist until Confirm.
func (s *TwoFactorService) Setup(userID uint) (*SetupResult, error) {
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return nil, err
	}
	if user.TwoFactorEnabled {
		return nil, ErrTwoFactorAlreadyEnabled
	}

	// Delete any pending (unconfirmed) secret
	_ = s.tfRepo.DeleteByUserID(userID)

	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "Metis",
		AccountName: user.Username,
	})
	if err != nil {
		return nil, fmt.Errorf("generate TOTP key: %w", err)
	}

	// Store secret (not yet confirmed — TwoFactorEnabled remains false)
	secret := &model.TwoFactorSecret{
		UserID: userID,
		Secret: key.Secret(),
	}
	if err := s.tfRepo.Create(secret); err != nil {
		return nil, err
	}

	return &SetupResult{
		Secret: key.Secret(),
		QRUri:  key.URL(),
	}, nil
}

// ConfirmResult holds backup codes generated on confirm.
type ConfirmResult struct {
	BackupCodes []string `json:"backupCodes"`
}

// Confirm verifies the TOTP code and enables 2FA for the user.
func (s *TwoFactorService) Confirm(userID uint, code string) (*ConfirmResult, error) {
	secret, err := s.tfRepo.FindByUserID(userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTwoFactorNotSetup
		}
		return nil, err
	}

	if !totp.Validate(code, secret.Secret) {
		return nil, ErrTwoFactorInvalidCode
	}

	// Generate backup codes
	backupCodes := generateBackupCodes(8)
	codesJSON, _ := json.Marshal(backupCodes)
	secret.BackupCodes = string(codesJSON)
	if err := s.tfRepo.Update(secret); err != nil {
		return nil, err
	}

	// Enable 2FA on user
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return nil, err
	}
	user.TwoFactorEnabled = true
	if err := s.userRepo.Update(user); err != nil {
		return nil, err
	}

	return &ConfirmResult{BackupCodes: backupCodes}, nil
}

// Verify checks a TOTP code or backup code for the user.
func (s *TwoFactorService) Verify(userID uint, code string) (bool, error) {
	secret, err := s.tfRepo.FindByUserID(userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, ErrTwoFactorNotSetup
		}
		return false, err
	}

	// Try TOTP first
	if totp.Validate(code, secret.Secret) {
		return true, nil
	}

	// Try backup code
	var codes []string
	if secret.BackupCodes != "" {
		_ = json.Unmarshal([]byte(secret.BackupCodes), &codes)
	}
	for i, c := range codes {
		if c == code {
			// Consume the backup code
			codes = append(codes[:i], codes[i+1:]...)
			codesJSON, _ := json.Marshal(codes)
			secret.BackupCodes = string(codesJSON)
			_ = s.tfRepo.Update(secret)
			return true, nil
		}
	}

	return false, nil
}

// Disable removes 2FA for the user.
func (s *TwoFactorService) Disable(userID uint) error {
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return err
	}
	if !user.TwoFactorEnabled {
		return ErrTwoFactorNotSetup
	}

	if err := s.tfRepo.DeleteByUserID(userID); err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	user.TwoFactorEnabled = false
	return s.userRepo.Update(user)
}

func generateBackupCodes(count int) []string {
	codes := make([]string, count)
	for i := range codes {
		n, _ := rand.Int(rand.Reader, big.NewInt(100000000))
		codes[i] = fmt.Sprintf("%08d", n.Int64())
	}
	return codes
}

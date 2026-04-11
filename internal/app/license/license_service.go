package license

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/samber/do/v2"
	"gorm.io/gorm"

	"metis/internal/database"
)

var (
	ErrLicenseNotFound        = errors.New("error.license.not_found")
	ErrLicenseAlreadyRevoked  = errors.New("error.license.already_revoked")
	ErrProductNotPublished    = errors.New("error.license.product_not_published")
	ErrLicenseeNotActive      = errors.New("error.license.licensee_not_active")
	ErrProductKeyNotFound     = errors.New("error.license.product_key_not_found")
	ErrRevokedLicenseNoExport = errors.New("error.license.revoked_no_export")
)

type LicenseService struct {
	licenseRepo  *LicenseRepo
	productRepo  *ProductRepo
	licenseeRepo *LicenseeRepo
	keyRepo      *ProductKeyRepo
	db           *database.DB
	jwtSecret    []byte
}

func NewLicenseService(i do.Injector) (*LicenseService, error) {
	return &LicenseService{
		licenseRepo:  do.MustInvoke[*LicenseRepo](i),
		productRepo:  do.MustInvoke[*ProductRepo](i),
		licenseeRepo: do.MustInvoke[*LicenseeRepo](i),
		keyRepo:      do.MustInvoke[*ProductKeyRepo](i),
		db:           do.MustInvoke[*database.DB](i),
		jwtSecret:    do.MustInvoke[[]byte](i),
	}, nil
}

type IssueLicenseParams struct {
	ProductID        uint
	LicenseeID       uint
	PlanID           *uint
	PlanName         string
	RegistrationCode string
	ConstraintValues json.RawMessage
	ValidFrom        time.Time
	ValidUntil       *time.Time
	Notes            string
	IssuedBy         uint
}

func (s *LicenseService) IssueLicense(params IssueLicenseParams) (*License, error) {
	// Validate product
	product, err := s.productRepo.FindByID(params.ProductID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrProductNotFound
		}
		return nil, err
	}
	if product.Status != StatusPublished {
		return nil, ErrProductNotPublished
	}

	// Validate licensee
	licensee, err := s.licenseeRepo.FindByID(params.LicenseeID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrLicenseeNotFound
		}
		return nil, err
	}
	if licensee.Status != LicenseeStatusActive {
		return nil, ErrLicenseeNotActive
	}

	// Get current key
	key, err := s.keyRepo.FindCurrentByProductID(params.ProductID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrProductKeyNotFound
		}
		return nil, err
	}

	// Get encryption key
	encKey, err := GetEncryptionKeyWithFallback(s.jwtSecret)
	if err != nil {
		return nil, err
	}

	now := time.Now()

	// Build payload
	var constraintMap map[string]any
	if len(params.ConstraintValues) > 0 {
		if err := json.Unmarshal(params.ConstraintValues, &constraintMap); err != nil {
			return nil, fmt.Errorf("invalid constraint values: %w", err)
		}
	}
	if constraintMap == nil {
		constraintMap = make(map[string]any)
	}

	payload := map[string]any{
		"v":    1,
		"pid":  product.Code,
		"lic":  licensee.Code,
		"licn": licensee.Name,
		"reg":  params.RegistrationCode,
		"con":  constraintMap,
		"iat":  now.Unix(),
		"nbf":  params.ValidFrom.Unix(),
		"exp":  nil,
		"kv":   key.Version,
	}
	if params.ValidUntil != nil {
		payload["exp"] = params.ValidUntil.Unix()
	}

	// Sign
	sig, err := SignLicense(payload, key.EncryptedPrivateKey, encKey)
	if err != nil {
		return nil, fmt.Errorf("sign license: %w", err)
	}

	// Generate activation code
	activationCode, err := GenerateActivationCode(payload, sig)
	if err != nil {
		return nil, fmt.Errorf("generate activation code: %w", err)
	}

	// Create license record in transaction
	license := &License{
		ProductID:        &params.ProductID,
		LicenseeID:       &params.LicenseeID,
		PlanID:           params.PlanID,
		PlanName:         params.PlanName,
		RegistrationCode: params.RegistrationCode,
		ConstraintValues: JSONText(params.ConstraintValues),
		ValidFrom:        params.ValidFrom,
		ValidUntil:       params.ValidUntil,
		ActivationCode:   activationCode,
		KeyVersion:       key.Version,
		Signature:        sig,
		Status:           LicenseStatusIssued,
		IssuedBy:         params.IssuedBy,
		Notes:            params.Notes,
	}

	err = s.db.Transaction(func(tx *gorm.DB) error {
		return s.licenseRepo.CreateInTx(tx, license)
	})
	if err != nil {
		return nil, err
	}

	return license, nil
}

func (s *LicenseService) RevokeLicense(id uint, revokedBy uint) error {
	detail, err := s.licenseRepo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrLicenseNotFound
		}
		return err
	}
	if detail.Status == LicenseStatusRevoked {
		return ErrLicenseAlreadyRevoked
	}

	now := time.Now()
	return s.licenseRepo.UpdateStatus(id, map[string]any{
		"status":     LicenseStatusRevoked,
		"revoked_at": now,
		"revoked_by": revokedBy,
	})
}

func (s *LicenseService) GetLicense(id uint) (*LicenseDetail, error) {
	detail, err := s.licenseRepo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrLicenseNotFound
		}
		return nil, err
	}
	return detail, nil
}

func (s *LicenseService) ListLicenses(params LicenseListParams) ([]LicenseListItem, int64, error) {
	return s.licenseRepo.List(params)
}

type LicFile struct {
	ActivationCode string `json:"activationCode"`
	PublicKey      string `json:"publicKey"`
}

func (s *LicenseService) ExportLicFile(id uint) (string, string, error) {
	detail, err := s.licenseRepo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", "", ErrLicenseNotFound
		}
		return "", "", err
	}
	if detail.Status == LicenseStatusRevoked {
		return "", "", ErrRevokedLicenseNoExport
	}

	// Get the key version used for signing
	if detail.ProductID == nil {
		return "", "", errors.New("license has no associated product")
	}
	key, err := s.keyRepo.FindByProductIDAndVersion(*detail.ProductID, detail.KeyVersion)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", "", ErrProductKeyNotFound
		}
		return "", "", err
	}

	licFile := &LicFile{
		ActivationCode: detail.ActivationCode,
		PublicKey:      key.PublicKey,
	}

	plainJSON, err := json.Marshal(licFile)
	if err != nil {
		return "", "", fmt.Errorf("marshal license file: %w", err)
	}

	productIdentity := detail.ProductName
	if productIdentity == "" {
		productIdentity = detail.ProductCode
	}

	encryptedContent, err := EncryptLicenseFile(plainJSON, detail.RegistrationCode, productIdentity)
	if err != nil {
		return "", "", err
	}

	filename := fmt.Sprintf("%s_%s.lic", detail.ProductCode, detail.CreatedAt.Format("20060102"))
	if detail.ProductCode == "" {
		filename = fmt.Sprintf("license_%s.lic", detail.CreatedAt.Format("20060102"))
	}

	return encryptedContent, filename, nil
}

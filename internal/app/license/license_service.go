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
	ErrLicenseNotFound          = errors.New("error.license.not_found")
	ErrLicenseAlreadyRevoked    = errors.New("error.license.already_revoked")
	ErrLicenseAlreadySuspended  = errors.New("error.license.already_suspended")
	ErrLicenseNotSuspended      = errors.New("error.license.not_suspended")
	ErrProductNotPublished      = errors.New("error.license.product_not_published")
	ErrLicenseeNotActive        = errors.New("error.license.licensee_not_active")
	ErrProductKeyNotFound       = errors.New("error.license.product_key_not_found")
	ErrRevokedLicenseNoExport   = errors.New("error.license.revoked_no_export")
	ErrRegistrationNotFound     = errors.New("error.license.registration_not_found")
	ErrRegistrationAlreadyBound = errors.New("error.license.registration_already_bound")
	ErrRegistrationExpired      = errors.New("error.license.registration_expired")
	ErrInvalidLicenseState      = errors.New("error.license.invalid_state")
	ErrBulkReissueTooMany       = errors.New("error.license.bulk_reissue_too_many")
)

type LicenseService struct {
	licenseRepo  *LicenseRepo
	productRepo  *ProductRepo
	licenseeRepo *LicenseeRepo
	keyRepo      *ProductKeyRepo
	regRepo      *LicenseRegistrationRepo
	db           *database.DB
	jwtSecret    []byte
}

func NewLicenseService(i do.Injector) (*LicenseService, error) {
	return &LicenseService{
		licenseRepo:  do.MustInvoke[*LicenseRepo](i),
		productRepo:  do.MustInvoke[*ProductRepo](i),
		licenseeRepo: do.MustInvoke[*LicenseeRepo](i),
		keyRepo:      do.MustInvoke[*ProductKeyRepo](i),
		regRepo:      do.MustInvoke[*LicenseRegistrationRepo](i),
		db:           do.MustInvoke[*database.DB](i),
		jwtSecret:    do.MustInvoke[[]byte](i),
	}, nil
}

type IssueLicenseParams struct {
	ProductID              uint
	LicenseeID             uint
	PlanID                 *uint
	PlanName               string
	RegistrationCode       string
	AutoCreateRegistration bool
	ConstraintValues       json.RawMessage
	ValidFrom              time.Time
	ValidUntil             *time.Time
	Notes                  string
	IssuedBy               uint
}

type licensePayloadArgs struct {
	ProductCode      string
	LicenseeCode     string
	LicenseeName     string
	RegistrationCode string
	ConstraintValues JSONText
	IssuedAt         time.Time
	ValidFrom        time.Time
	ValidUntil       *time.Time
	KeyVersion       int
}

func buildLicensePayload(args licensePayloadArgs) (map[string]any, error) {
	var constraintMap map[string]any
	if len(args.ConstraintValues) > 0 {
		if err := json.Unmarshal(args.ConstraintValues.RawMessage(), &constraintMap); err != nil {
			return nil, fmt.Errorf("invalid constraint values: %w", err)
		}
	}
	if constraintMap == nil {
		constraintMap = make(map[string]any)
	}

	payload := map[string]any{
		"v":    1,
		"pid":  args.ProductCode,
		"lic":  args.LicenseeCode,
		"licn": args.LicenseeName,
		"reg":  args.RegistrationCode,
		"con":  constraintMap,
		"iat":  args.IssuedAt.Unix(),
		"nbf":  args.ValidFrom.Unix(),
		"exp":  nil,
		"kv":   args.KeyVersion,
	}
	if args.ValidUntil != nil {
		payload["exp"] = args.ValidUntil.Unix()
	}
	return payload, nil
}

func deriveLifecycleStatus(validFrom time.Time, validUntil *time.Time) string {
	now := time.Now()
	if validFrom.After(now) {
		return LicenseLifecyclePending
	}
	if validUntil != nil && !validUntil.After(now) {
		return LicenseLifecycleExpired
	}
	return LicenseLifecycleActive
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
	payload, err := buildLicensePayload(licensePayloadArgs{
		ProductCode:      product.Code,
		LicenseeCode:     licensee.Code,
		LicenseeName:     licensee.Name,
		RegistrationCode: params.RegistrationCode,
		ConstraintValues: JSONText(params.ConstraintValues),
		IssuedAt:         now,
		ValidFrom:        params.ValidFrom,
		ValidUntil:       params.ValidUntil,
		KeyVersion:       key.Version,
	})
	if err != nil {
		return nil, err
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

	// Validate registration code
	var reg *LicenseRegistration
	if params.RegistrationCode != "" {
		r, err := s.regRepo.FindByCode(params.RegistrationCode)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				if params.AutoCreateRegistration {
					r = &LicenseRegistration{
						ProductID:  &params.ProductID,
						LicenseeID: &params.LicenseeID,
						Code:       params.RegistrationCode,
						Source:     "manual_input",
					}
					if err := s.regRepo.Create(r); err != nil {
						return nil, err
					}
				} else {
					return nil, ErrRegistrationNotFound
				}
			} else {
				return nil, err
			}
		}
		if r.BoundLicenseID != nil {
			return nil, ErrRegistrationAlreadyBound
		}
		if r.ExpiresAt != nil && !r.ExpiresAt.After(now) {
			return nil, ErrRegistrationExpired
		}
		reg = r
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
		LifecycleStatus:  deriveLifecycleStatus(params.ValidFrom, params.ValidUntil),
		IssuedBy:         params.IssuedBy,
		Notes:            params.Notes,
	}

	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := s.licenseRepo.CreateInTx(tx, license); err != nil {
			return err
		}
		if reg != nil {
			if err := s.regRepo.UpdateBoundLicenseInTx(tx, reg.ID, license.ID); err != nil {
				return err
			}
		}
		return nil
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
		"status":           LicenseStatusRevoked,
		"lifecycle_status": LicenseLifecycleRevoked,
		"revoked_at":       now,
		"revoked_by":       revokedBy,
	})
}

func (s *LicenseService) RenewLicense(id uint, newValidUntil *time.Time, renewedBy uint) error {
	detail, err := s.licenseRepo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrLicenseNotFound
		}
		return err
	}
	if detail.LifecycleStatus == LicenseLifecycleRevoked {
		return ErrLicenseAlreadyRevoked
	}

	updates := map[string]any{
		"lifecycle_status": LicenseLifecycleActive,
		"valid_until":      newValidUntil,
	}
	return s.licenseRepo.UpdateStatus(id, updates)
}

func (s *LicenseService) UpgradeLicense(id uint, params IssueLicenseParams) (*License, error) {
	original, err := s.licenseRepo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrLicenseNotFound
		}
		return nil, err
	}
	if original.LifecycleStatus == LicenseLifecycleRevoked {
		return nil, ErrLicenseAlreadyRevoked
	}

	// Unbind registration from original license so it can be reused for upgrade
	if original.RegistrationCode != "" && original.RegistrationCode == params.RegistrationCode {
		if err := s.regRepo.UnbindLicenseInTx(s.db, params.RegistrationCode); err != nil {
			return nil, err
		}
	}

	// Issue new license
	newLicense, err := s.IssueLicense(params)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	// Revoke original and link to new
	if err := s.licenseRepo.UpdateStatus(id, map[string]any{
		"status":           LicenseStatusRevoked,
		"lifecycle_status": LicenseLifecycleRevoked,
		"revoked_at":       now,
		"revoked_by":       params.IssuedBy,
	}); err != nil {
		return nil, err
	}

	if err := s.licenseRepo.UpdateStatus(newLicense.ID, map[string]any{
		"original_license_id": id,
	}); err != nil {
		return nil, err
	}

	return newLicense, nil
}

func (s *LicenseService) SuspendLicense(id uint, suspendedBy uint) error {
	detail, err := s.licenseRepo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrLicenseNotFound
		}
		return err
	}
	if detail.LifecycleStatus == LicenseLifecycleRevoked {
		return ErrLicenseAlreadyRevoked
	}
	if detail.LifecycleStatus == LicenseLifecycleSuspended {
		return ErrLicenseAlreadySuspended
	}

	now := time.Now()
	return s.licenseRepo.UpdateStatus(id, map[string]any{
		"lifecycle_status": LicenseLifecycleSuspended,
		"suspended_at":     now,
		"suspended_by":     suspendedBy,
	})
}

func (s *LicenseService) ReactivateLicense(id uint) error {
	detail, err := s.licenseRepo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrLicenseNotFound
		}
		return err
	}
	if detail.LifecycleStatus == LicenseLifecycleRevoked {
		return ErrLicenseAlreadyRevoked
	}
	if detail.LifecycleStatus != LicenseLifecycleSuspended {
		return ErrLicenseNotSuspended
	}

	newStatus := deriveLifecycleStatus(detail.ValidFrom, detail.ValidUntil)
	return s.licenseRepo.UpdateStatus(id, map[string]any{
		"lifecycle_status": newStatus,
		"suspended_at":     nil,
		"suspended_by":     nil,
	})
}

func (s *LicenseService) CheckExpiredLicenses() error {
	now := time.Now()
	expired, err := s.licenseRepo.FindExpired(now)
	if err != nil {
		return err
	}
	for i := range expired {
		if err := s.licenseRepo.UpdateStatus(expired[i].ID, map[string]any{
			"lifecycle_status": LicenseLifecycleExpired,
		}); err != nil {
			return err
		}
	}
	return nil
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
	if detail.LifecycleStatus == LicenseLifecycleRevoked {
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

// --- LicenseRegistration services ---

func generateRegistrationCode() (string, error) {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	return generateRandomCode(charset, 16, "RG-")
}

type CreateLicenseRegistrationParams struct {
	ProductID  *uint
	LicenseeID *uint
	Code       string
	Source     string
	ExpiresAt  *time.Time
}

func (s *LicenseService) CreateLicenseRegistration(params CreateLicenseRegistrationParams) (*LicenseRegistration, error) {
	code := params.Code
	if code == "" {
		c, err := generateRegistrationCode()
		if err != nil {
			return nil, err
		}
		code = c
	}

	exists, err := s.regRepo.CodeExists(code)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, errors.New("error.license.registration_code_exists")
	}

	lr := &LicenseRegistration{
		ProductID:  params.ProductID,
		LicenseeID: params.LicenseeID,
		Code:       code,
		Source:     params.Source,
		ExpiresAt:  params.ExpiresAt,
	}
	if lr.Source == "" {
		lr.Source = "pre_registered"
	}

	if err := s.regRepo.Create(lr); err != nil {
		return nil, err
	}
	return lr, nil
}

func (s *LicenseService) GenerateLicenseRegistration(productID, licenseeID *uint) (*LicenseRegistration, error) {
	var code string
	for i := 0; i < 3; i++ {
		c, err := generateRegistrationCode()
		if err != nil {
			return nil, err
		}
		exists, err := s.regRepo.CodeExists(c)
		if err != nil {
			return nil, err
		}
		if !exists {
			code = c
			break
		}
	}
	if code == "" {
		return nil, errors.New("error.license.registration_code_collision")
	}

	lr := &LicenseRegistration{
		ProductID:  productID,
		LicenseeID: licenseeID,
		Code:       code,
		Source:     "auto_generated",
	}
	if err := s.regRepo.Create(lr); err != nil {
		return nil, err
	}
	return lr, nil
}

func (s *LicenseService) ListLicenseRegistrations(params LicenseRegistrationListParams) ([]LicenseRegistration, int64, error) {
	return s.regRepo.List(params)
}

func (s *LicenseService) CleanupExpiredRegistrations() error {
	return s.regRepo.DeleteExpired(time.Now())
}

// --- Key rotation impact assessment ---

type RotateKeyImpact struct {
	AffectedCount  int64 `json:"affectedCount"`
	CurrentVersion int   `json:"currentVersion"`
}

func (s *LicenseService) AssessKeyRotationImpact(productID uint) (*RotateKeyImpact, error) {
	key, err := s.keyRepo.FindCurrentByProductID(productID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrProductKeyNotFound
		}
		return nil, err
	}

	count, err := s.licenseRepo.CountByProductAndKeyVersionLessThan(productID, key.Version)
	if err != nil {
		return nil, err
	}

	return &RotateKeyImpact{
		AffectedCount:  count,
		CurrentVersion: key.Version,
	}, nil
}

func (s *LicenseService) BulkReissueLicenses(productID uint, ids []uint, issuedBy uint) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	if len(ids) > 100 {
		return 0, ErrBulkReissueTooMany
	}

	key, err := s.keyRepo.FindCurrentByProductID(productID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, ErrProductKeyNotFound
		}
		return 0, err
	}

	encKey, err := GetEncryptionKeyWithFallback(s.jwtSecret)
	if err != nil {
		return 0, err
	}

	reissued := 0
	for _, id := range ids {
		detail, err := s.licenseRepo.FindByID(id)
		if err != nil {
			continue
		}
		if detail.ProductID == nil || *detail.ProductID != productID {
			continue
		}
		if detail.LifecycleStatus == LicenseLifecycleRevoked {
			continue
		}

		// Rebuild payload
		var constraintMap map[string]any
		if len(detail.ConstraintValues) > 0 {
			_ = json.Unmarshal(detail.ConstraintValues.RawMessage(), &constraintMap)
		}
		if constraintMap == nil {
			constraintMap = make(map[string]any)
		}

		payload := map[string]any{
			"v":    1,
			"pid":  detail.ProductCode,
			"lic":  detail.LicenseeCode,
			"licn": detail.LicenseeName,
			"reg":  detail.RegistrationCode,
			"con":  constraintMap,
			"iat":  detail.CreatedAt.Unix(),
			"nbf":  detail.ValidFrom.Unix(),
			"exp":  nil,
			"kv":   key.Version,
		}
		if detail.ValidUntil != nil {
			payload["exp"] = detail.ValidUntil.Unix()
		}

		sig, err := SignLicense(payload, key.EncryptedPrivateKey, encKey)
		if err != nil {
			continue
		}

		activationCode, err := GenerateActivationCode(payload, sig)
		if err != nil {
			continue
		}

		if err := s.licenseRepo.UpdateStatus(id, map[string]any{
			"key_version":     key.Version,
			"signature":       sig,
			"activation_code": activationCode,
		}); err != nil {
			continue
		}
		reissued++
	}

	return reissued, nil
}

package registration

import (
	"metis/internal/app/license/domain"
	"time"

	"github.com/samber/do/v2"
	"gorm.io/gorm"

	"metis/internal/database"
)

type LicenseRegistrationRepo struct {
	DB *database.DB
}

func NewLicenseRegistrationRepo(i do.Injector) (*LicenseRegistrationRepo, error) {
	db := do.MustInvoke[*database.DB](i)
	return &LicenseRegistrationRepo{DB: db}, nil
}

func (r *LicenseRegistrationRepo) Create(lr *domain.LicenseRegistration) error {
	return r.DB.Create(lr).Error
}

func (r *LicenseRegistrationRepo) CreateInTx(tx *gorm.DB, lr *domain.LicenseRegistration) error {
	return tx.Create(lr).Error
}

func (r *LicenseRegistrationRepo) FindByID(id uint) (*domain.LicenseRegistration, error) {
	var lr domain.LicenseRegistration
	if err := r.DB.First(&lr, id).Error; err != nil {
		return nil, err
	}
	return &lr, nil
}

func (r *LicenseRegistrationRepo) FindByCode(code string) (*domain.LicenseRegistration, error) {
	var lr domain.LicenseRegistration
	if err := r.DB.Where("code = ?", code).First(&lr).Error; err != nil {
		return nil, err
	}
	return &lr, nil
}

func (r *LicenseRegistrationRepo) FindByCodeInTx(tx *gorm.DB, code string) (*domain.LicenseRegistration, error) {
	var lr domain.LicenseRegistration
	if err := tx.Where("code = ?", code).First(&lr).Error; err != nil {
		return nil, err
	}
	return &lr, nil
}

type LicenseRegistrationListParams struct {
	ProductID  uint
	LicenseeID uint
	Available  bool
	Page       int
	PageSize   int
}

func (r *LicenseRegistrationRepo) List(params LicenseRegistrationListParams) ([]domain.LicenseRegistration, int64, error) {
	if params.Page < 1 {
		params.Page = 1
	}
	if params.PageSize < 1 {
		params.PageSize = 20
	}

	query := r.DB.Model(&domain.LicenseRegistration{})
	if params.ProductID > 0 {
		query = query.Where("product_id = ?", params.ProductID)
	}
	if params.LicenseeID > 0 {
		query = query.Where("licensee_id = ?", params.LicenseeID)
	}
	if params.Available {
		query = query.Where("bound_license_id IS NULL AND (expires_at IS NULL OR expires_at > ?)", time.Now())
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []domain.LicenseRegistration
	offset := (params.Page - 1) * params.PageSize
	if err := query.Offset(offset).Limit(params.PageSize).Order("created_at DESC").Find(&items).Error; err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

func (r *LicenseRegistrationRepo) UpdateBoundLicenseInTx(tx *gorm.DB, id uint, boundLicenseID uint) error {
	return tx.Model(&domain.LicenseRegistration{}).Where("id = ?", id).Update("bound_license_id", boundLicenseID).Error
}

func (r *LicenseRegistrationRepo) UnbindLicenseInTx(tx *gorm.DB, code string) error {
	return tx.Model(&domain.LicenseRegistration{}).Where("code = ?", code).Update("bound_license_id", nil).Error
}

func (r *LicenseRegistrationRepo) DeleteExpired(now time.Time) error {
	return r.DB.Where("expires_at IS NOT NULL AND expires_at <= ? AND bound_license_id IS NULL", now).Delete(&domain.LicenseRegistration{}).Error
}

func (r *LicenseRegistrationRepo) CodeExists(code string) (bool, error) {
	var count int64
	if err := r.DB.Model(&domain.LicenseRegistration{}).Where("code = ?", code).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

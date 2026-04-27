package certificate

import (
	"metis/internal/app/license/domain"
	"time"

	"github.com/samber/do/v2"
	"gorm.io/gorm"

	"metis/internal/database"
)

type LicenseRepo struct {
	db *database.DB
}

func NewLicenseRepo(i do.Injector) (*LicenseRepo, error) {
	db := do.MustInvoke[*database.DB](i)
	return &LicenseRepo{db: db}, nil
}

func (r *LicenseRepo) CreateInTx(tx *gorm.DB, l *domain.License) error {
	return tx.Create(l).Error
}

type LicenseDetail struct {
	domain.License
	ProductName       string `gorm:"column:product_name"`
	ProductCode       string `gorm:"column:product_code"`
	ProductLicenseKey string `gorm:"column:product_license_key"`
	LicenseeName      string `gorm:"column:licensee_name"`
	LicenseeCode      string `gorm:"column:licensee_code"`
}

func (r *LicenseRepo) FindByID(id uint) (*LicenseDetail, error) {
	var detail LicenseDetail
	err := r.db.Model(&domain.License{}).
		Select("license_licenses.*, "+
			"license_products.name as product_name, "+
			"license_products.code as product_code, "+
			"license_products.license_key as product_license_key, "+
			"license_licensees.name as licensee_name, "+
			"license_licensees.code as licensee_code").
		Joins("LEFT JOIN license_products ON license_products.id = license_licenses.product_id AND license_products.deleted_at IS NULL").
		Joins("LEFT JOIN license_licensees ON license_licensees.id = license_licenses.licensee_id AND license_licensees.deleted_at IS NULL").
		Where("license_licenses.id = ?", id).
		First(&detail).Error
	if err != nil {
		return nil, err
	}
	return &detail, nil
}

type LicenseListParams struct {
	ProductID       uint
	LicenseeID      uint
	Status          string
	LifecycleStatus string
	Keyword         string
	Page            int
	PageSize        int
}

type LicenseListItem struct {
	domain.License
	ProductName  string `gorm:"column:product_name"`
	LicenseeName string `gorm:"column:licensee_name"`
}

func (r *LicenseRepo) List(params LicenseListParams) ([]LicenseListItem, int64, error) {
	if params.Page < 1 {
		params.Page = 1
	}
	if params.PageSize < 1 {
		params.PageSize = 20
	}

	base := r.db.Model(&domain.License{}).
		Joins("LEFT JOIN license_products ON license_products.id = license_licenses.product_id AND license_products.deleted_at IS NULL").
		Joins("LEFT JOIN license_licensees ON license_licensees.id = license_licenses.licensee_id AND license_licensees.deleted_at IS NULL")

	if params.ProductID > 0 {
		base = base.Where("license_licenses.product_id = ?", params.ProductID)
	}
	if params.LicenseeID > 0 {
		base = base.Where("license_licenses.licensee_id = ?", params.LicenseeID)
	}
	if params.Status != "" {
		base = base.Where("license_licenses.status = ?", params.Status)
	}
	if params.LifecycleStatus != "" {
		base = base.Where("license_licenses.lifecycle_status = ?", params.LifecycleStatus)
	}
	if params.Keyword != "" {
		like := "%" + params.Keyword + "%"
		base = base.Where("(license_licenses.plan_name LIKE ? OR license_licenses.registration_code LIKE ?)", like, like)
	}

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []LicenseListItem
	offset := (params.Page - 1) * params.PageSize
	if err := base.Select("license_licenses.*, " +
		"license_products.name as product_name, " +
		"license_licensees.name as licensee_name").
		Offset(offset).Limit(params.PageSize).
		Order("license_licenses.created_at DESC").
		Find(&items).Error; err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

func (r *LicenseRepo) UpdateStatus(id uint, updates map[string]any) error {
	return r.db.Model(&domain.License{}).Where("id = ?", id).Updates(updates).Error
}

func (r *LicenseRepo) UpdateStatusInTx(tx *gorm.DB, id uint, updates map[string]any) error {
	return tx.Model(&domain.License{}).Where("id = ?", id).Updates(updates).Error
}

func (r *LicenseRepo) UpdateExpiredStatus(now time.Time, statuses []string) error {
	return r.db.Model(&domain.License{}).
		Where("lifecycle_status IN ? AND valid_until IS NOT NULL AND valid_until <= ?", statuses, now).
		Update("lifecycle_status", domain.LicenseLifecycleExpired).Error
}

func (r *LicenseRepo) CountByProductAndKeyVersionLessThan(productID uint, version int) (int64, error) {
	var count int64
	err := r.db.Model(&domain.License{}).
		Where("product_id = ? AND status != ? AND key_version < ?", productID, domain.LicenseStatusRevoked, version).
		Count(&count).Error
	return count, err
}

func (r *LicenseRepo) FindByProductID(productID uint) ([]domain.License, error) {
	var items []domain.License
	err := r.db.Where("product_id = ?", productID).Find(&items).Error
	if err != nil {
		return nil, err
	}
	return items, nil
}

func (r *LicenseRepo) FindReissueableByProductID(productID uint, version int) ([]domain.License, error) {
	var items []domain.License
	err := r.db.Where("product_id = ? AND status != ? AND key_version < ?", productID, domain.LicenseStatusRevoked, version).Find(&items).Error
	if err != nil {
		return nil, err
	}
	return items, nil
}

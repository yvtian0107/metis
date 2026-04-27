package licensee

import (
	"github.com/samber/do/v2"
	"metis/internal/app/license/domain"

	"metis/internal/database"
)

type LicenseeRepo struct {
	DB *database.DB
}

func NewLicenseeRepo(i do.Injector) (*LicenseeRepo, error) {
	db := do.MustInvoke[*database.DB](i)
	return &LicenseeRepo{DB: db}, nil
}

func (r *LicenseeRepo) Create(l *domain.Licensee) error {
	return r.DB.Create(l).Error
}

func (r *LicenseeRepo) FindByID(id uint) (*domain.Licensee, error) {
	var l domain.Licensee
	if err := r.DB.First(&l, id).Error; err != nil {
		return nil, err
	}
	return &l, nil
}

type LicenseeListParams struct {
	Keyword  string
	Status   string
	Page     int
	PageSize int
}

func (r *LicenseeRepo) List(params LicenseeListParams) ([]domain.Licensee, int64, error) {
	if params.Page < 1 {
		params.Page = 1
	}
	if params.PageSize < 1 {
		params.PageSize = 20
	}

	query := r.DB.Model(&domain.Licensee{})
	if params.Keyword != "" {
		like := "%" + params.Keyword + "%"
		query = query.Where("name LIKE ? OR code LIKE ?", like, like)
	}
	if params.Status != "" && params.Status != "all" {
		query = query.Where("status = ?", params.Status)
	} else if params.Status == "" {
		// Default: exclude archived
		query = query.Where("status = ?", domain.LicenseeStatusActive)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []domain.Licensee
	offset := (params.Page - 1) * params.PageSize
	if err := query.Offset(offset).Limit(params.PageSize).Order("created_at DESC").Find(&items).Error; err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

func (r *LicenseeRepo) Update(l *domain.Licensee) error {
	return r.DB.Save(l).Error
}

func (r *LicenseeRepo) UpdateStatus(id uint, status string) error {
	return r.DB.Model(&domain.Licensee{}).Where("id = ?", id).Update("status", status).Error
}

func (r *LicenseeRepo) ExistsByName(name string, excludeID ...uint) (bool, error) {
	var count int64
	q := r.DB.Model(&domain.Licensee{}).Where("name = ?", name)
	if len(excludeID) > 0 && excludeID[0] > 0 {
		q = q.Where("id != ?", excludeID[0])
	}
	if err := q.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *LicenseeRepo) ExistsByCode(code string) (bool, error) {
	var count int64
	if err := r.DB.Model(&domain.Licensee{}).Where("code = ?", code).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

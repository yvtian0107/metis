package definition

import (
	"github.com/samber/do/v2"
	. "metis/internal/app/itsm/domain"

	"metis/internal/database"
)

type ServiceDefRepo struct {
	db *database.DB
}

func NewServiceDefRepo(i do.Injector) (*ServiceDefRepo, error) {
	db := do.MustInvoke[*database.DB](i)
	return &ServiceDefRepo{db: db}, nil
}

func (r *ServiceDefRepo) Create(svc *ServiceDefinition) error {
	return r.db.Create(svc).Error
}

func (r *ServiceDefRepo) FindByID(id uint) (*ServiceDefinition, error) {
	var s ServiceDefinition
	if err := r.db.First(&s, id).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *ServiceDefRepo) FindByCode(code string) (*ServiceDefinition, error) {
	var s ServiceDefinition
	if err := r.db.Where("code = ?", code).First(&s).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *ServiceDefRepo) Update(id uint, updates map[string]any) error {
	return r.db.Model(&ServiceDefinition{}).Where("id = ?", id).Updates(updates).Error
}

func (r *ServiceDefRepo) Delete(id uint) error {
	return r.db.Delete(&ServiceDefinition{}, id).Error
}

type ServiceDefListParams struct {
	CatalogID  *uint
	EngineType *string
	Keyword    string
	IsActive   *bool
	Page       int
	PageSize   int
}

func (r *ServiceDefRepo) List(params ServiceDefListParams) ([]ServiceDefinition, int64, error) {
	query := r.db.Model(&ServiceDefinition{})

	if params.CatalogID != nil {
		query = query.Where("catalog_id = ?", *params.CatalogID)
	}
	if params.EngineType != nil {
		query = query.Where("engine_type = ?", *params.EngineType)
	}
	if params.Keyword != "" {
		like := "%" + params.Keyword + "%"
		query = query.Where("name LIKE ? OR code LIKE ? OR description LIKE ?", like, like, like)
	}
	if params.IsActive != nil {
		query = query.Where("is_active = ?", *params.IsActive)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if params.Page < 1 {
		params.Page = 1
	}
	if params.PageSize < 1 {
		params.PageSize = 20
	}

	var items []ServiceDefinition
	offset := (params.Page - 1) * params.PageSize
	if err := query.Offset(offset).Limit(params.PageSize).Order("sort_order ASC, id DESC").Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

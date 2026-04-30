package catalog

import (
	"github.com/samber/do/v2"
	. "metis/internal/app/itsm/domain"

	"metis/internal/database"
)

type CatalogRepo struct {
	db *database.DB
}

func NewCatalogRepo(i do.Injector) (*CatalogRepo, error) {
	db := do.MustInvoke[*database.DB](i)
	return &CatalogRepo{db: db}, nil
}

func (r *CatalogRepo) Create(catalog *ServiceCatalog) error {
	return r.db.Create(catalog).Error
}

func (r *CatalogRepo) FindByID(id uint) (*ServiceCatalog, error) {
	var c ServiceCatalog
	if err := r.db.First(&c, id).Error; err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *CatalogRepo) FindByCode(code string) (*ServiceCatalog, error) {
	var c ServiceCatalog
	if err := r.db.Where("code = ?", code).First(&c).Error; err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *CatalogRepo) Update(id uint, updates map[string]any) error {
	return r.db.Model(&ServiceCatalog{}).Where("id = ?", id).Updates(updates).Error
}

func (r *CatalogRepo) Delete(id uint) error {
	return r.db.Delete(&ServiceCatalog{}, id).Error
}

func (r *CatalogRepo) FindAll() ([]ServiceCatalog, error) {
	var catalogs []ServiceCatalog
	if err := r.db.Order("sort_order ASC, id ASC").Find(&catalogs).Error; err != nil {
		return nil, err
	}
	return catalogs, nil
}

func (r *CatalogRepo) FindByParentID(parentID *uint) ([]ServiceCatalog, error) {
	var catalogs []ServiceCatalog
	query := r.db.Order("sort_order ASC, id ASC")
	if parentID == nil {
		query = query.Where("parent_id IS NULL")
	} else {
		query = query.Where("parent_id = ?", *parentID)
	}
	if err := query.Find(&catalogs).Error; err != nil {
		return nil, err
	}
	return catalogs, nil
}

func (r *CatalogRepo) HasChildren(id uint) (bool, error) {
	var count int64
	if err := r.db.Model(&ServiceCatalog{}).Where("parent_id = ?", id).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *CatalogRepo) HasServices(id uint) (bool, error) {
	var count int64
	if err := r.db.Model(&ServiceDefinition{}).Where("catalog_id = ?", id).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *CatalogRepo) ServiceCountsByCatalog() (map[uint]int64, int64, error) {
	type row struct {
		CatalogID uint
		Count     int64
	}
	var rows []row
	if err := r.db.Model(&ServiceDefinition{}).
		Select("catalog_id, COUNT(*) AS count").
		Group("catalog_id").
		Scan(&rows).Error; err != nil {
		return nil, 0, err
	}

	counts := make(map[uint]int64, len(rows))
	var total int64
	for _, row := range rows {
		counts[row.CatalogID] = row.Count
		total += row.Count
	}
	return counts, total, nil
}

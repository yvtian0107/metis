package product

import (
	"github.com/samber/do/v2"
	"gorm.io/gorm"
	"metis/internal/app/license/domain"

	"metis/internal/database"
)

// --- ProductRepo ---

type ProductRepo struct {
	DB *database.DB
}

func NewProductRepo(i do.Injector) (*ProductRepo, error) {
	db := do.MustInvoke[*database.DB](i)
	return &ProductRepo{DB: db}, nil
}

func (r *ProductRepo) Create(p *domain.Product) error {
	return r.DB.Create(p).Error
}

func (r *ProductRepo) FindByID(id uint) (*domain.Product, error) {
	var p domain.Product
	if err := r.DB.First(&p, id).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *ProductRepo) FindByIDWithPlans(id uint) (*domain.Product, error) {
	var p domain.Product
	if err := r.DB.Preload("Plans", func(db *gorm.DB) *gorm.DB {
		return db.Order("sort_order ASC, id ASC")
	}).First(&p, id).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *ProductRepo) FindByCode(code string) (*domain.Product, error) {
	var p domain.Product
	if err := r.DB.Where("code = ?", code).First(&p).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *ProductRepo) ExistsByCode(code string) (bool, error) {
	var count int64
	if err := r.DB.Model(&domain.Product{}).Where("code = ?", code).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

type ProductListParams struct {
	Keyword  string
	Status   string
	Page     int
	PageSize int
}

type ProductListItem struct {
	domain.Product
	PlanCount int `json:"planCount" gorm:"column:plan_count"`
}

func (r *ProductRepo) List(params ProductListParams) ([]ProductListItem, int64, error) {
	if params.Page < 1 {
		params.Page = 1
	}
	if params.PageSize < 1 {
		params.PageSize = 20
	}

	query := r.DB.Model(&domain.Product{})
	if params.Keyword != "" {
		like := "%" + params.Keyword + "%"
		query = query.Where("name LIKE ? OR code LIKE ?", like, like)
	}
	if params.Status != "" {
		query = query.Where("status = ?", params.Status)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var products []domain.Product
	offset := (params.Page - 1) * params.PageSize
	if err := query.Offset(offset).Limit(params.PageSize).Order("created_at DESC").Find(&products).Error; err != nil {
		return nil, 0, err
	}

	// Get plan counts in a single query
	productIDs := make([]uint, len(products))
	for i, p := range products {
		productIDs[i] = p.ID
	}

	planCounts := make(map[uint]int)
	if len(productIDs) > 0 {
		type countResult struct {
			ProductID uint `gorm:"column:product_id"`
			Count     int  `gorm:"column:cnt"`
		}
		var counts []countResult
		r.DB.Model(&domain.Plan{}).
			Select("product_id, COUNT(*) as cnt").
			Where("product_id IN ?", productIDs).
			Group("product_id").
			Find(&counts)
		for _, c := range counts {
			planCounts[c.ProductID] = c.Count
		}
	}

	items := make([]ProductListItem, len(products))
	for i, p := range products {
		items[i] = ProductListItem{Product: p, PlanCount: planCounts[p.ID]}
	}

	return items, total, nil
}

func (r *ProductRepo) Update(p *domain.Product) error {
	return r.DB.Save(p).Error
}

func (r *ProductRepo) UpdateStatus(id uint, status string) error {
	return r.DB.Model(&domain.Product{}).Where("id = ?", id).Update("status", status).Error
}

func (r *ProductRepo) UpdateSchema(id uint, schema []byte) error {
	return r.DB.Model(&domain.Product{}).Where("id = ?", id).Update("constraint_schema", string(schema)).Error
}

// --- PlanRepo ---

type PlanRepo struct {
	DB *database.DB
}

func NewPlanRepo(i do.Injector) (*PlanRepo, error) {
	db := do.MustInvoke[*database.DB](i)
	return &PlanRepo{DB: db}, nil
}

func (r *PlanRepo) Create(p *domain.Plan) error {
	return r.DB.Create(p).Error
}

func (r *PlanRepo) FindByID(id uint) (*domain.Plan, error) {
	var p domain.Plan
	if err := r.DB.First(&p, id).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *PlanRepo) ListByProductID(productID uint) ([]domain.Plan, error) {
	var plans []domain.Plan
	if err := r.DB.Where("product_id = ?", productID).
		Order("sort_order ASC, id ASC").
		Find(&plans).Error; err != nil {
		return nil, err
	}
	return plans, nil
}

func (r *PlanRepo) ExistsByName(productID uint, name string, excludeID ...uint) (bool, error) {
	var count int64
	q := r.DB.Model(&domain.Plan{}).Where("product_id = ? AND name = ?", productID, name)
	if len(excludeID) > 0 && excludeID[0] > 0 {
		q = q.Where("id != ?", excludeID[0])
	}
	if err := q.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *PlanRepo) Update(p *domain.Plan) error {
	return r.DB.Save(p).Error
}

func (r *PlanRepo) Delete(id uint) error {
	return r.DB.Delete(&domain.Plan{}, id).Error
}

func (r *PlanRepo) ClearDefault(productID uint) error {
	return r.DB.Model(&domain.Plan{}).
		Where("product_id = ? AND is_default = ?", productID, true).
		Update("is_default", false).Error
}

func (r *PlanRepo) SetDefault(id uint, isDefault bool) error {
	return r.DB.Model(&domain.Plan{}).Where("id = ?", id).Update("is_default", isDefault).Error
}

// --- ProductKeyRepo ---

type ProductKeyRepo struct {
	DB *database.DB
}

func NewProductKeyRepo(i do.Injector) (*ProductKeyRepo, error) {
	db := do.MustInvoke[*database.DB](i)
	return &ProductKeyRepo{DB: db}, nil
}

func (r *ProductKeyRepo) Create(k *domain.ProductKey) error {
	return r.DB.Create(k).Error
}

func (r *ProductKeyRepo) FindCurrentByProductID(productID uint) (*domain.ProductKey, error) {
	var k domain.ProductKey
	if err := r.DB.Where("product_id = ? AND is_current = ?", productID, true).First(&k).Error; err != nil {
		return nil, err
	}
	return &k, nil
}

func (r *ProductKeyRepo) RevokeByProductID(tx *gorm.DB, productID uint) error {
	now := tx.NowFunc()
	return tx.Model(&domain.ProductKey{}).
		Where("product_id = ? AND is_current = ?", productID, true).
		Updates(map[string]any{"is_current": false, "revoked_at": now}).Error
}

func (r *ProductKeyRepo) CreateInTx(tx *gorm.DB, k *domain.ProductKey) error {
	return tx.Create(k).Error
}

func (r *ProductKeyRepo) FindByProductIDAndVersion(productID uint, version int) (*domain.ProductKey, error) {
	var k domain.ProductKey
	if err := r.DB.Where("product_id = ? AND version = ?", productID, version).First(&k).Error; err != nil {
		return nil, err
	}
	return &k, nil
}

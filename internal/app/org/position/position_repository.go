package position

import (
	"github.com/samber/do/v2"
	"metis/internal/app/org/domain"

	"metis/internal/database"
)

type PositionRepo struct {
	db *database.DB
}

func NewPositionRepo(i do.Injector) (*PositionRepo, error) {
	db := do.MustInvoke[*database.DB](i)
	return &PositionRepo{db: db}, nil
}

func (r *PositionRepo) Create(pos *domain.Position) error {
	return r.db.Create(pos).Error
}

func (r *PositionRepo) FindByID(id uint) (*domain.Position, error) {
	var pos domain.Position
	if err := r.db.First(&pos, id).Error; err != nil {
		return nil, err
	}
	return &pos, nil
}

func (r *PositionRepo) FindByCode(code string) (*domain.Position, error) {
	var pos domain.Position
	if err := r.db.Where("code = ?", code).First(&pos).Error; err != nil {
		return nil, err
	}
	return &pos, nil
}

func (r *PositionRepo) Update(id uint, updates map[string]any) error {
	return r.db.Model(&domain.Position{}).Where("id = ?", id).Updates(updates).Error
}

func (r *PositionRepo) Delete(id uint) error {
	return r.db.Delete(&domain.Position{}, id).Error
}

type PositionListParams struct {
	Keyword  string
	Page     int
	PageSize int
}

type PositionUsage struct {
	DepartmentCount int
	MemberCount     int
	Departments     []domain.PositionDepartmentSummary
}

func (r *PositionRepo) List(params PositionListParams) ([]domain.Position, int64, error) {
	if params.Page < 1 {
		params.Page = 1
	}

	base := r.db.Model(&domain.Position{})
	if params.Keyword != "" {
		like := "%" + params.Keyword + "%"
		base = base.Where("name LIKE ? OR code LIKE ?", like, like)
	}

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []domain.Position
	query := base.Order("sort ASC, id ASC")
	if params.PageSize > 0 {
		offset := (params.Page - 1) * params.PageSize
		query = query.Offset(offset).Limit(params.PageSize)
	}
	// pageSize=0 means return all (no pagination)
	if err := query.Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *PositionRepo) UsageByPositionIDs(positionIDs []uint) (map[uint]PositionUsage, error) {
	usage := make(map[uint]PositionUsage, len(positionIDs))
	if len(positionIDs) == 0 {
		return usage, nil
	}

	var deptRows []struct {
		PositionID   uint
		DepartmentID uint
		Name         string
		Code         string
	}
	if err := r.db.Model(&domain.DepartmentPosition{}).
		Select("department_positions.position_id, departments.id AS department_id, departments.name, departments.code").
		Joins("JOIN departments ON departments.id = department_positions.department_id AND departments.deleted_at IS NULL").
		Where("department_positions.position_id IN ?", positionIDs).
		Order("departments.sort ASC, departments.id ASC").
		Scan(&deptRows).Error; err != nil {
		return nil, err
	}
	for _, row := range deptRows {
		item := usage[row.PositionID]
		item.DepartmentCount++
		item.Departments = append(item.Departments, domain.PositionDepartmentSummary{
			ID:   row.DepartmentID,
			Name: row.Name,
			Code: row.Code,
		})
		usage[row.PositionID] = item
	}

	var memberRows []struct {
		PositionID uint
		Count      int
	}
	if err := r.db.Model(&domain.UserPosition{}).
		Select("position_id, COUNT(DISTINCT user_id) AS count").
		Where("position_id IN ?", positionIDs).
		Group("position_id").
		Scan(&memberRows).Error; err != nil {
		return nil, err
	}
	for _, row := range memberRows {
		item := usage[row.PositionID]
		item.MemberCount = row.Count
		usage[row.PositionID] = item
	}

	return usage, nil
}

func (r *PositionRepo) ListActive() ([]domain.Position, error) {
	var items []domain.Position
	if err := r.db.Where("is_active = ?", true).Order("sort ASC, id ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *PositionRepo) InUse(id uint) (bool, error) {
	var count int64
	if err := r.db.Model(&domain.UserPosition{}).Where("position_id = ?", id).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

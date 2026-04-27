package runtime

import (
	"github.com/samber/do/v2"
	"gorm.io/gorm"

	"metis/internal/database"
)

type SkillRepo struct {
	db *database.DB
}

func NewSkillRepo(i do.Injector) (*SkillRepo, error) {
	return &SkillRepo{db: do.MustInvoke[*database.DB](i)}, nil
}

type SkillListParams struct {
	Keyword  string
	Page     int
	PageSize int
}

func (r *SkillRepo) List(params SkillListParams) ([]Skill, int64, error) {
	if params.Page < 1 {
		params.Page = 1
	}
	if params.PageSize < 1 {
		params.PageSize = 20
	}

	query := r.db.Model(&Skill{})
	if params.Keyword != "" {
		like := "%" + params.Keyword + "%"
		query = query.Where("name LIKE ? OR display_name LIKE ?", like, like)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var skills []Skill
	offset := (params.Page - 1) * params.PageSize
	if err := query.Offset(offset).Limit(params.PageSize).Order("created_at DESC").Find(&skills).Error; err != nil {
		return nil, 0, err
	}
	return skills, total, nil
}

func (r *SkillRepo) FindByID(id uint) (*Skill, error) {
	var s Skill
	if err := r.db.First(&s, id).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *SkillRepo) Create(s *Skill) error {
	return r.db.Create(s).Error
}

func (r *SkillRepo) Update(s *Skill) error {
	return r.db.Save(s).Error
}

func (r *SkillRepo) Delete(id uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("skill_id = ?", id).Delete(&AgentSkill{}).Error; err != nil {
			return err
		}
		return tx.Delete(&Skill{}, id).Error
	})
}

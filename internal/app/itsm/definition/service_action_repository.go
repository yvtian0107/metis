package definition

import (
	"github.com/samber/do/v2"
	. "metis/internal/app/itsm/domain"

	"metis/internal/database"
)

type ServiceActionRepo struct {
	db *database.DB
}

func NewServiceActionRepo(i do.Injector) (*ServiceActionRepo, error) {
	db := do.MustInvoke[*database.DB](i)
	return &ServiceActionRepo{db: db}, nil
}

func (r *ServiceActionRepo) Create(action *ServiceAction) error {
	return r.db.Create(action).Error
}

func (r *ServiceActionRepo) FindByID(id uint) (*ServiceAction, error) {
	var a ServiceAction
	if err := r.db.First(&a, id).Error; err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *ServiceActionRepo) FindByServiceAndCode(serviceID uint, code string) (*ServiceAction, error) {
	var a ServiceAction
	if err := r.db.Where("service_id = ? AND code = ?", serviceID, code).First(&a).Error; err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *ServiceActionRepo) Update(id uint, updates map[string]any) error {
	return r.db.Model(&ServiceAction{}).Where("id = ?", id).Updates(updates).Error
}

func (r *ServiceActionRepo) Delete(id uint) error {
	return r.db.Delete(&ServiceAction{}, id).Error
}

func (r *ServiceActionRepo) ListByService(serviceID uint) ([]ServiceAction, error) {
	var actions []ServiceAction
	if err := r.db.Where("service_id = ?", serviceID).Order("id ASC").Find(&actions).Error; err != nil {
		return nil, err
	}
	return actions, nil
}

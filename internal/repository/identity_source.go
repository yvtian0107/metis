package repository

import (
	"errors"
	"strings"

	"github.com/samber/do/v2"
	"gorm.io/gorm"

	"metis/internal/database"
	"metis/internal/model"
)

var ErrDomainConflict = errors.New("error.identity.domain_conflict")

type IdentitySourceRepo struct {
	db *database.DB
}

func NewIdentitySource(i do.Injector) (*IdentitySourceRepo, error) {
	db := do.MustInvoke[*database.DB](i)
	return &IdentitySourceRepo{db: db}, nil
}

func (r *IdentitySourceRepo) List() ([]model.IdentitySource, error) {
	var sources []model.IdentitySource
	if err := r.db.Order("sort_order ASC, id ASC").Find(&sources).Error; err != nil {
		return nil, err
	}
	return sources, nil
}

func (r *IdentitySourceRepo) FindByID(id uint) (*model.IdentitySource, error) {
	var source model.IdentitySource
	if err := r.db.First(&source, id).Error; err != nil {
		return nil, err
	}
	return &source, nil
}

// FindByDomain returns the first enabled identity source that matches the given email domain.
func (r *IdentitySourceRepo) FindByDomain(domain string) (*model.IdentitySource, error) {
	domain = strings.ToLower(strings.TrimSpace(domain))
	var sources []model.IdentitySource
	if err := r.db.Where("enabled = ?", true).Order("sort_order ASC").Find(&sources).Error; err != nil {
		return nil, err
	}
	for _, s := range sources {
		for _, d := range strings.Split(s.Domains, ",") {
			if strings.ToLower(strings.TrimSpace(d)) == domain {
				return &s, nil
			}
		}
	}
	return nil, gorm.ErrRecordNotFound
}

// CheckDomainConflict verifies that none of the given domains are bound to another source.
func (r *IdentitySourceRepo) CheckDomainConflict(domains string, excludeID uint) error {
	if domains == "" {
		return nil
	}
	newDomains := parseDomains(domains)
	if len(newDomains) == 0 {
		return nil
	}

	var sources []model.IdentitySource
	query := r.db.Model(&model.IdentitySource{})
	if excludeID > 0 {
		query = query.Where("id != ?", excludeID)
	}
	if err := query.Find(&sources).Error; err != nil {
		return err
	}

	for _, s := range sources {
		existing := parseDomains(s.Domains)
		for _, nd := range newDomains {
			for _, ed := range existing {
				if nd == ed {
					return ErrDomainConflict
				}
			}
		}
	}
	return nil
}

func (r *IdentitySourceRepo) Create(source *model.IdentitySource) error {
	return r.db.Create(source).Error
}

func (r *IdentitySourceRepo) Update(source *model.IdentitySource) error {
	return r.db.Save(source).Error
}

func (r *IdentitySourceRepo) Delete(id uint) error {
	return r.db.Delete(&model.IdentitySource{}, id).Error
}

func (r *IdentitySourceRepo) Toggle(id uint) (*model.IdentitySource, error) {
	source, err := r.FindByID(id)
	if err != nil {
		return nil, err
	}
	source.Enabled = !source.Enabled
	if err := r.db.Save(source).Error; err != nil {
		return nil, err
	}
	return source, nil
}

func parseDomains(s string) []string {
	var result []string
	for _, d := range strings.Split(s, ",") {
		d = strings.ToLower(strings.TrimSpace(d))
		if d != "" {
			result = append(result, d)
		}
	}
	return result
}

package definition

import (
	"github.com/samber/do/v2"
	"gorm.io/gorm"
	. "metis/internal/app/itsm/domain"

	"metis/internal/database"
)

type KnowledgeDocRepo struct {
	db *gorm.DB
}

func NewKnowledgeDocRepo(i do.Injector) (*KnowledgeDocRepo, error) {
	db := do.MustInvoke[*database.DB](i)
	return &KnowledgeDocRepo{db: db.DB}, nil
}

func (r *KnowledgeDocRepo) Create(doc *ServiceKnowledgeDocument) error {
	return r.db.Create(doc).Error
}

func (r *KnowledgeDocRepo) ListByServiceID(serviceID uint) ([]ServiceKnowledgeDocument, error) {
	var docs []ServiceKnowledgeDocument
	err := r.db.Where("service_id = ?", serviceID).Order("created_at DESC").Find(&docs).Error
	return docs, err
}

func (r *KnowledgeDocRepo) GetByID(id uint) (*ServiceKnowledgeDocument, error) {
	var doc ServiceKnowledgeDocument
	err := r.db.First(&doc, id).Error
	return &doc, err
}

func (r *KnowledgeDocRepo) GetByServiceAndID(serviceID, id uint) (*ServiceKnowledgeDocument, error) {
	var doc ServiceKnowledgeDocument
	err := r.db.Where("service_id = ? AND id = ?", serviceID, id).First(&doc).Error
	return &doc, err
}

func (r *KnowledgeDocRepo) Delete(id uint) error {
	return r.db.Delete(&ServiceKnowledgeDocument{}, id).Error
}

func (r *KnowledgeDocRepo) DeleteByService(serviceID, id uint) error {
	return r.db.Where("service_id = ? AND id = ?", serviceID, id).Delete(&ServiceKnowledgeDocument{}).Error
}

func (r *KnowledgeDocRepo) UpdateParseResult(id uint, status, parsedText, parseError string) error {
	updates := map[string]any{
		"parse_status": status,
		"parsed_text":  parsedText,
		"parse_error":  parseError,
	}
	return r.db.Model(&ServiceKnowledgeDocument{}).Where("id = ?", id).Updates(updates).Error
}

// ListCompletedByServiceID returns all documents with parse_status=completed for a given service.
func (r *KnowledgeDocRepo) ListCompletedByServiceID(serviceID uint) ([]ServiceKnowledgeDocument, error) {
	var docs []ServiceKnowledgeDocument
	err := r.db.Where("service_id = ? AND parse_status = ?", serviceID, "completed").Find(&docs).Error
	return docs, err
}

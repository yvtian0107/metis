package runtime

// Legacy KnowledgeBaseRepo — wraps the old ai_knowledge_bases table.
// Kept temporarily so that compile/embedding services continue to work
// during the migration period. Will be removed when Phase 4 converts
// these services to use KnowledgeAsset + KnowledgeEngine.

import (
	"github.com/samber/do/v2"

	"metis/internal/database"
)

type KnowledgeBaseRepo struct {
	db *database.DB
}

func NewKnowledgeBaseRepo(i do.Injector) (*KnowledgeBaseRepo, error) {
	db := do.MustInvoke[*database.DB](i)
	return &KnowledgeBaseRepo{db: db}, nil
}

func (r *KnowledgeBaseRepo) Create(kb *KnowledgeBase) error {
	return r.db.Create(kb).Error
}

func (r *KnowledgeBaseRepo) FindByID(id uint) (*KnowledgeBase, error) {
	var kb KnowledgeBase
	if err := r.db.First(&kb, id).Error; err != nil {
		return nil, err
	}
	return &kb, nil
}

func (r *KnowledgeBaseRepo) List() ([]KnowledgeBase, error) {
	var kbs []KnowledgeBase
	err := r.db.Order("id DESC").Find(&kbs).Error
	return kbs, err
}

func (r *KnowledgeBaseRepo) Update(kb *KnowledgeBase) error {
	return r.db.Save(kb).Error
}

func (r *KnowledgeBaseRepo) Delete(id uint) error {
	return r.db.Delete(&KnowledgeBase{}, id).Error
}

// UpdateSourceCount updates source_count from the old kb_id column in ai_knowledge_sources.
func (r *KnowledgeBaseRepo) UpdateSourceCount(id uint) error {
	return r.db.Exec(`
		UPDATE ai_knowledge_bases SET
			source_count = (SELECT COUNT(*) FROM ai_knowledge_sources WHERE kb_id = ?)
		WHERE id = ?
	`, id, id).Error
}

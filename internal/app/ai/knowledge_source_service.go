package ai

import (
	"errors"
	"log/slog"

	"github.com/samber/do/v2"
	"gorm.io/gorm"
)

var (
	ErrSourceNotFound = errors.New("knowledge source not found")
)

type knowledgeGraphNodeDeleter interface {
	DeleteNodesBySourceID(kbID uint, sourceID uint) (int64, error)
}

type KnowledgeSourceService struct {
	repo      *KnowledgeSourceRepo
	kbRepo    *KnowledgeBaseRepo
	graphRepo knowledgeGraphNodeDeleter
}

func NewKnowledgeSourceService(i do.Injector) (*KnowledgeSourceService, error) {
	return &KnowledgeSourceService{
		repo:      do.MustInvoke[*KnowledgeSourceRepo](i),
		kbRepo:    do.MustInvoke[*KnowledgeBaseRepo](i),
		graphRepo: do.MustInvoke[*KnowledgeGraphRepo](i),
	}, nil
}

func (s *KnowledgeSourceService) Create(src *KnowledgeSource) error {
	if err := s.repo.Create(src); err != nil {
		return err
	}
	if err := s.kbRepo.UpdateSourceCount(src.KbID); err != nil {
		slog.Error("failed to update kb counts after source create", "kb_id", src.KbID, "error", err)
	}
	return nil
}

func (s *KnowledgeSourceService) Get(id uint) (*KnowledgeSource, error) {
	src, err := s.repo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSourceNotFound
		}
		return nil, err
	}
	return src, nil
}

func (s *KnowledgeSourceService) Delete(id uint) error {
	src, err := s.repo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrSourceNotFound
		}
		return err
	}

	// Collect all source IDs to clean up from graph (self + children)
	sourceIDs := []uint{id}
	childIDs, _ := s.repo.FindChildIDs(id)
	sourceIDs = append(sourceIDs, childIDs...)

	// Delete child sources (URL with depth > 0)
	s.repo.DeleteByParentID(id)
	if err := s.repo.Delete(id); err != nil {
		return err
	}

	// Clean up FalkorDB graph nodes referencing deleted sources
	for _, sid := range sourceIDs {
		deleted, err := s.graphRepo.DeleteNodesBySourceID(src.KbID, sid)
		if err != nil {
			slog.Error("failed to clean graph nodes for deleted source", "source_id", sid, "kb_id", src.KbID, "error", err)
		} else if deleted > 0 {
			slog.Info("cleaned graph nodes for deleted source", "source_id", sid, "kb_id", src.KbID, "deleted_nodes", deleted)
		}
	}

	if err := s.kbRepo.UpdateSourceCount(src.KbID); err != nil {
		slog.Error("failed to update kb counts after source delete", "kb_id", src.KbID, "error", err)
	}
	return nil
}

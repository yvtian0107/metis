package ai

import (
	"errors"
	"fmt"

	"github.com/samber/do/v2"
	"gorm.io/gorm"
)

var (
	ErrKnowledgeBaseNotFound = errors.New("knowledge base not found")
)

type knowledgeGraphDeleter interface {
	DeleteGraph(kbID uint) error
}

type KnowledgeBaseService struct {
	repo       *KnowledgeBaseRepo
	sourceRepo *KnowledgeSourceRepo
	graphRepo  knowledgeGraphDeleter
}

func NewKnowledgeBaseService(i do.Injector) (*KnowledgeBaseService, error) {
	return &KnowledgeBaseService{
		repo:       do.MustInvoke[*KnowledgeBaseRepo](i),
		sourceRepo: do.MustInvoke[*KnowledgeSourceRepo](i),
		graphRepo:  do.MustInvoke[*KnowledgeGraphRepo](i),
	}, nil
}

func (s *KnowledgeBaseService) Create(kb *KnowledgeBase) error {
	kb.CompileStatus = CompileStatusIdle
	return s.repo.Create(kb)
}

func (s *KnowledgeBaseService) Get(id uint) (*KnowledgeBase, error) {
	kb, err := s.repo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrKnowledgeBaseNotFound
		}
		return nil, err
	}
	return kb, nil
}

func (s *KnowledgeBaseService) Update(kb *KnowledgeBase) error {
	return s.repo.Update(kb)
}

func (s *KnowledgeBaseService) Delete(id uint) error {
	// Delete FalkorDB graph (contains all nodes and edges)
	if err := s.graphRepo.DeleteGraph(id); err != nil {
		return fmt.Errorf("delete graph: %w", err)
	}
	// Delete GORM sources
	if err := s.sourceRepo.DeleteByKbID(id); err != nil {
		return fmt.Errorf("delete sources: %w", err)
	}
	return s.repo.Delete(id)
}

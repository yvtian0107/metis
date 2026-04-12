package ai

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/samber/do/v2"

	"metis/internal/llm"
	"metis/internal/pkg/crypto"
)

// KnowledgeEmbeddingService generates vector embeddings for knowledge nodes.
type KnowledgeEmbeddingService struct {
	graphRepo *KnowledgeGraphRepo
	kbRepo    *KnowledgeBaseRepo
	modelRepo *ModelRepo
	logRepo   *KnowledgeLogRepo
	encKey    crypto.EncryptionKey
}

func NewKnowledgeEmbeddingService(i do.Injector) (*KnowledgeEmbeddingService, error) {
	return &KnowledgeEmbeddingService{
		graphRepo: do.MustInvoke[*KnowledgeGraphRepo](i),
		kbRepo:    do.MustInvoke[*KnowledgeBaseRepo](i),
		modelRepo: do.MustInvoke[*ModelRepo](i),
		logRepo:   do.MustInvoke[*KnowledgeLogRepo](i),
		encKey:    do.MustInvoke[crypto.EncryptionKey](i),
	}, nil
}

// GenerateEmbeddings generates vector embeddings for all non-index nodes in a KB,
// writes them to FalkorDB, and rebuilds the HNSW vector index.
func (s *KnowledgeEmbeddingService) GenerateEmbeddings(ctx context.Context, kbID uint) error {
	kb, err := s.kbRepo.FindByID(kbID)
	if err != nil {
		return fmt.Errorf("find kb %d: %w", kbID, err)
	}

	// Check if embedding is configured
	if kb.EmbeddingProviderID == nil || kb.EmbeddingModelID == "" {
		slog.Info("embedding not configured, skipping", "kb_id", kbID)
		return nil
	}

	// Resolve the embedding client
	client, err := s.resolveEmbeddingClient(kb)
	if err != nil {
		slog.Error("failed to resolve embedding client", "kb_id", kbID, "error", err)
		return fmt.Errorf("resolve embedding client: %w", err)
	}

	// Get all non-index nodes
	nodes, err := s.graphRepo.FindAllNodes(kbID)
	if err != nil {
		return fmt.Errorf("find nodes: %w", err)
	}

	// Build input texts
	var inputs []string
	var nodeIDs []string
	for _, n := range nodes {
		inputs = append(inputs, n.Title+"\n"+n.Summary)
		nodeIDs = append(nodeIDs, n.ID)
	}

	if len(inputs) == 0 {
		slog.Info("no nodes to embed", "kb_id", kbID)
		return nil
	}

	// Drop vector index before writing embeddings (avoid HNSW concurrent write bug)
	if err := s.graphRepo.DropVectorIndex(kbID); err != nil {
		slog.Warn("failed to drop vector index", "kb_id", kbID, "error", err)
	}

	// Generate embeddings in batches
	batchSize := 100
	var dimension int
	for i := 0; i < len(inputs); i += batchSize {
		end := i + batchSize
		if end > len(inputs) {
			end = len(inputs)
		}

		resp, err := client.Embedding(ctx, llm.EmbeddingRequest{
			Model: kb.EmbeddingModelID,
			Input: inputs[i:end],
		})
		if err != nil {
			slog.Error("embedding API error", "kb_id", kbID, "batch", i/batchSize, "error", err)
			s.logRepo.Create(&KnowledgeLog{
				KbID:         kbID,
				Action:       "embedding_error",
				ErrorMessage: fmt.Sprintf("batch %d: %s", i/batchSize, err.Error()),
			})
			continue
		}

		// Write embeddings to FalkorDB
		for j, emb := range resp.Embeddings {
			if dimension == 0 {
				dimension = len(emb)
			}
			nodeIdx := i + j
			if nodeIdx >= len(nodeIDs) {
				break
			}
			if err := s.graphRepo.SetNodeEmbedding(kbID, nodeIDs[nodeIdx], emb); err != nil {
				slog.Error("failed to set embedding", "kb_id", kbID, "node_id", nodeIDs[nodeIdx], "error", err)
			}
		}
	}

	// Rebuild HNSW vector index
	if dimension > 0 {
		if err := s.graphRepo.CreateVectorIndex(kbID, dimension); err != nil {
			slog.Error("failed to create vector index", "kb_id", kbID, "error", err)
			return fmt.Errorf("create vector index: %w", err)
		}
		slog.Info("vector index rebuilt", "kb_id", kbID, "dimension", dimension, "nodes", len(nodeIDs))
	}

	return nil
}

// resolveEmbeddingClient builds an llm.Client for the configured embedding provider.
func (s *KnowledgeEmbeddingService) resolveEmbeddingClient(kb *KnowledgeBase) (llm.Client, error) {
	if kb.EmbeddingProviderID == nil {
		return nil, fmt.Errorf("no embedding provider configured")
	}

	// Find the provider
	provider, err := func() (*Provider, error) {
		var p Provider
		if err := s.modelRepo.db.First(&p, *kb.EmbeddingProviderID).Error; err != nil {
			return nil, err
		}
		return &p, nil
	}()
	if err != nil {
		return nil, fmt.Errorf("find embedding provider: %w", err)
	}

	apiKey, err := decryptAPIKey(provider.APIKeyEncrypted, s.encKey)
	if err != nil {
		return nil, fmt.Errorf("decrypt api key: %w", err)
	}

	client, err := llm.NewClient(provider.Protocol, provider.BaseURL, apiKey)
	if err != nil {
		return nil, fmt.Errorf("create embedding client: %w", err)
	}

	return client, nil
}

package runtime

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/samber/do/v2"

	"metis/internal/llm"
	"metis/internal/pkg/crypto"
)

// KnowledgeEmbeddingService generates vector embeddings for both
// graph nodes (FalkorDB HNSW) and RAG chunks (pgvector).
type KnowledgeEmbeddingService struct {
	graphRepo *KnowledgeGraphRepo
	assetRepo *KnowledgeAssetRepo
	chunkRepo *RAGChunkRepo
	modelRepo *ModelRepo
	logRepo   *KnowledgeLogRepo
	encKey    crypto.EncryptionKey
}

func NewKnowledgeEmbeddingService(i do.Injector) (*KnowledgeEmbeddingService, error) {
	return &KnowledgeEmbeddingService{
		graphRepo: do.MustInvoke[*KnowledgeGraphRepo](i),
		assetRepo: do.MustInvoke[*KnowledgeAssetRepo](i),
		chunkRepo: do.MustInvoke[*RAGChunkRepo](i),
		modelRepo: do.MustInvoke[*ModelRepo](i),
		logRepo:   do.MustInvoke[*KnowledgeLogRepo](i),
		encKey:    do.MustInvoke[crypto.EncryptionKey](i),
	}, nil
}

// ---------------------------------------------------------------------------
// Public API
// ---------------------------------------------------------------------------

// GenerateGraphEmbeddings generates vector embeddings for all graph nodes of
// the given asset, writes them to FalkorDB, and rebuilds the HNSW vector index.
// This replaces the old GenerateEmbeddings that operated on KnowledgeBase.
func (s *KnowledgeEmbeddingService) GenerateGraphEmbeddings(ctx context.Context, assetID uint) error {
	asset, err := s.assetRepo.FindByID(assetID)
	if err != nil {
		return fmt.Errorf("find asset %d: %w", assetID, err)
	}
	return s.generateGraphEmbeddingsForAsset(ctx, asset)
}

// GenerateRAGEmbeddings generates vector embeddings for all RAG chunks of
// the given asset that don't yet have embeddings, and writes them to the
// ai_rag_chunks table (pgvector column).
func (s *KnowledgeEmbeddingService) GenerateRAGEmbeddings(ctx context.Context, assetID uint) error {
	asset, err := s.assetRepo.FindByID(assetID)
	if err != nil {
		return fmt.Errorf("find asset %d: %w", assetID, err)
	}
	return s.generateRAGEmbeddingsForAsset(ctx, asset)
}

// GenerateEmbeddings is the legacy entry point used by KnowledgeCompileService.
// It routes to GenerateGraphEmbeddings. Once compile service is fully migrated
// to use asset IDs, this can be removed.
func (s *KnowledgeEmbeddingService) GenerateEmbeddings(ctx context.Context, kbID uint) error {
	return s.GenerateGraphEmbeddings(ctx, kbID)
}

// EmbedQuery generates a single embedding vector for a query string using the
// given asset's embedding configuration. Shared by both graph and RAG search.
func (s *KnowledgeEmbeddingService) EmbedQuery(ctx context.Context, asset *KnowledgeAsset, query string) ([]float32, error) {
	client, err := s.resolveEmbeddingClientForAsset(asset)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	resp, err := client.Embedding(ctx, llm.EmbeddingRequest{
		Model: asset.EmbeddingModelID,
		Input: []string{query},
	})
	if err != nil {
		return nil, fmt.Errorf("embedding API: %w", err)
	}
	if len(resp.Embeddings) == 0 {
		return nil, errEmbeddingEmpty
	}
	return resp.Embeddings[0], nil
}

// ---------------------------------------------------------------------------
// Graph embeddings (FalkorDB HNSW)
// ---------------------------------------------------------------------------

func (s *KnowledgeEmbeddingService) generateGraphEmbeddingsForAsset(ctx context.Context, asset *KnowledgeAsset) error {
	if asset.EmbeddingProviderID == nil || asset.EmbeddingModelID == "" {
		slog.Info("embedding not configured, skipping", "asset_id", asset.ID)
		return nil
	}

	client, err := s.resolveEmbeddingClientForAsset(asset)
	if err != nil {
		slog.Error("failed to resolve embedding client", "asset_id", asset.ID, "error", err)
		return fmt.Errorf("resolve embedding client: %w", err)
	}

	nodes, err := s.graphRepo.FindAllNodes(asset.ID)
	if err != nil {
		return fmt.Errorf("find nodes: %w", err)
	}

	var inputs []string
	var nodeIDs []string
	for _, n := range nodes {
		inputs = append(inputs, n.Title+"\n"+n.Summary)
		nodeIDs = append(nodeIDs, n.ID)
	}

	if len(inputs) == 0 {
		slog.Info("no nodes to embed", "asset_id", asset.ID)
		return nil
	}

	// Drop vector index before writing embeddings (avoid HNSW concurrent write bug)
	if err := s.graphRepo.DropVectorIndex(asset.ID); err != nil {
		slog.Warn("failed to drop vector index", "asset_id", asset.ID, "error", err)
	}

	batchSize := 100
	var dimension int
	for i := 0; i < len(inputs); i += batchSize {
		end := i + batchSize
		if end > len(inputs) {
			end = len(inputs)
		}

		resp, err := client.Embedding(ctx, llm.EmbeddingRequest{
			Model: asset.EmbeddingModelID,
			Input: inputs[i:end],
		})
		if err != nil {
			slog.Error("embedding API error", "asset_id", asset.ID, "batch", i/batchSize, "error", err)
			_ = s.logRepo.Create(&KnowledgeLog{
				AssetID:      asset.ID,
				Action:       "embedding_error",
				ErrorMessage: fmt.Sprintf("batch %d: %s", i/batchSize, err.Error()),
			})
			continue
		}

		for j, emb := range resp.Embeddings {
			if dimension == 0 {
				dimension = len(emb)
			}
			nodeIdx := i + j
			if nodeIdx >= len(nodeIDs) {
				break
			}
			if err := s.graphRepo.SetNodeEmbedding(asset.ID, nodeIDs[nodeIdx], emb); err != nil {
				slog.Error("failed to set embedding", "asset_id", asset.ID, "node_id", nodeIDs[nodeIdx], "error", err)
			}
		}
	}

	if dimension > 0 {
		if err := s.graphRepo.CreateVectorIndex(asset.ID, dimension); err != nil {
			slog.Error("failed to create vector index", "asset_id", asset.ID, "error", err)
			return fmt.Errorf("create vector index: %w", err)
		}
		slog.Info("graph vector index rebuilt", "asset_id", asset.ID, "dimension", dimension, "nodes", len(nodeIDs))
	}

	return nil
}

// ---------------------------------------------------------------------------
// RAG embeddings (pgvector)
// ---------------------------------------------------------------------------

func (s *KnowledgeEmbeddingService) generateRAGEmbeddingsForAsset(ctx context.Context, asset *KnowledgeAsset) error {
	if asset.EmbeddingProviderID == nil || asset.EmbeddingModelID == "" {
		slog.Info("embedding not configured, skipping RAG embeddings", "asset_id", asset.ID)
		return nil
	}

	client, err := s.resolveEmbeddingClientForAsset(asset)
	if err != nil {
		return fmt.Errorf("resolve embedding client: %w", err)
	}

	// Fetch chunks without embeddings
	chunks, err := s.chunkRepo.FindWithoutEmbedding(asset.ID)
	if err != nil {
		return fmt.Errorf("find chunks without embedding: %w", err)
	}

	if len(chunks) == 0 {
		slog.Info("all chunks already have embeddings", "asset_id", asset.ID)
		return nil
	}

	slog.Info("generating RAG embeddings", "asset_id", asset.ID, "chunks", len(chunks))

	batchSize := 100
	var dimension int
	embedded := 0
	for i := 0; i < len(chunks); i += batchSize {
		end := i + batchSize
		if end > len(chunks) {
			end = len(chunks)
		}

		batch := chunks[i:end]
		inputs := make([]string, len(batch))
		for j, c := range batch {
			inputs[j] = c.Content
		}

		resp, err := client.Embedding(ctx, llm.EmbeddingRequest{
			Model: asset.EmbeddingModelID,
			Input: inputs,
		})
		if err != nil {
			slog.Error("embedding API error", "asset_id", asset.ID, "batch", i/batchSize, "error", err)
			_ = s.logRepo.Create(&KnowledgeLog{
				AssetID:      asset.ID,
				Action:       "embedding_error",
				ErrorMessage: fmt.Sprintf("RAG batch %d: %s", i/batchSize, err.Error()),
			})
			continue
		}

		for j, emb := range resp.Embeddings {
			if j >= len(batch) {
				break
			}
			if dimension == 0 {
				dimension = len(emb)
			}
			if err := s.chunkRepo.UpdateEmbedding(batch[j].ID, emb); err != nil {
				slog.Error("failed to update chunk embedding", "chunk_id", batch[j].ID, "error", err)
				continue
			}
			embedded++
		}
	}

	// Ensure pgvector index exists
	if dimension > 0 {
		if err := s.chunkRepo.EnsureVectorIndex(dimension); err != nil {
			slog.Warn("failed to ensure vector index", "error", err)
		}
	}

	slog.Info("RAG embeddings complete", "asset_id", asset.ID, "embedded", embedded, "dimension", dimension)
	return nil
}

// ---------------------------------------------------------------------------
// Shared: resolve embedding LLM client from asset config
// ---------------------------------------------------------------------------

func (s *KnowledgeEmbeddingService) resolveEmbeddingClientForAsset(asset *KnowledgeAsset) (llm.Client, error) {
	if asset.EmbeddingProviderID == nil {
		return nil, errEmbeddingNotConfigured
	}

	var provider Provider
	if err := s.modelRepo.db.First(&provider, *asset.EmbeddingProviderID).Error; err != nil {
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

package runtime

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/samber/do/v2"

	"metis/internal/scheduler"
)

// NaiveChunkEngine implements KnowledgeEngine for the "naive_chunk" RAG type.
// It splits source documents into fixed-size chunks, embeds them via the
// embedding service, and stores them in ai_rag_chunks for vector search.
type NaiveChunkEngine struct {
	chunkRepo    *RAGChunkRepo
	assetRepo    *KnowledgeAssetRepo
	sourceRepo   *KnowledgeSourceRepo
	logRepo      *KnowledgeLogRepo
	embeddingSvc *KnowledgeEmbeddingService
	engine       *scheduler.Engine
}

func NewNaiveChunkEngine(i do.Injector) (*NaiveChunkEngine, error) {
	e := &NaiveChunkEngine{
		chunkRepo:    do.MustInvoke[*RAGChunkRepo](i),
		assetRepo:    do.MustInvoke[*KnowledgeAssetRepo](i),
		sourceRepo:   do.MustInvoke[*KnowledgeSourceRepo](i),
		logRepo:      do.MustInvoke[*KnowledgeLogRepo](i),
		embeddingSvc: do.MustInvoke[*KnowledgeEmbeddingService](i),
		engine:       do.MustInvoke[*scheduler.Engine](i),
	}
	RegisterEngine(AssetCategoryKB, KBTypeNaiveChunk, e)
	return e, nil
}

// Build performs incremental chunking: only processes sources that don't yet
// have chunks in the store. After chunking, generates embeddings for all
// chunks that are missing vectors.
func (e *NaiveChunkEngine) Build(ctx context.Context, asset *KnowledgeAsset, sources []*KnowledgeSource) error {
	var cfg RAGConfig
	if err := asset.GetConfig(&cfg); err != nil {
		cfg = DefaultRAGConfig()
	}
	if cfg.ChunkSize <= 0 {
		cfg.ChunkSize = 512
	}
	if cfg.ChunkOverlap < 0 {
		cfg.ChunkOverlap = 0
	}

	// Update status to building
	if err := e.assetRepo.UpdateStatus(asset.ID, AssetStatusBuilding); err != nil {
		return fmt.Errorf("update status: %w", err)
	}

	// --- Phase 1: Chunking ---
	totalCreated := 0
	for _, src := range sources {
		if src.Content == "" {
			continue
		}
		// Check if chunks already exist for this source
		existing, _ := e.chunkRepo.ListByAssetAndSource(asset.ID, src.ID)
		if len(existing) > 0 {
			continue // skip — already chunked
		}

		chunks := splitIntoChunks(src.Content, cfg.ChunkSize, cfg.ChunkOverlap)
		var ragChunks []RAGChunk
		for i, chunk := range chunks {
			ragChunks = append(ragChunks, RAGChunk{
				AssetID:    asset.ID,
				SourceID:   src.ID,
				Content:    chunk,
				ChunkIndex: i,
			})
		}
		if len(ragChunks) > 0 {
			if err := e.chunkRepo.CreateBatch(ragChunks); err != nil {
				slog.Error("failed to create chunks", "asset_id", asset.ID, "source_id", src.ID, "error", err)
				continue
			}
			totalCreated += len(ragChunks)
		}
	}

	// --- Phase 2: Embedding ---
	if asset.EmbeddingProviderID != nil && asset.EmbeddingModelID != "" {
		if err := e.embeddingSvc.GenerateRAGEmbeddings(ctx, asset.ID); err != nil {
			slog.Error("failed to generate RAG embeddings", "asset_id", asset.ID, "error", err)
			// Non-fatal: chunks are stored, just without vectors. Text search still works.
		}
	}

	// Log
	_ = e.logRepo.Create(&KnowledgeLog{
		AssetID:      asset.ID,
		Action:       KnowledgeLogCompile,
		NodesCreated: totalCreated,
		Details:      fmt.Sprintf("created %d chunks from %d sources", totalCreated, len(sources)),
	})

	// Update status
	if err := e.assetRepo.UpdateStatus(asset.ID, AssetStatusReady); err != nil {
		return fmt.Errorf("update status: %w", err)
	}

	return nil
}

// Rebuild deletes all existing chunks and rebuilds from scratch.
func (e *NaiveChunkEngine) Rebuild(ctx context.Context, asset *KnowledgeAsset, sources []*KnowledgeSource) error {
	if err := e.chunkRepo.DeleteByAsset(asset.ID); err != nil {
		return fmt.Errorf("delete existing chunks: %w", err)
	}
	return e.Build(ctx, asset, sources)
}

// Search performs vector similarity search if embeddings are available,
// falling back to text matching otherwise.
func (e *NaiveChunkEngine) Search(ctx context.Context, asset *KnowledgeAsset, query *RecallQuery) (*RecallResult, error) {
	topK := query.TopK
	if topK <= 0 {
		topK = 5
	}

	// Try vector search if embedding is configured
	if asset.EmbeddingProviderID != nil && asset.EmbeddingModelID != "" {
		result, err := e.vectorSearch(ctx, asset, query.Query, topK)
		if err != nil {
			slog.Warn("vector search failed, falling back to text", "asset_id", asset.ID, "error", err)
		} else if len(result.Items) > 0 {
			return result, nil
		}
	}

	// Fallback: text-based search
	return e.textSearch(asset, query.Query, topK)
}

// ContentStats returns chunk counts.
func (e *NaiveChunkEngine) ContentStats(ctx context.Context, asset *KnowledgeAsset) (*ContentStats, error) {
	count, err := e.chunkRepo.CountByAsset(asset.ID)
	if err != nil {
		return nil, err
	}
	return &ContentStats{
		ChunkCount: int(count),
	}, nil
}

// ---------------------------------------------------------------------------
// Search strategies
// ---------------------------------------------------------------------------

func (e *NaiveChunkEngine) vectorSearch(ctx context.Context, asset *KnowledgeAsset, query string, topK int) (*RecallResult, error) {
	queryVec, err := e.embeddingSvc.EmbedQuery(ctx, asset, query)
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}

	matches, err := e.chunkRepo.VectorSearch(asset.ID, queryVec, topK)
	if err != nil {
		return nil, fmt.Errorf("vector search: %w", err)
	}

	result := &RecallResult{
		Debug: &RecallDebug{
			Mode:      "vector",
			TotalHits: len(matches),
		},
	}
	for _, m := range matches {
		result.Items = append(result.Items, KnowledgeUnit{
			ID:         fmt.Sprintf("chunk_%d", m.ID),
			AssetID:    asset.ID,
			UnitType:   "document_chunk",
			Content:    m.Content,
			Summary:    m.Summary,
			Score:      m.Score,
			SourceRefs: []uint{m.SourceID},
		})
	}
	return result, nil
}

func (e *NaiveChunkEngine) textSearch(asset *KnowledgeAsset, query string, topK int) (*RecallResult, error) {
	chunks, total, err := e.chunkRepo.ListByAsset(asset.ID, 1, topK*10)
	if err != nil {
		return nil, fmt.Errorf("list chunks: %w", err)
	}

	result := &RecallResult{
		Debug: &RecallDebug{
			Mode:      "text",
			TotalHits: int(total),
		},
	}

	queryLower := strings.ToLower(query)
	count := 0
	for _, chunk := range chunks {
		if count >= topK {
			break
		}
		contentLower := strings.ToLower(chunk.Content)
		if strings.Contains(contentLower, queryLower) || query == "" {
			result.Items = append(result.Items, KnowledgeUnit{
				ID:         fmt.Sprintf("chunk_%d", chunk.ID),
				AssetID:    asset.ID,
				UnitType:   "document_chunk",
				Content:    chunk.Content,
				Summary:    chunk.Summary,
				SourceRefs: []uint{chunk.SourceID},
			})
			count++
		}
	}

	return result, nil
}

// --- Text chunking ---

// splitIntoChunks splits text into chunks of approximately chunkSize characters
// with overlap.
func splitIntoChunks(text string, chunkSize, overlap int) []string {
	if chunkSize <= 0 {
		chunkSize = 512
	}
	if overlap < 0 {
		overlap = 0
	}
	if overlap >= chunkSize {
		overlap = chunkSize / 4
	}

	// Split by paragraphs first
	paragraphs := strings.Split(text, "\n\n")
	var chunks []string
	var current strings.Builder

	for _, para := range paragraphs {
		para = strings.TrimSpace(para)
		if para == "" {
			continue
		}

		if current.Len()+len(para)+2 > chunkSize && current.Len() > 0 {
			chunks = append(chunks, strings.TrimSpace(current.String()))
			// Keep overlap from the end of current chunk
			content := current.String()
			current.Reset()
			if overlap > 0 && len(content) > overlap {
				current.WriteString(content[len(content)-overlap:])
				current.WriteString("\n\n")
			}
		}

		if current.Len() > 0 {
			current.WriteString("\n\n")
		}
		current.WriteString(para)
	}

	if current.Len() > 0 {
		chunks = append(chunks, strings.TrimSpace(current.String()))
	}

	// If no paragraphs found or text is very long, fall back to fixed-size splitting
	if len(chunks) == 0 && len(text) > 0 {
		for i := 0; i < len(text); i += chunkSize - overlap {
			end := i + chunkSize
			if end > len(text) {
				end = len(text)
			}
			chunks = append(chunks, text[i:end])
			if end == len(text) {
				break
			}
		}
	}

	return chunks
}

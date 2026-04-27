package runtime

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"strings"

	"github.com/samber/do/v2"

	"metis/internal/database"
)

// RAGChunkRepo provides GORM persistence for RAGChunk (NaiveRAG chunk storage).
type RAGChunkRepo struct {
	db *database.DB
}

func NewRAGChunkRepo(i do.Injector) (*RAGChunkRepo, error) {
	db := do.MustInvoke[*database.DB](i)
	return &RAGChunkRepo{db: db}, nil
}

func (r *RAGChunkRepo) Create(chunk *RAGChunk) error {
	return r.db.Create(chunk).Error
}

func (r *RAGChunkRepo) CreateBatch(chunks []RAGChunk) error {
	if len(chunks) == 0 {
		return nil
	}
	return r.db.CreateInBatches(chunks, 100).Error
}

func (r *RAGChunkRepo) FindByID(id uint) (*RAGChunk, error) {
	var chunk RAGChunk
	if err := r.db.First(&chunk, id).Error; err != nil {
		return nil, err
	}
	return &chunk, nil
}

// ListByAsset returns paginated chunks for a given asset.
func (r *RAGChunkRepo) ListByAsset(assetID uint, page, pageSize int) ([]RAGChunk, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	q := r.db.Model(&RAGChunk{}).Where("asset_id = ?", assetID)

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var chunks []RAGChunk
	offset := (page - 1) * pageSize
	if err := q.Order("chunk_index ASC, id ASC").Offset(offset).Limit(pageSize).Find(&chunks).Error; err != nil {
		return nil, 0, err
	}

	return chunks, total, nil
}

// CountByAsset returns total chunks for an asset.
func (r *RAGChunkRepo) CountByAsset(assetID uint) (int64, error) {
	var count int64
	err := r.db.Model(&RAGChunk{}).Where("asset_id = ?", assetID).Count(&count).Error
	return count, err
}

// DeleteByAsset removes all chunks for an asset.
func (r *RAGChunkRepo) DeleteByAsset(assetID uint) error {
	return r.db.Where("asset_id = ?", assetID).Delete(&RAGChunk{}).Error
}

// DeleteBySource removes all chunks from a specific source within an asset.
func (r *RAGChunkRepo) DeleteBySource(assetID, sourceID uint) error {
	return r.db.Where("asset_id = ? AND source_id = ?", assetID, sourceID).Delete(&RAGChunk{}).Error
}

// ListByAssetAndSource returns chunks for a specific source within an asset.
func (r *RAGChunkRepo) ListByAssetAndSource(assetID, sourceID uint) ([]RAGChunk, error) {
	var chunks []RAGChunk
	err := r.db.Where("asset_id = ? AND source_id = ?", assetID, sourceID).
		Order("chunk_index ASC").Find(&chunks).Error
	return chunks, err
}

// ---------------------------------------------------------------------------
// Embedding support
// ---------------------------------------------------------------------------

// FindWithoutEmbedding returns all chunks for an asset that have no embedding yet.
func (r *RAGChunkRepo) FindWithoutEmbedding(assetID uint) ([]RAGChunk, error) {
	var chunks []RAGChunk
	err := r.db.Where("asset_id = ? AND (embedding IS NULL OR embedding = '')", assetID).
		Order("id ASC").Find(&chunks).Error
	return chunks, err
}

// UpdateEmbedding stores the embedding vector as a JSON array string.
func (r *RAGChunkRepo) UpdateEmbedding(chunkID uint, embedding []float32) error {
	data, err := json.Marshal(embedding)
	if err != nil {
		return fmt.Errorf("marshal embedding: %w", err)
	}
	return r.db.Model(&RAGChunk{}).Where("id = ?", chunkID).
		Update("embedding", string(data)).Error
}

// EnsureVectorIndex creates a vector search index if the database supports it.
// For PostgreSQL with pgvector, this creates an HNSW index on a generated column.
// For other databases (SQLite), this is a no-op.
func (r *RAGChunkRepo) EnsureVectorIndex(dimension int) error {
	dialector := r.db.Dialector.Name()
	if dialector != "postgres" {
		slog.Debug("vector index not supported for this database", "dialector", dialector)
		return nil
	}

	// First ensure the pgvector extension exists
	if err := r.db.Exec("CREATE EXTENSION IF NOT EXISTS vector").Error; err != nil {
		slog.Warn("failed to create pgvector extension", "error", err)
		return fmt.Errorf("create pgvector extension: %w", err)
	}

	// Add a vector column if it doesn't exist
	addCol := fmt.Sprintf(
		`DO $$ BEGIN
			ALTER TABLE ai_rag_chunks ADD COLUMN embedding_vec vector(%d);
		EXCEPTION WHEN duplicate_column THEN NULL;
		END $$`, dimension)
	if err := r.db.Exec(addCol).Error; err != nil {
		return fmt.Errorf("add vector column: %w", err)
	}

	// Sync embedding_vec from embedding JSON text
	if err := r.db.Exec(
		`UPDATE ai_rag_chunks SET embedding_vec = embedding::vector WHERE embedding IS NOT NULL AND embedding != '' AND embedding_vec IS NULL`,
	).Error; err != nil {
		slog.Warn("failed to sync embedding_vec from JSON", "error", err)
	}

	// Create HNSW index (idempotent — IF NOT EXISTS)
	idx := fmt.Sprintf(
		`CREATE INDEX IF NOT EXISTS idx_rag_chunks_embedding_vec ON ai_rag_chunks USING hnsw (embedding_vec vector_cosine_ops)`)
	if err := r.db.Exec(idx).Error; err != nil {
		return fmt.Errorf("create hnsw index: %w", err)
	}

	slog.Info("pgvector HNSW index ensured", "dimension", dimension)
	return nil
}

// VectorSearch performs cosine similarity search using pgvector.
// Falls back to in-memory cosine similarity for non-Postgres databases.
func (r *RAGChunkRepo) VectorSearch(assetID uint, queryVec []float32, topK int) ([]RAGChunkWithScore, error) {
	dialector := r.db.Dialector.Name()

	if dialector == "postgres" {
		return r.vectorSearchPgvector(assetID, queryVec, topK)
	}
	return r.vectorSearchFallback(assetID, queryVec, topK)
}

// RAGChunkWithScore pairs a chunk with its similarity score.
type RAGChunkWithScore struct {
	RAGChunk
	Score float64
}

// vectorSearchPgvector uses pgvector's <=> cosine distance operator.
func (r *RAGChunkRepo) vectorSearchPgvector(assetID uint, queryVec []float32, topK int) ([]RAGChunkWithScore, error) {
	vecStr := float32SliceToVectorLiteral(queryVec)

	query := fmt.Sprintf(
		`SELECT id, asset_id, source_id, content, summary, metadata, chunk_index, parent_chunk_id,
		        1 - (embedding_vec <=> '%s'::vector) AS score
		 FROM ai_rag_chunks
		 WHERE asset_id = ? AND embedding_vec IS NOT NULL
		 ORDER BY embedding_vec <=> '%s'::vector
		 LIMIT ?`, vecStr, vecStr)

	var results []RAGChunkWithScore
	if err := r.db.Raw(query, assetID, topK).Scan(&results).Error; err != nil {
		return nil, fmt.Errorf("pgvector search: %w", err)
	}
	return results, nil
}

// vectorSearchFallback loads all chunks with embeddings and computes cosine similarity in Go.
func (r *RAGChunkRepo) vectorSearchFallback(assetID uint, queryVec []float32, topK int) ([]RAGChunkWithScore, error) {
	var chunks []RAGChunk
	if err := r.db.Where("asset_id = ? AND embedding IS NOT NULL AND embedding != ''", assetID).
		Find(&chunks).Error; err != nil {
		return nil, fmt.Errorf("load chunks: %w", err)
	}

	type scored struct {
		idx   int
		score float64
	}
	var items []scored
	for i, c := range chunks {
		var emb []float32
		if err := json.Unmarshal([]byte(c.EmbeddingJSON), &emb); err != nil {
			continue
		}
		s := cosineSimilarity(queryVec, emb)
		items = append(items, scored{idx: i, score: s})
	}

	sort.Slice(items, func(a, b int) bool { return items[a].score > items[b].score })

	if topK > len(items) {
		topK = len(items)
	}
	results := make([]RAGChunkWithScore, topK)
	for i := 0; i < topK; i++ {
		results[i] = RAGChunkWithScore{
			RAGChunk: chunks[items[i].idx],
			Score:    items[i].score,
		}
	}
	return results, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// float32SliceToVectorLiteral formats []float32 as a pgvector literal "[0.1,0.2,...]".
func float32SliceToVectorLiteral(v []float32) string {
	var b strings.Builder
	b.WriteByte('[')
	for i, f := range v {
		if i > 0 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, "%g", f)
	}
	b.WriteByte(']')
	return b.String()
}

func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	denom := math.Sqrt(normA) * math.Sqrt(normB)
	if denom == 0 {
		return 0
	}
	return dot / denom
}

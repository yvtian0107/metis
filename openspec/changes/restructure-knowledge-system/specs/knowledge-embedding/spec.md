## MODIFIED Requirements

### Requirement: Dual embedding paths
The system SHALL support two embedding paths based on knowledge asset category:
- **Graph embedding (kg)**: Embed node `title + "\n" + summary`, store as `vecf32` property in FalkorDB, manage HNSW index. This is the existing behavior.
- **RAG embedding (kb)**: Embed chunk content, store as vector column in the relational chunk table (pgvector or equivalent). Manage vector index in the relational store.

#### Scenario: Graph asset embedding
- **WHEN** a knowledge graph compilation completes
- **THEN** system SHALL generate embeddings for all nodes (except index nodes) and store in FalkorDB with HNSW index

#### Scenario: RAG asset embedding
- **WHEN** a knowledge base build completes
- **THEN** system SHALL generate embeddings for all chunks and store in the relational chunk table with vector index

### Requirement: Per-asset embedding configuration
Each knowledge asset (both kb and kg) SHALL have its own embedding_provider_id and embedding_model_id configuration. The embedding client SHALL be resolved per-asset at build/compile time.

#### Scenario: Different embedding models
- **WHEN** knowledge base KB1 uses embedding model A and knowledge graph KG1 uses embedding model B
- **THEN** system SHALL use model A for KB1's chunks and model B for KG1's nodes independently

### Requirement: Embedding batch processing
The system SHALL process embeddings in batches to avoid API rate limits. Batch size SHALL be configurable per asset type. Index nodes (node_type=index) in knowledge graphs SHALL be skipped.

#### Scenario: Batch embedding generation
- **WHEN** a build/compile produces 500 items needing embedding
- **THEN** system SHALL process them in configurable batch sizes, reporting progress for each batch

### Requirement: HNSW index lifecycle for graphs
For knowledge graphs, the HNSW vector index SHALL be dropped before compilation and recreated after all embeddings are written. Dimension is inferred from the first embedding vector.

#### Scenario: Index rebuild during compile
- **WHEN** a knowledge graph compilation starts
- **THEN** system SHALL drop the existing HNSW index, and after embeddings are written, create a new HNSW index with the correct dimension

### Requirement: Vector index for RAG
For knowledge bases, the vector index SHALL be managed in the relational store (pgvector). Index creation and updates SHALL be handled during the build process.

#### Scenario: Vector index creation
- **WHEN** a knowledge base build completes and chunks have embeddings
- **THEN** system SHALL ensure a vector index exists on the chunk table for the asset's embedding dimension

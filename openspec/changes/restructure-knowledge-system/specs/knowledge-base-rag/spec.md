## ADDED Requirements

### Requirement: Knowledge base entity (NaiveRAG)
The system SHALL support a knowledge base entity for document-based retrieval (NaiveRAG). Each knowledge base SHALL have: id, name, description, category (fixed as `kb`), type (chunking strategy), status (idle/building/ready/error), embedding_provider_id, embedding_model_id, config (JSON, type-specific), auto_build (boolean), and timestamps.

#### Scenario: Create knowledge base with type
- **WHEN** admin creates a knowledge base with name "Product Docs" and type "naive_chunk"
- **THEN** system SHALL create the entity with category=kb, type=naive_chunk, status=idle

#### Scenario: Type is immutable after creation
- **WHEN** admin attempts to update the type of an existing knowledge base
- **THEN** system SHALL return a 400 error "type cannot be changed after creation"

### Requirement: Knowledge base types (chunking strategies)
The system SHALL support the following knowledge base types, each representing a different chunking and indexing strategy:
- `naive_chunk`: Standard chunking by paragraph or fixed length with vector retrieval. Default type.
- `parent_child`: Small chunks for retrieval, large chunks for context return. Suitable for long documents.
- `summary_first`: Generate summaries before indexing. Suitable for policy/regulation documents.
- `qa_extract`: Extract question-answer pairs from documents. Suitable for FAQ scenarios.

#### Scenario: Create with default type
- **WHEN** admin creates a knowledge base without specifying type
- **THEN** system SHALL default to type `naive_chunk`

#### Scenario: List available types
- **WHEN** frontend requests available knowledge base types
- **THEN** system SHALL return type metadata including identifier, display name, description, and default config schema

### Requirement: Knowledge base source management
The system SHALL allow associating sources from the source pool with a knowledge base. Sources are referenced (not copied).

#### Scenario: Add sources to knowledge base
- **WHEN** admin selects sources S1, S2, S3 from the source pool and adds them to knowledge base KB1
- **THEN** system SHALL create association records and mark KB1 as needing build if auto_build is enabled

#### Scenario: Remove source from knowledge base
- **WHEN** admin removes source S1 from knowledge base KB1
- **THEN** system SHALL remove the association and mark KB1 as needing rebuild

### Requirement: Knowledge base build
The system SHALL build a knowledge base by processing associated sources through the selected type's chunking strategy, generating embeddings, and creating vector + fulltext indexes. Build is an async task.

#### Scenario: Trigger build
- **WHEN** admin triggers build on knowledge base KB1 with type naive_chunk
- **THEN** system SHALL enqueue an async task that: reads all associated sources, chunks content according to naive_chunk strategy, generates embeddings, stores chunks with vectors in the vector store, creates fulltext index

#### Scenario: Incremental build
- **WHEN** admin triggers build on KB1 that already has chunks, and only source S3 is new
- **THEN** system SHALL only process S3 and add new chunks without rebuilding existing ones

#### Scenario: Full rebuild
- **WHEN** admin triggers full rebuild on KB1
- **THEN** system SHALL delete all existing chunks and rebuild from all associated sources

### Requirement: Knowledge base chunk storage
The system SHALL store chunks in a relational table with vector column. Each chunk SHALL have: id, asset_id, source_id, content, summary, metadata (JSON), embedding (vector), chunk_index, parent_chunk_id (nullable, for parent_child type), and timestamps.

#### Scenario: Naive chunk storage
- **WHEN** build completes for a naive_chunk knowledge base
- **THEN** chunks SHALL be stored with content segments and their embeddings

#### Scenario: Parent-child chunk storage
- **WHEN** build completes for a parent_child knowledge base
- **THEN** child chunks (small, for retrieval) SHALL reference parent chunks (large, for context) via parent_chunk_id

### Requirement: Knowledge base search
The system SHALL support searching a knowledge base with multiple retrieval modes: vector, fulltext, hybrid. Default mode is hybrid.

#### Scenario: Vector search
- **WHEN** query is sent with mode=vector
- **THEN** system SHALL embed the query, perform vector similarity search, return top-K KnowledgeUnit items

#### Scenario: Fulltext search
- **WHEN** query is sent with mode=fulltext
- **THEN** system SHALL perform fulltext search on chunk content, return top-K KnowledgeUnit items

#### Scenario: Hybrid search
- **WHEN** query is sent with mode=hybrid
- **THEN** system SHALL perform both vector and fulltext search, merge and re-rank results, return top-K KnowledgeUnit items

#### Scenario: Parent-child context return
- **WHEN** a child chunk is hit during search on a parent_child knowledge base
- **THEN** system SHALL return the parent chunk's content as the context, with the child chunk as the match reference

### Requirement: Knowledge base CRUD API
The system SHALL provide REST endpoints under `/api/v1/ai/knowledge/bases`:
- `POST /` — create knowledge base
- `GET /` — list with pagination, type filter, status filter
- `GET /:id` — get detail including chunk count, source count
- `PUT /:id` — update (name, description, config, embedding config; type immutable)
- `DELETE /:id` — delete knowledge base and all its chunks
- `POST /:id/build` — trigger incremental build
- `POST /:id/rebuild` — trigger full rebuild
- `GET /:id/progress` — get build progress
- `GET /:id/sources` — list associated sources
- `POST /:id/sources` — associate sources
- `DELETE /:id/sources/:sourceId` — remove source association
- `GET /:id/chunks` — list chunks with pagination
- `GET /:id/search` — search knowledge base

#### Scenario: Create and build
- **WHEN** admin creates a knowledge base, associates sources, and triggers build
- **THEN** system SHALL process sources through the selected strategy and produce searchable chunks

#### Scenario: Delete with cleanup
- **WHEN** admin deletes a knowledge base
- **THEN** system SHALL delete all chunks, vector indexes, and source associations (but NOT the sources themselves)

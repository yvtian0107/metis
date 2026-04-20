## ADDED Requirements

### Requirement: Unified KnowledgeEngine interface
The system SHALL define a unified `KnowledgeEngine` interface that all knowledge processing strategies implement. The interface SHALL include: Build, Rebuild, Search, and ContentStats methods.

#### Scenario: Engine registration
- **WHEN** the application starts
- **THEN** all available engines SHALL be registered with their `category:type` key (e.g., `kb:naive_chunk`, `kg:concept_map`)

#### Scenario: Engine routing
- **WHEN** a build or search operation is requested for a knowledge asset
- **THEN** system SHALL look up the engine by the asset's `category:type` and delegate to that engine

#### Scenario: Unknown engine type
- **WHEN** an operation is requested for an unregistered `category:type`
- **THEN** system SHALL return a 400 error "unsupported knowledge type"

### Requirement: Unified KnowledgeAsset model
The system SHALL use a single `ai_knowledge_assets` table for both knowledge bases and knowledge graphs with fields: id, name, description, category (`kb`/`kg`), type (strategy identifier), status, config (JSON), compile_model_id, embedding_provider_id, embedding_model_id, auto_build, source_count, and timestamps.

#### Scenario: Create kb asset
- **WHEN** a knowledge base is created with type naive_chunk
- **THEN** system SHALL store a record with category=kb, type=naive_chunk

#### Scenario: Create kg asset
- **WHEN** a knowledge graph is created with type concept_map
- **THEN** system SHALL store a record with category=kg, type=concept_map

#### Scenario: Category determines API behavior
- **WHEN** API endpoints for knowledge bases and knowledge graphs are called
- **THEN** they SHALL filter by the appropriate category value

### Requirement: Unified RecallResult protocol
The system SHALL define a unified recall result structure returned by all engine Search methods: items (KnowledgeUnit array), relations (optional, only for graph engines), sources (source references), and debug info (optional).

#### Scenario: RAG engine search result
- **WHEN** a knowledge base engine returns search results
- **THEN** result SHALL contain items (chunks as KnowledgeUnit) and sources, with relations empty

#### Scenario: Graph engine search result
- **WHEN** a knowledge graph engine returns search results
- **THEN** result SHALL contain items (nodes as KnowledgeUnit), relations (edges), and sources

### Requirement: KnowledgeUnit standard structure
Each KnowledgeUnit SHALL have: id, asset_id, unit_type (document_chunk/fact/entity/relation_summary), title, content, summary, metadata (JSON), source_refs (source IDs), and score (when returned from search).

#### Scenario: Chunk unit
- **WHEN** a RAG engine returns a chunk
- **THEN** it SHALL be wrapped as KnowledgeUnit with unit_type=document_chunk

#### Scenario: Graph node unit
- **WHEN** a graph engine returns a node
- **THEN** it SHALL be wrapped as KnowledgeUnit with unit_type=entity or fact

### Requirement: Type metadata registry
The system SHALL provide an API to list available types for each category, returning metadata: identifier, display_name, description, default_config_schema, and icon.

#### Scenario: List knowledge base types
- **WHEN** frontend requests `GET /api/v1/ai/knowledge/types?category=kb`
- **THEN** system SHALL return all registered knowledge base types with metadata

#### Scenario: List knowledge graph types
- **WHEN** frontend requests `GET /api/v1/ai/knowledge/types?category=kg`
- **THEN** system SHALL return all registered knowledge graph types with metadata

### Requirement: Sidecar unified search API
The system SHALL provide a unified search endpoint for Sidecar that accepts asset IDs and routes to the correct engine. The endpoint SHALL merge results from multiple assets if an agent is bound to several.

#### Scenario: Sidecar search across multiple assets
- **WHEN** Sidecar sends a search request with asset IDs [KB1, KG1]
- **THEN** system SHALL query KB1 via RAG engine and KG1 via graph engine, merge results into a single RecallResult

#### Scenario: Sidecar search single asset
- **WHEN** Sidecar sends a search request with a single asset ID
- **THEN** system SHALL route to the correct engine and return results

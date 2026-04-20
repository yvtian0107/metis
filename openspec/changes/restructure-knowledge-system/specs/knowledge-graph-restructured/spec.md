## ADDED Requirements

### Requirement: Knowledge graph as independent entity
The system SHALL support a knowledge graph entity for entity-relationship based knowledge. Each knowledge graph SHALL have: id, name, description, category (fixed as `kg`), type (graph construction strategy), status (idle/compiling/ready/error), compile_model_id, embedding_provider_id, embedding_model_id, config (JSON, type-specific), auto_compile (boolean), and timestamps.

#### Scenario: Create knowledge graph with type
- **WHEN** admin creates a knowledge graph with name "Product Relations" and type "concept_map"
- **THEN** system SHALL create the entity with category=kg, type=concept_map, status=idle

#### Scenario: Type is immutable after creation
- **WHEN** admin attempts to update the type of an existing knowledge graph
- **THEN** system SHALL return a 400 error "type cannot be changed after creation"

### Requirement: Knowledge graph types (construction strategies)
The system SHALL support the following knowledge graph types, each representing a different graph construction strategy:
- `concept_map`: Extract concepts and relationships using Map-Reduce. Current existing capability. Default type.
- `entity_relation`: Extract named entities (person/org/product) and their relationships. Suitable for business domains.
- `light_graph`: LightRAG-style fast construction with lower cost. Trades precision for speed.
- `event_graph`: Extract event chains and causal relationships. Suitable for process/timeline knowledge.

#### Scenario: Create with default type
- **WHEN** admin creates a knowledge graph without specifying type
- **THEN** system SHALL default to type `concept_map`

#### Scenario: List available types
- **WHEN** frontend requests available knowledge graph types
- **THEN** system SHALL return type metadata including identifier, display name, description, and default config schema

### Requirement: Knowledge graph source management
The system SHALL allow associating sources from the source pool with a knowledge graph. Sources are referenced (not copied).

#### Scenario: Add sources to knowledge graph
- **WHEN** admin selects sources from the source pool and adds them to a knowledge graph
- **THEN** system SHALL create association records and mark the graph as needing compile if auto_compile is enabled

#### Scenario: Remove source from knowledge graph
- **WHEN** admin removes a source from a knowledge graph
- **THEN** system SHALL remove the association and mark the graph as needing recompile

### Requirement: Knowledge graph compilation
The system SHALL compile a knowledge graph by processing associated sources through the selected type's construction strategy, writing nodes and edges to FalkorDB, generating embeddings, and creating indexes. Compilation is an async task.

#### Scenario: Trigger compilation (concept_map)
- **WHEN** admin triggers compile on a concept_map knowledge graph
- **THEN** system SHALL enqueue an async task that: reads sources, extracts concepts via LLM (Map phase), merges and deduplicates (Reduce phase), writes nodes and edges to FalkorDB, generates embeddings, creates fulltext and HNSW indexes

#### Scenario: Incremental compilation
- **WHEN** admin triggers compile on a graph that already has nodes, with new sources added
- **THEN** system SHALL process only new sources and merge results with existing graph, including cascade impact analysis

#### Scenario: Full recompilation
- **WHEN** admin triggers full recompile
- **THEN** system SHALL delete the existing FalkorDB graph and rebuild from all associated sources

### Requirement: Knowledge graph lint
The system SHALL run lint checks after compilation to detect quality issues: orphan nodes (no edges), sparse nodes (content below minimum length), contradiction edges.

#### Scenario: Lint after compile
- **WHEN** compilation completes
- **THEN** system SHALL run lint and record results in the compile log

### Requirement: Knowledge graph compile log
The system SHALL log each compilation operation with: action, model_id, nodes_created, nodes_updated, edges_created, lint_issues, cascade_details, error_message.

#### Scenario: View compile logs
- **WHEN** admin views compile logs for a knowledge graph
- **THEN** system SHALL return a chronological list of compilation operations with statistics

### Requirement: Knowledge graph search
The system SHALL support searching a knowledge graph with vector similarity + graph expansion. Query is embedded, top-K similar nodes are found, then 1-2 hop graph traversal expands related nodes.

#### Scenario: Hybrid search with graph expansion
- **WHEN** query is sent to a knowledge graph
- **THEN** system SHALL embed the query, find top-K similar nodes via vector search, expand 1-2 hops via graph traversal, return KnowledgeUnit items with Relations

#### Scenario: Fallback to fulltext
- **WHEN** a knowledge graph has no embeddings yet
- **THEN** system SHALL fall back to fulltext search on node title + summary

### Requirement: Knowledge graph CRUD API
The system SHALL provide REST endpoints under `/api/v1/ai/knowledge/graphs`:
- `POST /` — create knowledge graph
- `GET /` — list with pagination, type filter, status filter
- `GET /:id` — get detail including node_count, edge_count, source_count
- `PUT /:id` — update (name, description, config, model config; type immutable)
- `DELETE /:id` — delete graph and FalkorDB data
- `POST /:id/compile` — trigger incremental compile
- `POST /:id/recompile` — trigger full recompile
- `GET /:id/progress` — get compile progress
- `GET /:id/sources` — list associated sources
- `POST /:id/sources` — associate sources
- `DELETE /:id/sources/:sourceId` — remove source association
- `GET /:id/nodes` — list nodes with pagination
- `GET /:id/nodes/:nodeId` — get node detail
- `GET /:id/nodes/:nodeId/graph` — get node neighborhood graph
- `GET /:id/graph` — get full graph
- `GET /:id/logs` — get compile logs
- `GET /:id/search` — search knowledge graph

#### Scenario: Create and compile
- **WHEN** admin creates a knowledge graph, associates sources, and triggers compile
- **THEN** system SHALL process sources through the selected strategy and produce a searchable graph

#### Scenario: Delete with cleanup
- **WHEN** admin deletes a knowledge graph
- **THEN** system SHALL delete the FalkorDB graph, compile logs, and source associations (but NOT the sources themselves)

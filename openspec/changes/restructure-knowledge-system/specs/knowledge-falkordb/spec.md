## MODIFIED Requirements

### Requirement: FalkorDB graph lifecycle
FalkorDB SHALL be used exclusively for knowledge graph storage (category=kg). Graph naming convention changes from `kb_<id>` to `kg_<asset_id>`. First compilation creates the graph, deleting a knowledge graph asset triggers GRAPH.DELETE, full recompile deletes and recreates the graph.

#### Scenario: Graph creation on first compile
- **WHEN** a knowledge graph asset is compiled for the first time
- **THEN** system SHALL create a FalkorDB graph named `kg_<asset_id>`

#### Scenario: Graph deletion on asset delete
- **WHEN** a knowledge graph asset is deleted
- **THEN** system SHALL execute GRAPH.DELETE on `kg_<asset_id>`

#### Scenario: Full recompile
- **WHEN** a full recompile is triggered
- **THEN** system SHALL delete graph `kg_<asset_id>` and recreate it from scratch

### Requirement: FalkorDB connection management
FalkorDB connection SHALL be configured via metis.yaml and registered in the IOC container implementing `do.Shutdowner`. When FalkorDB is not configured, knowledge graph features SHALL degrade gracefully. When connection is lost, knowledge graph operations SHALL return 503.

#### Scenario: FalkorDB not configured
- **WHEN** FalkorDB connection is not configured in metis.yaml
- **THEN** knowledge graph creation and compilation SHALL be disabled, knowledge base (RAG) features SHALL continue to work normally

#### Scenario: FalkorDB connection lost
- **WHEN** FalkorDB connection is lost during operation
- **THEN** knowledge graph operations SHALL return 503, knowledge base (RAG) operations SHALL be unaffected

### Requirement: Node and edge storage
FalkorDB SHALL store `:KnowledgeNode` labels with properties: id, title, summary, content, node_type, source_ids (JSON), compiled_at, embedding (vecf32). Edge types: RELATED_TO, EXTENDS, CONTRADICTS, PART_OF. Upsert via MERGE on title.

#### Scenario: Upsert node by title
- **WHEN** compilation produces a node with a title that already exists in the graph
- **THEN** system SHALL MERGE (upsert) the node, updating content and metadata

#### Scenario: Create edges
- **WHEN** compilation identifies relationships between nodes
- **THEN** system SHALL create typed edges (RELATED_TO, EXTENDS, CONTRADICTS, PART_OF) with description property

### Requirement: Fulltext index
FalkorDB SHALL maintain a fulltext index on node title + summary fields for keyword search fallback.

#### Scenario: Index creation after compile
- **WHEN** compilation completes
- **THEN** system SHALL create or recreate the fulltext index on the graph

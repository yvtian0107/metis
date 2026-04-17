## ADDED Requirements

### Requirement: Knowledge base test infrastructure
The system SHALL provide a test harness for `KnowledgeBaseService` and `KnowledgeSourceService` using an in-memory SQLite database with `ai_knowledge_bases`, `ai_knowledge_sources`, and `ai_knowledge_logs` tables. The FalkorDB graph repository SHALL be mocked or stubbed in tests.

#### Scenario: Setup test database
- **WHEN** a knowledge base test initializes
- **THEN** it SHALL migrate knowledge tables into a shared-memory SQLite database

### Requirement: Test knowledge base CRUD
The service-layer test suite SHALL verify creation, update, retrieval, and deletion of knowledge bases.

#### Scenario: Create knowledge base
- **WHEN** `Create` is called with a valid `KnowledgeBase`
- **THEN** it is persisted with `CompileStatus="idle"`

#### Scenario: Update knowledge base
- **WHEN** `Update` is called with a new description
- **THEN** the persisted record reflects the change

#### Scenario: Get knowledge base
- **WHEN** `Get` is called with a valid ID
- **THEN** it returns the correct `KnowledgeBase`

#### Scenario: Get missing knowledge base returns not found
- **WHEN** `Get` is called with a non-existent ID
- **THEN** it returns `ErrKnowledgeBaseNotFound`

### Requirement: Test knowledge base deletion cascade
The service-layer test suite SHALL verify that deleting a knowledge base cleans up sources and invokes graph deletion.

#### Scenario: Delete knowledge base removes sources and graph
- **WHEN** `Delete` is called for a knowledge base that has sources and a graph
- **THEN** all sources are removed, the graph repo delete is invoked, and the base is removed

### Requirement: Test knowledge source validation
The service-layer test suite SHALL verify source creation constraints.

#### Scenario: Create a URL source with crawl settings
- **WHEN** a source of type="url" is created with `CrawlDepth=2` and a `URLPattern`
- **THEN** it is persisted with those settings and linked to the knowledge base

#### Scenario: Reject source without type
- **WHEN** a source is created with an empty type
- **THEN** creation fails with a validation error

### Requirement: Test knowledge source listing and deletion
The service-layer test suite SHALL verify source lifecycle operations.

#### Scenario: List sources by knowledge base
- **WHEN** `List` is called for a knowledge base with multiple sources
- **THEN** it returns only the sources belonging to that base

#### Scenario: Delete source
- **WHEN** `Delete` is called for an existing source
- **THEN** the source is removed

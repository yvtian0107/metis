## ADDED Requirements

### Requirement: Knowledge menu structure
The system SHALL organize knowledge features under the "知识" menu with three sub-entries: "知识管理" (source pool), "知识库" (NaiveRAG), "知识图谱" (graph). All three are peer-level entries.

#### Scenario: Navigate to knowledge section
- **WHEN** user clicks "知识" in the AI navigation
- **THEN** system SHALL show three sub-menu items: 知识管理, 知识库, 知识图谱

### Requirement: Knowledge source management page
The system SHALL provide a source management page listing all sources with: name, format, extract status, reference count (how many assets use it), file size, and last updated time. The page SHALL support file upload, URL addition, manual text input, search, and format/status filters.

#### Scenario: Upload file source
- **WHEN** user clicks upload and selects a PDF file
- **THEN** system SHALL upload the file, show it in the list with status "提取中", and update to "可用" when extraction completes

#### Scenario: View source references
- **WHEN** user views a source detail
- **THEN** system SHALL show which knowledge bases and knowledge graphs reference this source

#### Scenario: Filter sources
- **WHEN** user filters by format=PDF and status=ready
- **THEN** system SHALL show only PDF sources that have been successfully extracted

### Requirement: Knowledge base list page
The system SHALL provide a knowledge base list page showing: name, type (display name), status, source count, chunk count, and last updated time. The page SHALL support creating new knowledge bases and filtering by type and status.

#### Scenario: Create knowledge base
- **WHEN** user clicks "新建" on the knowledge base list page
- **THEN** system SHALL open a Sheet form with: name, description, type selection (card-style with descriptions), embedding model configuration

#### Scenario: Type selection UI
- **WHEN** user is creating a new knowledge base
- **THEN** system SHALL display available types as selectable cards, each showing: type name, description, and recommended use case

### Requirement: Knowledge base detail page
The system SHALL provide a knowledge base detail page with tabs: 概览, 素材, 内容, 检索测试, 设置.

#### Scenario: Overview tab
- **WHEN** user views the overview tab
- **THEN** system SHALL show: basic info, type, status, statistics (source count, chunk count), build progress (if building), and bound agents

#### Scenario: Sources tab
- **WHEN** user views the sources tab
- **THEN** system SHALL show associated sources with ability to add (select from source pool) or remove sources

#### Scenario: Content tab
- **WHEN** user views the content tab of a knowledge base
- **THEN** system SHALL show a paginated list of chunks with: content preview, source reference, chunk index, and metadata

#### Scenario: Search test tab
- **WHEN** user enters a query in the search test tab
- **THEN** system SHALL call the search API and display results as KnowledgeUnit cards with: title, content, score, and source reference

#### Scenario: Settings tab
- **WHEN** user views the settings tab
- **THEN** system SHALL show: embedding model configuration, type-specific parameters, auto-build toggle, and danger zone (delete)

### Requirement: Knowledge graph list page
The system SHALL provide a knowledge graph list page showing: name, type (display name), status, source count, node count, edge count, and last updated time. The page SHALL support creating new knowledge graphs and filtering by type and status.

#### Scenario: Create knowledge graph
- **WHEN** user clicks "新建" on the knowledge graph list page
- **THEN** system SHALL open a Sheet form with: name, description, type selection (card-style), compile model, embedding model configuration

### Requirement: Knowledge graph detail page
The system SHALL provide a knowledge graph detail page with tabs: 概览, 素材, 内容, 检索测试, 日志, 设置.

#### Scenario: Overview tab
- **WHEN** user views the overview tab
- **THEN** system SHALL show: basic info, type, status, statistics (node count, edge count, source count), compile progress (if compiling), and bound agents

#### Scenario: Sources tab
- **WHEN** user views the sources tab
- **THEN** system SHALL show associated sources with ability to add or remove, same as knowledge base

#### Scenario: Content tab - graph view
- **WHEN** user views the content tab of a knowledge graph
- **THEN** system SHALL show a force-directed graph visualization and a node table view (togglable), reusing existing graph visualization components

#### Scenario: Search test tab
- **WHEN** user enters a query in the search test tab
- **THEN** system SHALL call the search API and display results as KnowledgeUnit cards, plus a relationship graph for connected nodes

#### Scenario: Compile logs tab
- **WHEN** user views the logs tab
- **THEN** system SHALL show compilation history with: timestamp, action, model used, nodes/edges created, lint issues

#### Scenario: Settings tab
- **WHEN** user views the settings tab
- **THEN** system SHALL show: compile model, embedding model, type-specific config (e.g., target_content_length, min_content_length), auto-compile toggle, and danger zone

### Requirement: Build/compile progress display
The system SHALL show real-time build/compile progress with: current phase, progress bar, items processed / total, current item name. Progress SHALL auto-refresh every 2 seconds.

#### Scenario: Knowledge base build progress
- **WHEN** a knowledge base is building
- **THEN** system SHALL show phases: preparing → chunking → embedding → indexing → completed

#### Scenario: Knowledge graph compile progress
- **WHEN** a knowledge graph is compiling
- **THEN** system SHALL show phases: preparing → source_reading → calling_llm → node_writing → embedding → completed

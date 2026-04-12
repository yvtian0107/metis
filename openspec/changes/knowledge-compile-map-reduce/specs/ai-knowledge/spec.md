## MODIFIED Requirements

### Requirement: LLM knowledge compilation (Wiki mode)
The system SHALL compile all extracted sources in a knowledge base into a knowledge graph using an LLM via a two-phase Map-Reduce pipeline. Phase 1 (Map) processes each source independently to extract concept nodes. Phase 2 (Reduce) merges all extracted concepts, deduplicates synonymous concepts, and integrates with the existing knowledge graph. The final output uses name-driven references — concept names and source titles, not database IDs. The system resolves names to IDs when writing to the database.

#### Scenario: Trigger compilation
- **WHEN** user sends POST /api/v1/ai/knowledge-bases/:id/compile
- **THEN** system sets compile_status=compiling and enqueues ai-knowledge-compile task

#### Scenario: Map phase extracts concepts per source
- **WHEN** ai-knowledge-compile task runs
- **THEN** system SHALL iterate each completed source and call LLM independently to extract concept nodes with title, summary, content, and relationships
- **THEN** each Map call input SHALL be limited to a single source (≤ 8000 chars content) plus system prompt

#### Scenario: Map phase tolerates individual source failures
- **WHEN** a single source's Map LLM call fails (timeout, API error, parse error)
- **THEN** system SHALL log a warning and continue processing remaining sources
- **THEN** system SHALL only fail the entire compilation if ALL sources fail in the Map phase

#### Scenario: Reduce phase merges extracted concepts
- **WHEN** all Map calls complete (with at least one success)
- **THEN** system SHALL send the structured Map results (title + summary + relations per source) plus existing node summaries and cascade analysis to LLM
- **THEN** LLM SHALL merge synonymous concepts, establish cross-source relationships, and output structured JSON with new_nodes and updated_nodes

#### Scenario: Reduce output is compatible with existing format
- **WHEN** Reduce phase LLM returns results
- **THEN** the output format SHALL be identical to the existing compileOutput structure ({nodes, updated_nodes})
- **THEN** system writes Nodes to DB, resolves concept names to Node IDs, writes Edges

#### Scenario: Incremental compilation with cascade updates
- **WHEN** new Sources have been added since last compilation
- **THEN** system runs Map phase on all completed sources
- **THEN** Reduce phase receives Map results + all existing Node titles/summaries
- **THEN** LLM outputs new_nodes AND updated_nodes (existing concepts affected by new information)

#### Scenario: Full recompilation
- **WHEN** user sends POST /api/v1/ai/knowledge-bases/:id/recompile
- **THEN** system deletes all existing Nodes and Edges for this KB
- **THEN** system runs full Map-Reduce compilation from all Sources

#### Scenario: Compilation progress reports Map phase granularity
- **WHEN** Map phase is in progress
- **THEN** system SHALL update compile progress with current source index and title (e.g., "分析来源 3/10: Source Title")

#### Scenario: Compilation generates index node
- **WHEN** compilation completes
- **THEN** system creates or updates an index Node (node_type=index) containing a summary table of all concept titles and their summaries

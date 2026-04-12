## MODIFIED Requirements

### Requirement: LLM knowledge compilation (Wiki mode)
The system SHALL compile all extracted sources in a knowledge base into a knowledge graph using an LLM. Compilation produces concept Nodes with title, summary, and mandatory content (complete wiki article). Nodes without substantive content SHALL NOT be created. The LLM output uses name-driven references — concept names and source titles, not database IDs. The system resolves names to IDs when writing to the database. Edge types are limited to "related" and "contradicts".

#### Scenario: Trigger compilation
- **WHEN** user sends POST /api/v1/ai/knowledge-bases/:id/compile
- **THEN** system sets compile_status=compiling and enqueues ai-knowledge-compile task

#### Scenario: Compilation produces nodes and edges
- **WHEN** ai-knowledge-compile task runs
- **THEN** LLM receives all Source contents + existing Node titles/summaries
- **THEN** LLM outputs structured JSON with new_nodes and updated_nodes
- **THEN** every node in the output SHALL have non-empty content (complete wiki article)
- **THEN** system writes Nodes to DB, resolves concept names to Node IDs, writes Edges
- **THEN** nodes with content shorter than minContentLength (default 200 characters) SHALL be discarded
- **THEN** edges referencing a non-existent target node SHALL be skipped (no ghost node creation)

#### Scenario: Incremental compilation with cascade updates
- **WHEN** new Sources have been added since last compilation
- **THEN** LLM receives new Sources + all existing Node titles/summaries
- **THEN** LLM outputs new_nodes AND updated_nodes (existing concepts affected by new information)
- **THEN** system updates existing Nodes and Edges accordingly

#### Scenario: Full recompilation
- **WHEN** user sends POST /api/v1/ai/knowledge-bases/:id/recompile
- **THEN** system deletes all existing Nodes and Edges for this KB
- **THEN** system runs full compilation from all Sources

#### Scenario: Edge types
- **WHEN** compilation creates edges between nodes
- **THEN** edge relation SHALL be one of: "related" (default) or "contradicts" (conflicting information)
- **THEN** edges with legacy relation types (extends, part_of) SHALL be treated as "related" when read

#### Scenario: Compile configuration
- **WHEN** knowledge base has a compileConfig JSON field set
- **THEN** compilation SHALL use targetContentLength (default 4000) to guide LLM article length in the prompt
- **THEN** compilation SHALL use minContentLength (default 200) to discard undersized nodes
- **THEN** compilation SHALL use maxChunkSize (default 0 = auto) to determine the long-document threshold

### Requirement: Post-compilation lint
After compilation completes, the system SHALL automatically run quality checks on the knowledge graph.

#### Scenario: Lint detects orphan nodes
- **WHEN** a Node has no Edges connecting to any other Node
- **THEN** system logs a warning in ai_knowledge_logs with lint_issues count

#### Scenario: Lint detects contradictions
- **WHEN** two Nodes with a "contradicts" Edge exist
- **THEN** system logs the contradiction for user review

## REMOVED Requirements

### Requirement: Compilation generates index node
**Reason**: Index nodes pollute the knowledge graph with meta-data that belongs in the UI layer, not in the graph topology. After this change all nodes have substantive content, making the "has content" marker in the index meaningless. The cascade impact analysis already builds the node catalog programmatically.
**Migration**: Frontend node list page and existing graph query APIs provide the same catalog functionality. No API changes needed.

### Requirement: Lint detects sparse nodes
**Reason**: After enforcing mandatory content on all nodes, sparse nodes (content=null with 3+ edges) can no longer be created. This lint check becomes a no-op.
**Migration**: None required. The lint phase simply no longer checks for this condition.

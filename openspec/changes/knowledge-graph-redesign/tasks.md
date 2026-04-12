## 1. Model & Type Changes

- [x] 1.1 Change `KnowledgeNode.Content` from `*string` to `string` in `knowledge_model.go`, update `ToResponse()` and `HasContent` logic
- [x] 1.2 Remove `NodeTypeIndex` constant, keep only `NodeTypeConcept`
- [x] 1.3 Remove `EdgeRelationExtends` and `EdgeRelationPartOf` constants, keep only `EdgeRelationRelated` and `EdgeRelationContradicts`
- [x] 1.4 Add `CompileConfig` struct with `TargetContentLength`, `MinContentLength`, `MaxChunkSize` fields
- [x] 1.5 Add `CompileConfig` JSON field to `KnowledgeBase` model, update GORM migration

## 2. LLM Output Structs

- [x] 2.1 Change `mapNodeOutput.Content` from `*string` to `string`, replace `Related []compileRelation` with `References []string`
- [x] 2.2 Change `compileNodeOutput.Content` from `*string` to `string`, replace `Related []compileRelation` with `References []string` and add `Contradicts []string`
- [x] 2.3 Remove `compileRelation` struct

## 3. Rewrite Prompts

- [x] 3.1 Rewrite `mapSystemPrompt`: mandate non-empty content (wiki article), use `references` instead of `related`, add `targetContentLength` and `minContentLength` guidance, remove "Create nodes even for concepts that don't have enough content" rule
- [x] 3.2 Rewrite `compileSystemPrompt`: same content mandate, use `references`/`contradicts` lists, only 2 edge types, remove null-content encouragement
- [x] 3.3 Add `scanSystemPrompt` for long-doc SCAN phase (lightweight: only title + summary output)

## 4. Core Compile Pipeline Changes

- [x] 4.1 In `writeCompileOutput`: remove ghost node creation logic (lines ~667-678), skip edges where target node doesn't exist
- [x] 4.2 In `writeCompileOutput`: add `minContentLength` check — discard nodes with content shorter than threshold
- [x] 4.3 In `writeCompileOutput`: adapt edge creation to use `References` (→ related) and `Contradicts` (→ contradicts) lists instead of `Related []compileRelation`
- [x] 4.4 Remove `generateIndexNode` method and its call in `HandleCompile`
- [x] 4.5 Update `runLint` to remove sparse-node check, keep orphan and contradiction checks only

## 5. Long Document Processing

- [x] 5.1 Implement `chunkSource(content string, maxSize int) []sourceChunk` — split by section headers > paragraphs > fixed length
- [x] 5.2 Implement `scanChunks(chunks []sourceChunk) []scanResult` — parallel LLM calls using `scanSystemPrompt`, output title+summary per chunk
- [x] 5.3 Implement `mergeScannedConcepts(results []scanResult) []globalConcept` — deduplicate by exact title, build concept→chunk_indices mapping
- [x] 5.4 Implement `gatherEvidence(concept globalConcept, chunks []sourceChunk, maxSize int) string` — collect relevant paragraphs from original text, rank by mention density, apply top-K with snippet fallback
- [x] 5.5 Implement `writeConceptArticles(concepts []globalConcept, evidences map[string]string) []mapNodeOutput` — parallel LLM calls to produce full articles from evidence bundles
- [x] 5.6 Integrate long-doc path into `runMapPhase`: if source content > maxChunkSize → use three-phase; otherwise use existing fast path
- [x] 5.7 Add maxChunkSize auto-calculation: read model's context window from provider config, multiply by 0.4

## 6. FalkorDB Queries

- [x] 6.1 Update `UpsertNodeByTitle` Cypher: remove `CASE WHEN $content IS NOT NULL` logic, always set content
- [x] 6.2 Update `FindAllNodes` and related queries: remove index-node filtering (no longer needed)
- [x] 6.3 Map legacy edge types on read: extends/part_of → related when returned from queries

## 7. Frontend Adaptations

- [x] 7.1 Update knowledge graph view to remove index node special rendering
- [x] 7.2 Simplify edge type color scheme: only `related` (default) and `contradicts` (red/highlighted)
- [x] 7.3 Add compile config fields (targetContentLength, minContentLength) to knowledge base edit form
- [x] 7.4 Update i18n locale files (en.json, zh-CN.json) with new config labels

## 8. Verification

- [ ] 8.1 Test fast-path compilation: short source produces nodes with mandatory content, no empty nodes
- [ ] 8.2 Test long-doc path: source exceeding maxChunkSize triggers three-phase, all produced nodes have content
- [ ] 8.3 Test edge creation: references to non-existent concepts are skipped (no ghost nodes)
- [ ] 8.4 Test incremental compilation: updated_nodes merge correctly, contradicts edges created
- [ ] 8.5 Test recompilation: old nodes replaced, no index node generated
- [x] 8.6 Verify `go build -tags dev ./cmd/server/` compiles without errors
- [x] 8.7 Verify `cd web && bun run lint` passes

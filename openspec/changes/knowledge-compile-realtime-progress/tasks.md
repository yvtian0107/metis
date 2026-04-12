## 1. Database Migration

- [x] 1.1 Add `compile_progress` JSON column to `knowledge_bases` table in `internal/app/ai/knowledge_model.go`
- [x] 1.2 Ensure GORM auto-migration includes the new column

## 2. Backend Progress Tracking

- [x] 2.1 Define `CompileProgress` struct in `internal/app/ai/knowledge_compile_service.go` with fields: Stage, Sources, Nodes, Embeddings, CurrentItem
- [x] 2.2 Add `updateProgress()` helper method to save progress to database
- [x] 2.3 Initialize progress at start of `HandleCompile` with stage "preparing"
- [x] 2.4 Update sources done count after each source is read
- [x] 2.5 Set stage to "calling_llm" and update current item before LLM call
- [x] 2.6 Update nodes total when LLM returns node list
- [x] 2.7 Update nodes done count after each node is written (batch update every 5 nodes)
- [x] 2.8 Set stage to "generating_embeddings" before embedding generation
- [x] 2.9 Update embeddings done count after each embedding is generated (batch update every 5 nodes)
- [x] 2.10 Set stage to "completed" when compilation finishes
- [x] 2.11 Clear progress JSON when compilation completes successfully

## 3. Backend API

- [x] 3.1 Add `GetCompileProgress` method to `KnowledgeBaseHandler`
- [x] 3.2 Register new route `GET /api/v1/ai/knowledge-bases/:id/progress` in `internal/app/ai/app.go`
- [x] 3.3 Return 404 if knowledge base not found
- [x] 3.4 Return empty progress with stage "idle" if not compiling

## 4. Frontend Types

- [x] 4.1 Add `CompileProgress` interface to `web/src/apps/ai/pages/knowledge/types.ts`
- [x] 4.2 Define progress stage union type: "preparing" | "calling_llm" | "writing_nodes" | "generating_embeddings" | "completed" | "idle"

## 5. Frontend Progress Component

- [x] 5.1 Create `CompileProgressPanel` component at `web/src/apps/ai/pages/knowledge/components/compile-progress.tsx`
- [x] 5.2 Display stage name with Chinese translation
- [x] 5.3 Display sources progress bar with done/total count
- [x] 5.4 Display nodes progress bar with done/total count (show "?" if total unknown)
- [x] 5.5 Display embeddings progress bar with done/total count
- [x] 5.6 Display current item text
- [x] 5.7 Add stage-specific icons/animations (spinner for active stage)

## 6. Frontend Integration

- [x] 6.1 Create `useCompileProgress` hook in `web/src/apps/ai/pages/knowledge/hooks/use-compile-progress.ts`
- [x] 6.2 Implement 2-second interval polling when compile status is "compiling"
- [x] 6.3 Stop polling when status changes to "completed" or "error"
- [x] 6.4 Import and use hook in `web/src/apps/ai/pages/knowledge/[id].tsx`
- [x] 6.5 Add `<CompileProgressPanel />` to knowledge base detail page
- [x] 6.6 Conditionally show panel only when status is "compiling"

## 7. Testing & Verification

- [ ] 7.1 Test progress updates correctly during source reading phase
- [ ] 7.2 Test progress shows "calling_llm" stage during LLM call
- [ ] 7.3 Test nodes count displays correctly when LLM returns
- [ ] 7.4 Test progress increments during node writing
- [ ] 7.5 Test progress increments during embedding generation
- [ ] 7.6 Test progress clears after compilation completes
- [ ] 7.7 Test polling stops when user navigates away
- [ ] 7.8 Test page refresh shows current progress

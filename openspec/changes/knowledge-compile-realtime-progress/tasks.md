## 1. Database Migration

- [ ] 1.1 Add `compile_progress` JSON column to `knowledge_bases` table in `internal/app/ai/knowledge_model.go`
- [ ] 1.2 Ensure GORM auto-migration includes the new column

## 2. Backend Progress Tracking

- [ ] 2.1 Define `CompileProgress` struct in `internal/app/ai/knowledge_compile_service.go` with fields: Stage, Sources, Nodes, Embeddings, CurrentItem
- [ ] 2.2 Add `updateProgress()` helper method to save progress to database
- [ ] 2.3 Initialize progress at start of `HandleCompile` with stage "preparing"
- [ ] 2.4 Update sources done count after each source is read
- [ ] 2.5 Set stage to "calling_llm" and update current item before LLM call
- [ ] 2.6 Update nodes total when LLM returns node list
- [ ] 2.7 Update nodes done count after each node is written (batch update every 5 nodes)
- [ ] 2.8 Set stage to "generating_embeddings" before embedding generation
- [ ] 2.9 Update embeddings done count after each embedding is generated (batch update every 5 nodes)
- [ ] 2.10 Set stage to "completed" when compilation finishes
- [ ] 2.11 Clear progress JSON when compilation completes successfully

## 3. Backend API

- [x] 3.1 Add `GetCompileProgress` method to `KnowledgeBaseHandler`
- [x] 3.2 Register new route `GET /api/v1/ai/knowledge-bases/:id/progress` in `internal/app/ai/app.go`
- [x] 3.3 Return 404 if knowledge base not found
- [x] 3.4 Return empty progress with stage "idle" if not compiling

## 4. Frontend Types

- [x] 4.1 Add `CompileProgress` interface to `web/src/apps/ai/pages/knowledge/types.ts`
- [x] 4.2 Define progress stage union type: "preparing" | "calling_llm" | "writing_nodes" | "generating_embeddings" | "completed" | "idle"

## 5. Frontend Progress Component

- [ ] 5.1 Create `CompileProgressPanel` component at `web/src/apps/ai/pages/knowledge/components/compile-progress.tsx`
- [ ] 5.2 Display stage name with Chinese translation
- [ ] 5.3 Display sources progress bar with done/total count
- [ ] 5.4 Display nodes progress bar with done/total count (show "?" if total unknown)
- [ ] 5.5 Display embeddings progress bar with done/total count
- [ ] 5.6 Display current item text
- [ ] 5.7 Add stage-specific icons/animations (spinner for active stage)

## 6. Frontend Integration

- [ ] 6.1 Create `useCompileProgress` hook in `web/src/apps/ai/pages/knowledge/hooks/use-compile-progress.ts`
- [ ] 6.2 Implement 2-second interval polling when compile status is "compiling"
- [ ] 6.3 Stop polling when status changes to "completed" or "error"
- [ ] 6.4 Import and use hook in `web/src/apps/ai/pages/knowledge/[id].tsx`
- [ ] 6.5 Add `<CompileProgressPanel />` to knowledge base detail page
- [ ] 6.6 Conditionally show panel only when status is "compiling"

## 7. Testing & Verification

- [ ] 7.1 Test progress updates correctly during source reading phase
- [ ] 7.2 Test progress shows "calling_llm" stage during LLM call
- [ ] 7.3 Test nodes count displays correctly when LLM returns
- [ ] 7.4 Test progress increments during node writing
- [ ] 7.5 Test progress increments during embedding generation
- [ ] 7.6 Test progress clears after compilation completes
- [ ] 7.7 Test polling stops when user navigates away
- [ ] 7.8 Test page refresh shows current progress

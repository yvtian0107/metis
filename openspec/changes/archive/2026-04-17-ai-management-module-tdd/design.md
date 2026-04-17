## Context

The AI App (`internal/app/ai/`) has extensive production code (Provider, Model, Tool, MCPServer, Skill, Agent, Session, Gateway, Executor, Knowledge Base) but only one test file: `data_stream_test.go`. Other apps like `license` and `itsm` have established patterns:
- In-memory SQLite (`glebarez/sqlite`) with `gorm.AutoMigrate`
- Service-level tests using real repositories (no mocks for DB)
- Handler-level tests with `httptest` and Gin test context

The AI App shares the same stack, so we can follow the same pattern. However, the Agent Gateway and Executors have external dependencies (`llm.Client`, tool execution) that must be mocked.

## Goals / Non-Goals

**Goals:**
- Establish shared test infrastructure for `internal/app/ai/`.
- Achieve service-layer TDD coverage for Provider, Model, Tool Registry, Agent, and Knowledge Base services.
- Achieve unit test coverage for ReAct executor and Gateway orchestration using mocks.
- Keep tests fast (in-memory DB, no network calls).

**Non-Goals:**
- End-to-end LLM integration tests (no real API calls to Anthropic/OpenAI).
- Frontend tests (out of scope).
- Test coverage for scheduler tasks that require full integration (`ai-knowledge-compile`, `ai-source-extract` with external parsers) — these are mocked at service boundaries.
- Changing production behavior purely to make testing easier (we mock at natural interface boundaries).

## Decisions

### 1. Test DB Pattern: Real SQLite + Real Repos
We will use `gorm.Open(sqlite.Open("file:<name>?mode=memory&cache=shared"))` and migrate all AI models. Services will be constructed with real repo instances, just like `license/testutil_test.go` and `itsm/test_helpers_test.go`.

**Rationale:** Fast, deterministic, and verifies real GORM queries. The AI App already uses simple SQL (no raw ClickHouse/FalkorDB in the service layer for most operations).

### 2. FalkorDB / Graph Repo: Mock at Service Boundary
`KnowledgeBaseService.Delete` calls `graphRepo.DeleteGraph(id)`. We will not run FalkorDB in tests. Instead, we inject a `KnowledgeGraphRepo` interface into the service (or construct a thin mock adapter in tests).

**Rationale:** FalkorDB requires a running server. The service logic we want to test is the orchestration (delete sources → delete graph → delete base), not the Cypher query itself.

### 3. LLM Client Mock: Shared `llm_mock_test.go`
`llm.Client` is already an interface. We will create a reusable `mockLLMClient` in `internal/app/ai/llm_mock_test.go` that can be programmed with a sequence of `llm.ChatEvent`s.

**Rationale:** Enables deterministic testing of `ReactExecutor`, `PlanAndExecuteExecutor`, and `AgentGateway.buildLLMClient` via adapter tests without network calls.

### 4. Tool Executor Mock: Inline Structs
`ToolExecutor` is already an interface (`ExecuteTool`). Tests will use inline `struct{ ... }` implementing the interface.

**Rationale:** Simple enough not to need a shared mock file.

### 5. No IOC Container in Tests
Tests construct services directly (e.g., `&AgentService{repo: agentRepo}`) rather than using `samber/do`.

**Rationale:** Faster, clearer dependencies, consistent with existing kernel/app tests. The ITSM app also constructs services directly in tests.

### 6. Crypto in Tests: Use a Static Test Key
Provider, MCPServer, and Skill services use `crypto.EncryptionKey`. Tests will use a static 32-byte test key (e.g., `make([]byte, 32)` filled with a fixed seed) so encryption/decryption is deterministic.

**Rationale:** Avoids needing the real container-secret derivation. Tests remain hermetic.

## Risks / Trade-offs

| Risk | Mitigation |
|------|------------|
| `AgentGateway.Run` spawns goroutines + `io.Pipe`; testing is tricky | Split tests: (a) `selectExecutor` / `buildToolDefinitions` pure logic, (b) full flow with a fake executor and small timeout |
| Adding many tests increases CI time | Tests are in-memory and parallel (`t.Parallel()` where safe); total added time should be < 10s |
| Mocking LLM might miss real protocol edge cases | Separate integration test file (`llm_integration_test.go`) already exists for real providers; we are not removing it |
| Knowledge compile/extract services depend on scheduler tasks | We test the service methods that enqueue tasks or parse inputs, not the full async pipeline |

## Migration Plan

No migration needed — this is a pure test addition. Steps:
1. Merge shared test infrastructure (`testutil_test.go`, `llm_mock_test.go`).
2. Merge TDD files per module in batches (Provider → Tool → Agent → Gateway → Knowledge).
3. Run `go test ./internal/app/ai/...` in CI to ensure all new tests pass.

## Open Questions

- Should we add handler-level tests in addition to service tests? *Decision: service-layer first; handler tests can be a fast-follow if time permits.*
- Should `AgentGateway` be refactored to accept interfaces for `agentRepo`, `modelRepo`, `providerRepo` to ease mocking? *Decision: no — we can construct gateway with real repos + mocked LLM/tool executor, which covers the critical paths.*

## 1. Shared Test Infrastructure

- [x] 1.1 Create `internal/app/ai/testutil_test.go` with `setupTestDB(t)` using in-memory SQLite and `gorm.AutoMigrate` for all AI models.
- [x] 1.2 Add `newTestEncryptionKey(t)` helper returning a deterministic 32-byte key for crypto operations in tests.
- [x] 1.3 Create `internal/app/ai/llm_mock_test.go` with `mockLLMClient` implementing `llm.Client` for programmable event sequences.
- [x] 1.4 Add `mockToolExecutor` struct in `internal/app/ai/tool_executor_mock_test.go` (or inline) for deterministic tool call results.
- [x] 1.5 Run `go test ./internal/app/ai/...` to confirm the test harness compiles and passes empty.

## 2. Provider Service TDD

- [x] 2.1 Create `internal/app/ai/provider_service_test.go` with `newProviderServiceForTest` using real `ProviderRepo` + test encryption key.
- [x] 2.2 Implement `TestProviderService_Create_OpenAI` to verify protocol inference and API key encryption.
- [x] 2.3 Implement `TestProviderService_Create_Anthropic` to verify protocol="anthropic".
- [x] 2.4 Implement `TestProviderService_Update_PreservesKey` to verify empty API key does not overwrite existing encrypted key.
- [x] 2.5 Implement `TestProviderService_Update_ReencryptsKey` to verify non-empty API key triggers re-encryption.
- [x] 2.6 Implement `TestProviderService_Delete_CascadesModels` to verify provider deletion removes associated models.
- [x] 2.7 Implement `TestProviderService_MaskAPIKey` for long and short key masking scenarios.

## 3. Model Service TDD

- [x] 3.1 Create `internal/app/ai/model_service_test.go` with `newModelServiceForTest` using real `ModelRepo` and seeded provider.
- [x] 3.2 Implement `TestModelService_Create` to verify model creation linked to a provider.
- [x] 3.3 Implement `TestModelService_Update` to verify field updates.
- [x] 3.4 Implement `TestModelService_Delete` to verify model removal.
- [x] 3.5 Implement `TestModelService_SetDefault_SwitchesDefault` to verify only one LLM is default at a time.
- [x] 3.6 Implement `TestModelService_SyncModels_Anthropic` to verify preset model insertion.
- [x] 3.7 Implement `TestModelService_SyncModels_Idempotent` to verify re-sync does not duplicate.

## 4. Tool Registry TDD

- [x] 4.1 Create `internal/app/ai/tool_service_test.go` with `newToolServiceForTest`.
- [x] 4.2 Implement `TestToolService_List` and `TestToolService_ToggleActive`.
- [x] 4.3 Create `internal/app/ai/mcp_server_service_test.go` with `newMCPServerServiceForTest`.
- [x] 4.4 Implement `TestMCPServerService_Create_SSERequiresURL` and `TestMCPServerService_Create_STDIRequiresCommand`.
- [x] 4.5 Implement `TestMCPServerService_Create_ValidSSE` and `TestMCPServerService_Create_EncryptsAuthConfig`.
- [x] 4.6 Implement `TestMCPServerService_MaskAuthConfig` for JSON value masking.
- [x] 4.7 Create `internal/app/ai/skill_service_test.go` with `newSkillServiceForTest`.
- [x] 4.8 Implement `TestSkillService_ImportGitHub` and `TestSkillResponse_ToolCount`.

## 5. Agent Service TDD

- [x] 5.1 Create `internal/app/ai/agent_service_test.go` with `newAgentServiceForTest`.
- [x] 5.2 Implement `TestAgentService_Create_InvalidType`, `TestAgentService_Create_AssistantWithoutModel`, `TestAgentService_Create_CodingWithoutRuntime`, `TestAgentService_Create_RemoteWithoutNode`, `TestAgentService_Create_InternalWithoutCode`.
- [x] 5.3 Implement `TestAgentService_Create_NameConflict` and `TestAgentService_Create_CodeConflict`.
- [x] 5.4 Implement `TestAgentService_Update` to verify field updates and default strategy.
- [x] 5.5 Implement `TestAgentService_Delete_WithRunningSessions` and `TestAgentService_Delete_Success`.
- [x] 5.6 Implement `TestAgentService_UpdateBindings_And_GetBindings` to verify atomic replacement of tool/MCP/skill/KB bindings.
- [x] 5.7 Implement `TestAgentService_ListTemplates`.

## 6. Session Service TDD

- [x] 6.1 Create `internal/app/ai/session_service_test.go` with `newSessionServiceForTest`.
- [x] 6.2 Implement `TestSessionService_Create` to verify session creation and title auto-generation from first user message.
- [x] 6.3 Implement `TestSessionService_GetMessages` and `TestSessionService_StoreMessage`.
- [x] 6.4 Implement `TestSessionService_EditMessage` to verify content update and truncation of later messages.
- [x] 6.5 Implement `TestSessionService_UpdateStatus`.

## 7. Gateway & Executor TDD

- [x] 7.1 Create `internal/app/ai/executor_react_test.go`.
- [x] 7.2 Implement `TestReactExecutor_DirectContent` to verify `LLMStart`, `ContentDelta`, `Done` event flow.
- [x] 7.3 Implement `TestReactExecutor_ToolCallRoundTrip` to verify two-turn execution with `ToolCall` and `ToolResult` events.
- [x] 7.4 Implement `TestReactExecutor_MaxTurnsExceeded`.
- [x] 7.5 Implement `TestReactExecutor_Cancelled`.
- [x] 7.6 Create `internal/app/ai/gateway_test.go` with `newGatewayForTest` using real repos + mocked LLM/tool executor.
- [x] 7.7 Implement `TestGateway_BuildToolDefinitions_FiltersInactive` and `TestGateway_BuildToolDefinitions_IncludesMCP`.
- [x] 7.8 Implement `TestGateway_SystemPromptAssembly` to verify concatenation of system prompt, instructions, and memories.
- [x] 7.9 Implement `TestGateway_Run_CompletesSession`, `TestGateway_Run_ErrorSession`, and `TestGateway_Run_CancelledSession`.

## 8. Knowledge Base TDD

- [x] 8.1 Create `internal/app/ai/knowledge_base_service_test.go` with `newKnowledgeBaseServiceForTest`; inject a stubbed `KnowledgeGraphRepo`.
- [x] 8.2 Implement `TestKnowledgeBaseService_Create` to verify `CompileStatus="idle"`.
- [x] 8.3 Implement `TestKnowledgeBaseService_Get` and `TestKnowledgeBaseService_Get_NotFound`.
- [x] 8.4 Implement `TestKnowledgeBaseService_Update`.
- [x] 8.5 Implement `TestKnowledgeBaseService_Delete_Cascade` to verify sources removal and graph repo invocation.
- [x] 8.6 Create `internal/app/ai/knowledge_source_service_test.go` with `newKnowledgeSourceServiceForTest`.
- [x] 8.7 Implement `TestKnowledgeSourceService_Create_URLWithCrawlSettings`.
- [x] 8.8 Implement `TestKnowledgeSourceService_ListByKB` and `TestKnowledgeSourceService_Delete`.

## 9. Verification

- [x] 9.1 Run `go test ./internal/app/ai/... -v` and fix any failures.
- [x] 9.2 Run `go test ./...` to confirm no regressions across the codebase.
- [x] 9.3 Check test coverage report (`make test-cover` or `go test -cover ./internal/app/ai/...`) and record baseline.

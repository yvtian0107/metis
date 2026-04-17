## Why

The AI App (`internal/app/ai/`) currently has virtually no test coverage — only `data_stream_test.go` exists for the UI message encoder. With complex business logic in Provider/Model/Tool/MCPServer/Skill/Agent/Session/Gateway/Knowledge services, and asynchronous executor runtimes (ReAct, Plan-and-Execute), the module is highly regression-prone. We need a systematic TDD effort to bring the AI management module up to the same standard as `license` and `itsm` apps.

## What Changes

- Add shared test infrastructure for AI App: in-memory SQLite harness, mock helpers for `llm.Client`, and fake tool executors.
- Add service-layer TDD for **Provider** and **Model** management (API key encryption, protocol inference, default model switching, preset model sync).
- Add service-layer TDD for **Tool Registry** (`Tool`, `MCPServer`, `Skill`) including transport validation, auth config encryption, and masking logic.
- Add service-layer TDD for **Agent** lifecycle (type validation, binding management, constraints by agent type).
- Add unit TDD for **Agent Gateway & Executors** using mocked LLM client and tool executor (ReAct loop, tool call round-trips, cancellation, memory extraction events).
- Add service-layer TDD for **Knowledge Base** (CRUD, graph deletion cascade, source validation, extraction status flows).

## Capabilities

### New Capabilities
- `ai-provider-tdd`: Service-layer test requirements for LLM provider management (CRUD, encryption, protocol inference, connection test).
- `ai-model-tdd`: Service-layer test requirements for AI model management (sync, default flag, capabilities).
- `ai-tool-registry-tdd`: Service-layer test requirements for builtin tools, MCP servers, and skills (validation, encryption, masking).
- `ai-agent-tdd`: Service-layer test requirements for agent lifecycle and binding management.
- `ai-agent-gateway-tdd`: Unit test requirements for agent gateway and executors with mocked dependencies.
- `ai-knowledge-tdd`: Service-layer test requirements for knowledge base and source management.

### Modified Capabilities
- *(none — this change adds tests for existing behavior without modifying functional requirements)*

## Impact

- New test files under `internal/app/ai/*_test.go`
- New shared mock file `internal/app/ai/llm_mock_test.go` (optional, for executor/gateway tests)
- No breaking API, model, or frontend changes
- Increases build/test time slightly but significantly improves regression safety

# Tasks

## Group 1: Core Interfaces & Dispatch

- [x] 1. Create `internal/app/ai/tool_executor.go` — define `ToolHandlerRegistry` interface + `CompositeToolExecutor` implementing `ToolExecutor`. Takes list of registries + sessionID + userID. Injects session_id into context via `tools.SessionIDKey`. Routes by `HasTool()` match.
- [x] 2. Add `ToolRegistryProvider` interface to `internal/app/app.go` — single method `GetToolRegistry() any` for cross-App registry discovery

## Group 2: ITSM Operator & StateStore

- [x] 3. Create `internal/app/itsm/tools/operator.go` — concrete `ServiceDeskOperator` implementation with 6 methods: MatchServices (keyword scoring against ServiceDefinition), LoadService (ServiceDef + FormDef fields + Actions + routing_field_hint from workflow_json), CreateTicket (via DB insert), ListMyTickets (query by requester_id), WithdrawTicket (validate requester + cancel), ValidateParticipants (parse workflow_json branches + resolve via engine.ParticipantResolver)
- [x] 4. Create `internal/app/itsm/tools/state_store.go` — `SessionStateStore` implementing `StateStore`, reads/writes `ai_agent_sessions.state` JSON field via raw DB queries

## Group 3: IOC Wiring

- [x] 5. Update `internal/app/itsm/app.go` — register ITSM tools.Registry, Operator, StateStore in Providers(). Implement ToolRegistryProvider on ITSMApp. Expose Registry via `do.ProvideAs` for AI App discovery.
- [x] 6. Update `internal/app/ai/app.go` — register CompositeToolExecutor factory. Collect GeneralToolRegistry + all ToolRegistryProvider registries from IOC.
- [x] 7. Update `internal/app/ai/gateway.go` — replace `TODO: inject real ToolExecutor` with CompositeToolExecutor, pass sessionID and userID from session context

## Group 4: Seed Verification

- [x] 8. Verify and fix `internal/app/itsm/tools/provider.go` SeedAgents — ensure "IT 服务台智能体" binds all 10 ITSM tools + 3 general tools correctly, update seed if any tools are missing or misnamed

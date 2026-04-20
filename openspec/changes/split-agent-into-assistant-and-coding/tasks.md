## 1. Backend Service Layer

- [x] 1.1 Add `EnsureType(agent *Agent, expectedType string) error` method to `AgentService` that returns `ErrAgentNotFound` on type mismatch
- [x] 1.2 Add `GetAccessibleByType(id, userID uint, expectedType string)` convenience method to `AgentService` that chains `GetAccessible` + `EnsureType`
- [x] 1.3 Add `GetOwnedByType(id, userID uint, expectedType string)` convenience method to `AgentService`
- [x] 1.4 Add `ListTemplatesByType(agentType string)` method to `AgentService` / `AgentRepo` that filters templates by type

## 2. Backend Typed Handlers

- [x] 2.1 Create `assistant_agent_handler.go` with `AssistantAgentHandler` struct wrapping `AgentService` + `AgentRepo`, implementing Create/List/Get/Update/Delete/ListTemplates — all forcing `type=assistant`
- [x] 2.2 Create `coding_agent_handler.go` with `CodingAgentHandler` struct wrapping `AgentService` + `AgentRepo`, implementing Create/List/Get/Update/Delete/ListTemplates — all forcing `type=coding`
- [x] 2.3 Register DI providers for `AssistantAgentHandler` and `CodingAgentHandler` in `app.go` Providers()

## 3. Backend Typed Routes

- [x] 3.1 Register `/api/v1/ai/assistant-agents` route group in `app.go` Routes() with all CRUD + templates endpoints
- [x] 3.2 Register `/api/v1/ai/coding-agents` route group in `app.go` Routes() with all CRUD + templates endpoints
- [x] 3.3 Keep existing `/api/v1/ai/agents` routes for internal use (no removal yet)

## 4. Backend Seed Rewrite

- [x] 4.1 Rewrite menu seed: replace single `Agent` menu with `助手智能体` (path `/ai/assistant-agents`, permission `ai:assistant-agent:list`) and `编码智能体` (path `/ai/coding-agents`, permission `ai:coding-agent:list`) under the `智能体` group
- [x] 4.2 Seed button permissions for both new menus: `ai:assistant-agent:create/update/delete` and `ai:coding-agent:create/update/delete`
- [x] 4.3 Seed Casbin API policies for `/api/v1/ai/assistant-agents` and `/api/v1/ai/coding-agents` routes
- [x] 4.4 Seed Casbin menu permissions for new permission keys
- [x] 4.5 Add permission migration logic: detect roles with old `ai:agent:*` permissions, grant equivalent new permissions, then remove old permissions
- [x] 4.6 Soft-delete old `ai:agent:list` menu entry
- [x] 4.7 Expand agent templates: add per-runtime coding templates (Claude Code, OpenCode, Codex, Aider) and additional assistant templates (Explore, Ops, Support)

## 5. Frontend API Layer

- [ ] 5.1 Define `AssistantAgentInfo` and `CodingAgentInfo` TypeScript interfaces (extending shared `AgentBase`) in `web/src/lib/api.ts`
- [ ] 5.2 Create `assistantAgentApi` object pointing to `/api/v1/ai/assistant-agents` with list/get/create/update/delete/templates methods
- [ ] 5.3 Create `codingAgentApi` object pointing to `/api/v1/ai/coding-agents` with list/get/create/update/delete/templates methods

## 6. Frontend Shared Components

- [ ] 6.1 Create `web/src/apps/ai/pages/_shared/agent-list-page.tsx` — generic agent card grid component accepting agentType, title, createPath, queryKey, permissions config
- [ ] 6.2 Create `web/src/apps/ai/pages/_shared/agent-detail-page.tsx` — generic agent detail component accepting agentType, basePath, permissions config
- [ ] 6.3 Create `web/src/apps/ai/pages/_shared/agent-form-common.tsx` — shared form fields (name, description, visibility, instructions)
- [ ] 6.4 Move `binding-checkbox-list.tsx` to `web/src/apps/ai/pages/_shared/`

## 7. Frontend Assistant Agent Pages

- [ ] 7.1 Create `web/src/apps/ai/pages/assistant-agents/index.tsx` — thin wrapper passing assistant config to shared list page
- [ ] 7.2 Create `web/src/apps/ai/pages/assistant-agents/create.tsx` — creation page composing common fields + assistant-specific fields (provider, model, strategy, temperature, max tokens, max turns, system prompt, tool/skill/mcp/kb bindings)
- [ ] 7.3 Create `web/src/apps/ai/pages/assistant-agents/[id].tsx` — detail page using shared component with assistant config sections
- [ ] 7.4 Create `web/src/apps/ai/pages/assistant-agents/[id]/edit.tsx` — edit page composing same fields as create, pre-filled

## 8. Frontend Coding Agent Pages

- [ ] 8.1 Create `web/src/apps/ai/pages/coding-agents/index.tsx` — thin wrapper passing coding config to shared list page
- [ ] 8.2 Create `web/src/apps/ai/pages/coding-agents/create.tsx` — creation page composing common fields + coding-specific fields (runtime, exec mode, workspace, node, mcp/skill bindings)
- [ ] 8.3 Create `web/src/apps/ai/pages/coding-agents/[id].tsx` — detail page using shared component with coding config sections
- [ ] 8.4 Create `web/src/apps/ai/pages/coding-agents/[id]/edit.tsx` — edit page composing same fields as create, pre-filled

## 9. Frontend Navigation and Routes

- [ ] 9.1 Update `web/src/apps/ai/module.ts` navigation: replace single `agents` group item with `assistantAgents` and `codingAgents` items, each with its own permission
- [ ] 9.2 Update `web/src/apps/ai/module.ts` routes: add `ai/assistant-agents` and `ai/coding-agents` route trees, remove old `ai/agents` routes
- [ ] 9.3 Add locale entries for `assistantAgents` and `codingAgents` in both `zh-CN.json` and `en.json` (titles, create/edit labels, empty states, menu labels)
- [ ] 9.4 Add permission label mappings in locale `menuPermissions` section for new permission keys

## 10. Cleanup

- [ ] 10.1 Delete old `web/src/apps/ai/pages/agents/` directory (all files)
- [ ] 10.2 Remove old `agentApi` from `web/src/lib/api.ts` (keep `AgentInfo` type if still used by session/chat pages, otherwise remove)
- [ ] 10.3 Verify chat page (`/ai/chat/:sid`) still works — it uses `sessionApi` which is unchanged
- [ ] 10.4 Run `cd web && bun run lint` and `cd web && bun run build` to verify frontend builds cleanly
- [ ] 10.5 Run `go build -tags dev ./cmd/server` to verify backend compiles

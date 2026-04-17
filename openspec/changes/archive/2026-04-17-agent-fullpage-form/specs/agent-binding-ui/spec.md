# Agent Binding UI

## Purpose

Checkbox list selectors for binding Tools, MCP Servers, Skills, and Knowledge Bases to agents.

## Requirements

### BindingCheckboxList Component
- Reusable component rendering a bordered section with title and scrollable checkbox list
- Props: title, API endpoint, selected IDs array, onChange callback, label field name
- Fetches available items from API via useQuery (pageSize=100)
- Each item: checkbox + name/displayName + optional description as muted text
- Max height with `overflow-y-auto` for long lists
- Loading state while fetching items
- Empty state when no items exist

### Integration in AgentForm
- Tool Bindings card section contains 4 `BindingCheckboxList` instances in a 2x2 responsive grid
- Binding endpoints:
  - Tools: `GET /api/v1/ai/tools?pageSize=100` — label: `displayName || name`
  - MCP Servers: `GET /api/v1/ai/mcp-servers?pageSize=100` — label: `name`
  - Skills: `GET /api/v1/ai/skills?pageSize=100` — label: `displayName || name`
  - Knowledge Bases: `GET /api/v1/ai/knowledge-bases?pageSize=100` — label: `name`
- Selected values stored as `toolIds: number[]`, `skillIds: number[]`, `mcpServerIds: number[]`, `knowledgeBaseIds: number[]`
- On form submit, these arrays are sent alongside agent fields to the API

### Detail Page Binding Display
- Configuration tab shows binding names (not IDs) in a readonly format
- Uses the same 4-section layout but with Badge components instead of checkboxes
- Fetches full item lists to resolve IDs to names
- Empty sections show dash or "None" text

### Query Keys
- `["ai-binding-tools"]` for tools list
- `["ai-binding-mcp-servers"]` for MCP servers list
- `["ai-binding-skills"]` for skills list
- `["ai-binding-knowledge-bases"]` for knowledge bases list

# Agent Form Page

## Purpose

Full-page create/edit form for AI agents, replacing the current Sheet (drawer) approach.

## Requirements

### Routes
- `GET /ai/agents/create` renders the form in create mode
- `GET /ai/agents/:id/edit` renders the form in edit mode with pre-populated data

### Form Sections (Card layout, single scrollable page)

1. **Basic Info** — name (required), type (assistant/coding), visibility (private/team/public), description
2. **Model Config** — visible only when type=assistant. Provider→Model cascade, strategy, temperature slider, maxTokens, maxTurns
3. **Runtime Config** — visible only when type=coding. Runtime, execMode, workspace, nodeId (when remote)
4. **Tool Bindings** — 4 checkbox lists in 2x2 grid: Tools, MCP Servers, Skills, Knowledge Bases
5. **Prompts** — systemPrompt (monospace textarea), instructions

### Behavior
- Create mode: empty form, submit calls `POST /api/v1/ai/agents`, navigate to detail on success
- Edit mode: fetch `AgentWithBindings`, populate form including binding IDs as checked items. Resolve providerId from modelId via model detail API. Submit calls `PUT /api/v1/ai/agents/:id`, navigate to detail on success
- Type switch: show/hide model config vs runtime config cards. Do not reset values
- Provider→Model cascade: selecting provider clears model selection, filters model dropdown by providerId. Same pattern as knowledge-base-form.tsx
- Page header: back arrow (navigate to list), title ("New Agent" / "Edit Agent"), save + cancel buttons

### Validation (Zod schema)
- name: required, max 128 chars
- type: required, enum
- visibility: required, enum
- modelId: required when type=assistant
- runtime: required when type=coding
- temperature: 0-2 range
- maxTokens: positive integer
- maxTurns: 1-100

### Shared Component
- `AgentForm` component accepts optional `agent?: AgentWithBindings` prop
- Create page: `<AgentForm />`
- Edit page: fetches agent data, shows loading state, then `<AgentForm agent={data} />`

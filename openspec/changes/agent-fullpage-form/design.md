## Overview

Replace the agent Sheet (drawer) with full-page create/edit forms using Card sections. Add tool binding checkbox lists. Redesign the detail page to merge overview + bindings.

## Architecture

### Route Structure

```
/ai/agents              → list page (existing, modified)
/ai/agents/create       → full-page create form (NEW)
/ai/agents/:id          → detail page (existing, redesigned)
/ai/agents/:id/edit     → full-page edit form (NEW)
```

### Component Hierarchy

```
agents/
├── index.tsx                    ← list page (remove Sheet, navigate to /create or /:id/edit)
├── create.tsx                   ← NEW: thin wrapper, renders <AgentForm />
├── [id].tsx                     ← redesigned: overview+bindings merged, edit navigates away
├── [id]/
│   └── edit.tsx                 ← NEW: fetches agent, renders <AgentForm agent={data} />
└── components/
    ├── agent-form.tsx           ← NEW: shared form with Card sections
    ├── binding-checkbox-list.tsx ← NEW: reusable checkbox list for bindings
    └── agent-sheet.tsx          ← DELETED
```

### AgentForm Component

Shared between create and edit. Props:

```typescript
interface AgentFormProps {
  agent?: AgentWithBindings  // undefined = create mode, present = edit mode
}
```

#### Card Sections Layout

```
┌─ Card: 基础信息 ────────────────────────────────────────────┐
│  Name (input)    Type (select: assistant/coding)             │
│  Visibility (select: private/team/public)                    │
│  Description (textarea, 2 rows)                              │
└──────────────────────────────────────────────────────────────┘

┌─ Card: 模型配置 (type=assistant only) ──────────────────────┐
│  Provider ▼  →  Model ▼  (cascade, same as knowledge-base)  │
│  Strategy (select: react / plan_and_execute)                 │
│  Temperature (slider 0-2, step 0.1)                          │
│  MaxTokens (number input)    MaxTurns (number input)         │
└──────────────────────────────────────────────────────────────┘

┌─ Card: 运行时配置 (type=coding only) ──────────────────────┐
│  Runtime ▼  ExecMode ▼  Workspace (input)  Node ▼ (remote)  │
└──────────────────────────────────────────────────────────────┘

┌─ Card: 工具绑定 ────────────────────────────────────────────┐
│  2x2 grid of BindingCheckboxList components:                 │
│  ┌─ Tools ──────────────┐  ┌─ MCP Servers ────────────┐    │
│  │ ☑ search_knowledge   │  │ ☐ GitHub MCP             │    │
│  │ ☑ read_document      │  │                           │    │
│  └──────────────────────┘  └───────────────────────────┘    │
│  ┌─ Skills ─────────────┐  ┌─ Knowledge Bases ────────┐    │
│  │ ☐ git-operations     │  │ ☐ 运维知识库              │    │
│  └──────────────────────┘  └───────────────────────────┘    │
└──────────────────────────────────────────────────────────────┘

┌─ Card: 提示词 ──────────────────────────────────────────────┐
│  System Prompt (textarea, 6 rows, monospace)                 │
│  Instructions (textarea, 4 rows)                             │
└──────────────────────────────────────────────────────────────┘
```

#### Type Switching

When user changes type between `assistant` and `coding`:
- Show/hide the relevant config card (model config vs runtime config)
- Do NOT reset field values — preserve them in case user switches back

### BindingCheckboxList Component

Reusable component for each binding type:

```typescript
interface BindingCheckboxListProps {
  title: string
  queryKey: string[]
  endpoint: string          // e.g. "/api/v1/ai/tools?pageSize=100"
  value: number[]           // selected IDs
  onChange: (ids: number[]) => void
  labelField: string        // "name" or "displayName"
  descField?: string        // optional description field
}
```

Renders a bordered box with title header, scrollable list of checkboxes (max-height with overflow-y-auto). Each item shows checkbox + name + optional description. Fetches items from API via useQuery.

### Detail Page Redesign

Tabs reduced from 3 to 2:

| Before | After |
|--------|-------|
| Overview / Bindings / Sessions | Configuration / Sessions |

**Configuration tab**: Readonly display of all agent fields + binding names. Uses the same Card section visual grouping as the form but with static text instead of inputs. Bindings show item names (fetched via API) instead of raw IDs.

**Header actions**:
- "编辑" button → `navigate(\`/ai/agents/${id}/edit\`)`
- "对话" button → create session + navigate to chat
- "删除" button → confirmation dialog, then navigate back to list

### Data Flow

```
Create: /ai/agents/create
  → AgentForm (no agent prop)
  → form.submit → POST /api/v1/ai/agents (with toolIds, skillIds, etc.)
  → navigate to /ai/agents/:id

Edit: /ai/agents/:id/edit
  → useQuery ["ai-agent", id] → AgentWithBindings
  → AgentForm (agent prop = fetched data)
  → Provider resolution: fetch model detail to resolve providerId (same pattern as knowledge-base-form)
  → form.submit → PUT /api/v1/ai/agents/:id (with toolIds, skillIds, etc.)
  → navigate to /ai/agents/:id
```

### Query Keys (avoiding collisions)

All binding list queries use unique keys:
- `["ai-tools-list"]` — tools for checkbox
- `["ai-mcp-servers-list"]` — MCP servers for checkbox
- `["ai-skills-list"]` — skills for checkbox
- `["ai-knowledge-bases-list"]` — KBs for checkbox

These are distinct from existing keys like `["ai-providers"]`.

## Decisions

1. **Create and edit are separate routes** sharing the same `AgentForm` component — not inline edit on detail page
2. **Checkbox list** (not transfer/shuttle) for bindings — simpler, sufficient for typical list sizes (<50 items)
3. **Card sections** on a single scrollable page — not tabs within the form
4. **Detail page keeps tabs** but only 2: Configuration (merged overview+bindings) and Sessions
5. **No breaking API changes** — frontend-only change, backend already supports all binding fields

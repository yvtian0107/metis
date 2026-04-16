## Why

The Agent create/edit form currently uses a Sheet (drawer), which is too cramped for the growing number of fields. More critically, the UI has **no way to bind Tools, MCP Servers, Skills, or Knowledge Bases** to an agent — the backend API supports all four binding types, but the frontend completely ignores them. Without bindings, agents are limited to raw LLM chat with no tool access.

## What Changes

- **Replace Sheet with full-page forms**: New routes `/ai/agents/create` and `/ai/agents/:id/edit` using Card-sectioned scrollable pages
- **Add tool binding UI**: Checkbox list selectors for Tools, MCP Servers, Skills, and Knowledge Bases within the form
- **Delete `agent-sheet.tsx`**: No longer needed — all create/edit flows use the new full-page form component
- **Redesign detail page (`[id].tsx`)**: Merge overview + bindings tabs into a single "Configuration" tab that shows binding names (not just IDs). Edit button navigates to `/ai/agents/:id/edit` instead of opening a Sheet
- **Shared form component**: `AgentForm` component shared between create and edit pages
- **Update list page**: Create/Edit buttons navigate to the new routes instead of opening a Sheet

## Capabilities

### New Capabilities
- `agent-form-page`: Full-page agent create/edit form with Card sections (basic info, model/runtime config, tool bindings, prompts)
- `agent-binding-ui`: Checkbox list selectors for binding Tools, MCP Servers, Skills, and Knowledge Bases to agents

### Modified Capabilities
_(none — no existing spec-level requirements are changing)_

## Impact

- **Frontend files changed**: `agents/index.tsx`, `agents/[id].tsx`, `ai/module.ts`, locale files
- **Frontend files added**: `agents/create.tsx`, `agents/[id]/edit.tsx`, `agents/components/agent-form.tsx`
- **Frontend files deleted**: `agents/components/agent-sheet.tsx`
- **Backend**: No changes needed — API already supports all binding fields
- **Routes**: Two new routes added to AI app module (`ai/agents/create`, `ai/agents/:id/edit`)

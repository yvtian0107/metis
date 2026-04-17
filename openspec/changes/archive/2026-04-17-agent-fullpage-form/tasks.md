# Tasks

## Group 1: Shared Components

- [x] 1. Create `agents/components/binding-checkbox-list.tsx` — reusable BindingCheckboxList component with title, scrollable checkbox list, useQuery data fetching, loading/empty states
- [x] 2. Create `agents/components/agent-form.tsx` — shared AgentForm component with 5 Card sections (basic info, model config, runtime config, tool bindings, prompts). Accept optional `agent?: AgentWithBindings` prop for edit mode. Include provider→model cascade, type-conditional sections, Zod schema validation, binding checkbox lists in 2x2 grid

## Group 2: Create & Edit Pages

- [x] 3. Create `agents/create.tsx` — thin page wrapper rendering `<AgentForm />`, with back arrow header, submit calls POST API, navigate to detail on success
- [x] 4. Create `agents/[id]/edit.tsx` — page that fetches `AgentWithBindings` by id, resolves providerId from modelId, renders `<AgentForm agent={data} />`, submit calls PUT API, navigate to detail on success

## Group 3: Route Registration

- [x] 5. Update `ai/module.ts` — add routes for `ai/agents/create` and `ai/agents/:id/edit` with lazy imports

## Group 4: List & Detail Page Updates

- [x] 6. Update `agents/index.tsx` — remove AgentSheet import/usage, create button navigates to `/ai/agents/create`, edit button navigates to `/ai/agents/:id/edit`
- [x] 7. Redesign `agents/[id].tsx` — merge overview+bindings into "Configuration" tab showing binding names (not IDs), edit button navigates to `/:id/edit`, remove AgentSheet import/usage. Keep Sessions tab unchanged

## Group 5: Cleanup & i18n

- [x] 8. Delete `agents/components/agent-sheet.tsx`
- [x] 9. Update `locales/en.json` and `locales/zh-CN.json` — add i18n keys for binding section titles (tools, mcpServers, skills, knowledgeBases labels), form page titles, empty states

## 1. Extend AgentItem Interface

- [x] 1.1 Update `AgentItem` in `web/src/apps/itsm/api.ts` to include `strategy`, `temperature`, `maxTurns`, `modelId` fields from the existing list API response

## 2. Add i18n Keys

- [x] 2.1 Add Chinese and English locale keys for status labels (`configured`, `unconfigured`, `error`) and agent preview fields (`strategy`, `temperature`, `maxTurns`) in ITSM locales

## 3. Agent Preview & Status Indicator Components

- [x] 3.1 Create `AgentPreview` inline component: renders strategy · temperature · maxTurns in `text-xs text-muted-foreground`, hidden when no agent selected
- [x] 3.2 Create `ConfigStatus` inline component: renders colored dot (green/gray/red) + label based on config health state

## 4. Page Layout Refactor

- [x] 4.1 Refactor `engine-config/index.tsx`: wrap Servicedesk and Decision cards in `grid grid-cols-1 md:grid-cols-2 gap-4` row
- [x] 4.2 Integrate `AgentPreview` into both `AgentField` sections (below the Select, conditionally rendered)
- [x] 4.3 Integrate `ConfigStatus` into all four CardHeader sections (Generator, Servicedesk, Decision, General)
- [x] 4.4 Pass full agent list to AgentField so it can derive preview data and status from the selected agentId

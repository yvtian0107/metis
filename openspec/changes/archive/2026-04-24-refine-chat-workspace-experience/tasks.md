## 1. Design Baseline

- [x] 1.1 Update `DESIGN.md` with Chat Workspace refinement rules for Composer, Workspace, Sidebar, message flow, and ITSM Surface states.
- [x] 1.2 Document that business pages must use shared design semantics instead of naked layout `className` for core chat sizing.

## 2. Shared Component Interfaces

- [x] 2.1 Add semantic Composer props: `variant`, `maxWidth`, `minRows`, `showToolbarHint`, and `attachmentTone`.
- [x] 2.2 Remove current business usage of `compact` and core layout `className` from Composer calls.
- [x] 2.3 Add semantic Workspace props: `density`, `messageWidth`, `composerPlacement`, and `emptyStateTone`.
- [x] 2.4 Add Sidebar visual variants for AI chat and ITSM service desk.

## 3. Experience Refinement

- [x] 3.1 Refine ITSM welcome stage layout, Composer size, quick prompts, and visual gravity.
- [x] 3.2 Refine message flow spacing, streaming state, tool activity, and error recovery.
- [x] 3.3 Refine ITSM `itsm.draft_form` loading, editable, error, and submitted states.
- [x] 3.4 Migrate AI Management and ITSM calls to the new semantic props.

## 4. Verification

- [x] 4.1 Run targeted ESLint for Chat Workspace, AI chat, and ITSM Service Desk.
- [x] 4.2 Run `cd web && bun run build`.
- [x] 4.3 Validate the OpenSpec change.

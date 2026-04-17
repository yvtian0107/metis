## 1. Department Detail Page — Core Structure

- [x] 1.1 Create `web/src/apps/org/pages/departments/[id].tsx` with route parameter parsing, data fetching (`GET /departments/:id` tree node lookup + `GET /departments/:id/positions` + `GET /org/users?departmentId=:id`), and basic page skeleton (back button + title)
- [x] 1.2 Build the info card section: brand-color stripe, avatar (first letter of name), department name, code badge, status dot, description-list grid (manager, parent, member count, created at, description), edit button, and overflow menu (⋯) with delete action
- [x] 1.3 Build the allowed positions section: display current positions as chips from the positions query, "Manage" button opening a Command popover for add/remove, save via `PUT /departments/:id/positions`; hide section from users without `org:department:update`
- [x] 1.4 Build the member list section: table with avatar+name+email, position badges (primary star), assigned date, actions dropdown (edit positions, view org info, remove); pagination and keyword search via `useListPage`; "Add Member" button gated by `org:assignment:create`
- [x] 1.5 Wire AddMemberSheet, EditPositionsSheet, UserOrgSheet, and remove confirmation dialog into the detail page, reusing existing shared components
- [x] 1.6 Build the sub-departments section: list direct children from tree data with name, code, member count, manager; click navigates to child detail page; hide section if no children

## 2. Department List Page — Enhancements

- [x] 2.1 Add `memberCount` column/badge to the tree table rows (data already available from `/departments/tree` response)
- [x] 2.2 Change row click behavior: clicking chevron toggles expand/collapse, clicking rest of row navigates to `/org/departments/:id` via React Router `useNavigate`; add visual navigation indicator (arrow or chevron-right) at row end
- [x] 2.3 Remove inline edit/delete action buttons from tree table rows (moved to detail page); keep only the navigation affordance
- [ ] 2.4 Simplify DepartmentSheet: remove allowed positions multi-select (moved to detail page), keep only basic fields (name, code, parent, manager, description)

## 3. Routing & Menu

- [x] 3.1 Update `web/src/apps/org/module.ts`: add `{ path: "org/departments/:id", lazy: () => import("./pages/departments/[id]") }` route; remove `org/assignments` route; add redirect from `org/assignments` to `org/departments`
- [x] 3.2 Update `internal/app/org/seed.go`: rename "部门管理" menu to "组织架构", remove "人员分配" menu item from seed data
- [x] 3.3 Update i18n locales (`zh-CN.json` and `en.json`): add detail page keys (section titles, action labels), rename page title key

## 4. Cleanup

- [ ] 4.1 Delete `web/src/apps/org/pages/assignments/` directory (index.tsx, department-tree.tsx, member-list.tsx, add-member-sheet.tsx, types.ts)
- [x] 4.2 Move shared types (TreeNode, MemberWithPositions, etc.) from deleted `types.ts` to a shared location (e.g., `web/src/apps/org/types.ts`) before deleting assignments directory
- [ ] 4.3 Verify all existing functionality is covered: member search, pagination, add/edit/remove members, edit positions with primary, view org info, position chips with badges — test against the spec scenarios

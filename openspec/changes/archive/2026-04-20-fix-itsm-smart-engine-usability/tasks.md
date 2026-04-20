## 1. Smart Continuation

- [x] 1.1 Trace the existing smart-progress submission points in engine, approval, and action completion flows and remove any paths that rely on delayed polling-style continuation.
- [x] 1.2 Submit `itsm-smart-progress` immediately after smart human activity completion and ensure duplicate submissions do not create duplicate next-step state.
- [x] 1.3 Submit `itsm-smart-progress` immediately after smart action completion and pass through the completed action context needed for the next decision cycle.
- [x] 1.4 Submit `itsm-smart-progress` immediately after AI `pending_approval` confirm/reject and ensure reject reason is available to the next decision cycle.
- [x] 1.5 Update SmartEngine decision execution so single-mode plans create only the current activity and preserve the agentic-with-hints decision model.

## 2. Approval Ownership And Authorization

- [x] 2.1 Align the backend approval count/list query logic so AI `pending_approval` items are returned only for authorized actionable users.
- [x] 2.2 Add server-side permission checks for AI decision confirm and reject endpoints and return the correct forbidden/invalid-state responses.
- [x] 2.3 Update the ticket detail smart activity card so AI confirm/reject controls render only for authorized users.
- [x] 2.4 Update the "我的审批" page and badge behavior so AI pending approvals match backend ownership exactly.

## 3. Seed Alignment

- [x] 3.1 Audit the 5 built-in smart service definitions and align every referenced participant position code with the built-in Org seed set.
- [x] 3.2 Update Org built-in position seeding so every participant code used by built-in smart services exists on fresh install.
- [x] 3.3 Update install-time admin bootstrap so the default admin receives the built-in org assignment needed for immediate smart-flow verification without affecting later sync behavior.

## 4. Verification

- [x] 4.1 Add or update backend tests/BDD scenarios for near-real-time continuation after approval completion, action completion, and AI decision confirm/reject.
- [x] 4.2 Add or update tests for AI pending approval visibility, approval counts, and unauthorized confirm/reject attempts.
- [x] 4.3 Add or update fresh-install/seed verification covering built-in smart services, participant validation, and admin default identity.

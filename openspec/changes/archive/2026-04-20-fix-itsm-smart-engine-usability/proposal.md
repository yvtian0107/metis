## Why

ITSM SmartEngine currently has several usability failures that break its intended operating model: decision continuation is not triggered at key event boundaries, AI approval work can be hidden from the correct approvers, and the seeded org/participant defaults do not match the built-in smart workflows. These issues make the engine look flaky in manual testing and, more importantly, risk degrading an event-driven agentic flow into delayed or stalled workflow progression.

## What Changes

- Make SmartEngine continuation event-driven at the actual completion boundaries of smart activities so approval completion, action completion, and AI decision confirm/reject each trigger the next decision run near real time.
- Correct approval ownership and authorization for AI `pending_approval` activities so "我的审批" shows the right records and only authorized users can confirm or reject AI decisions.
- Align built-in smart workflow participant seeds, org built-in positions, and install-time admin defaults so seeded demo/test data can satisfy participant validation and reflect real routing behavior.
- Clarify the SmartEngine decision trigger contract and decision-mode positioning so the engine remains agentic with workflow hints, rather than becoming a rule-first flow executor.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `itsm-smart-engine`: define the event-driven continuation triggers, near-real-time smart progress handoff, and the decision-mode contract that keeps workflow structure as hints to an agentic engine.
- `itsm-approval-api`: correct approval visibility and authorization rules for AI `pending_approval` activities, including confirmation and rejection paths.
- `itsm-approval-ui`: align the "我的审批" page with AI pending approval ownership so only actionable AI confirmations are listed and counted for the current user.
- `itsm-smart-ticket-detail`: align ticket-detail AI confirmation controls with backend ownership and authorization checks.
- `itsm-service-catalog`: align the built-in smart service/workflow seeds with resolvable participant identities so built-in intelligent flows pass participant validation on a fresh install.
- `org-builtin-seed`: align built-in position seeds with the participant codes used by the smart ITSM workflows.
- `seed-init`: define the install-time default admin organization identity required for fresh installs to exercise the built-in smart approval paths realistically.

## Impact

- **Backend**: Smart engine progress/continuation flow, approval handlers and queries, seed/install logic for ITSM and Org apps.
- **Frontend**: ITSM ticket detail and approval list views for AI `pending_approval` ownership and action controls.
- **APIs**: No new endpoints required, but existing approval/AI decision endpoints gain tighter authorization behavior.
- **Database**: No schema change expected; seed content and install-time default assignments change.
- **Testing**: BDD and fresh-install scenarios need coverage for event-driven continuation, approval visibility, and seeded participant reachability.

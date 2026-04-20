## Context

The current SmartEngine behavior is functionally close to the intended product model, but several usability failures break the operator experience and the architectural direction.

First, continuation is not consistently triggered at the event boundaries that actually advance smart workflows. Some paths rely on delayed polling-style progress tasks or stop after a human/AI/action step changes state, which makes the engine feel stalled and non-deterministic.

Second, AI `pending_approval` activities are crossing two concerns at once: they are workflow records in the backend, but they are rendered both in ticket detail and in the dedicated "我的审批" list. Ownership and authorization are not aligned tightly enough, so list filtering and confirm/reject controls can drift from the actual assignee/approver model.

Third, the seeded smart services, built-in org positions, and install-time admin identity are not fully aligned. That creates false negatives in `validate_participants`, makes built-in smart demos fail on fresh installs, and produces misleading manual test results.

The change is cross-cutting because it touches engine progression semantics, approval queries and permissions, frontend approval views, and installation/seed defaults.

## Goals / Non-Goals

**Goals:**
- Restore near-real-time continuation for SmartEngine after approval completion, action completion, and AI decision confirmation or rejection.
- Keep SmartEngine event-driven and agentic, with workflow structure acting as hints rather than as a rule-first executor.
- Make AI pending approvals visible only to the correct users in both approval counts and approval lists.
- Enforce server-side authorization for AI decision confirm/reject operations, regardless of frontend visibility.
- Align built-in smart service definitions, org built-in positions, and install-time admin org identity so fresh installs can exercise the seeded smart flows successfully.

**Non-Goals:**
- Rewriting SmartEngine into a new runtime or replacing the existing decision executor architecture.
- Adding new external APIs or changing the database schema.
- Redesigning the overall ITSM approval UX beyond the ownership and authorization fixes needed for correctness.
- Changing seeded services beyond the participant/identity alignment required to make existing built-in smart flows reachable and testable.

## Decisions

### 1. Smart continuation will be triggered from state-transition boundaries, not by generic polling

Continuation will be scheduled immediately after the state transitions that logically unblock the next decision cycle:
- when a smart approval activity is completed
- when a smart action activity finishes
- when an AI `pending_approval` activity is confirmed
- when an AI `pending_approval` activity is rejected

This keeps the system near real time without introducing synchronous recursive progression inside handlers. The continuation mechanism remains task-based for isolation and retry safety, but the submission point moves to the event boundary rather than depending on later background scans.

Alternative considered: run the next decision cycle inline inside each handler. Rejected because it couples HTTP latency to LLM/runtime execution, increases lock contention risk, and makes retries less controlled.

Alternative considered: keep a generic periodic smart-progress loop. Rejected because it blurs trigger ownership and encourages polling-style progression that contradicts the intended event-driven model.

### 2. Smart-progress submission will be idempotent at the ticket/activity boundary

Multiple completion events can race, especially around parallel approvals and action completion callbacks. The progress scheduling path should therefore be safe to trigger more than once for the same effective continuation point, with the engine re-checking current ticket/activity state before starting a new decision run.

Alternative considered: rely only on row locks and assume handlers never double-submit. Rejected because concurrent completion paths are expected in real workflows, and duplicate scheduling is easier to tolerate than missing continuation.

### 3. AI pending approval ownership will follow the same actionable-user model as other approvals

The approval list/count queries and the confirm/reject endpoints will all use the same backend ownership rule: an AI `pending_approval` item is actionable only for the user(s) actually assigned/authorized for that activity. Frontend filtering remains a convenience layer; backend authorization is the source of truth.

Alternative considered: allow any privileged admin to confirm/reject AI decisions. Rejected because it weakens approval accountability and makes "我的审批" semantics inconsistent.

### 4. Decision mode stays “agentic with hints”

The workflow JSON or collaboration spec remains guidance injected into the decision context. It can bias the agent toward plausible routes or participants, but it must not become a rule-first deterministic flow executor except where an explicit smart activity completion already changed state. This preserves the product positioning: the SmartEngine is an agentic engine informed by workflow hints, not a classic engine with LLM decoration.

Alternative considered: translate every decision trigger into a workflow-node transition table first, then ask AI only for missing values. Rejected because that would regress the engine toward a classic rule engine and undermine the stated agentic model.

### 5. Fresh-install usability will be fixed by seed alignment, not by weakening participant validation

The built-in smart services, built-in org positions, and default admin org identity will be made mutually consistent so `validate_participants` succeeds on a realistic fresh install. The validation rules themselves stay strict; the seed data becomes correct.

Alternative considered: loosen `validate_participants` for built-in smart services or install mode. Rejected because it would hide real participant resolution failures and reduce test signal quality.

## Risks / Trade-offs

- [Duplicate continuation submissions from concurrent completions] → Keep smart progress scheduling idempotent and re-check ticket/activity state before running a decision cycle.
- [Authorization logic diverges between list/count/detail/handlers] → Centralize the actionable-user rule for AI pending approvals and cover it with BDD/API scenarios.
- [Seed alignment may conflict with previously documented position codes] → Update the spec contracts together so Org built-ins and built-in smart service definitions reference the same codes.
- [Near-real-time continuation can increase task volume] → Preserve asynchronous execution and rely on existing task retry/backoff instead of inline decision runs.
- [Changing install-time admin identity may surprise existing environments] → Scope the default identity requirement to fresh install behavior and keep sync idempotent/non-destructive.

## Migration Plan

1. Update specs for SmartEngine continuation, approval ownership/authorization, approval UI visibility, service seed alignment, org built-in positions, and install-time admin identity.
2. Implement backend continuation trigger changes first so the engine regains deterministic progression.
3. Implement authorization/query fixes for AI pending approvals and update frontend approval surfaces to match.
4. Update fresh-install seed behavior for org positions and admin assignment, plus built-in smart service participant references.
5. Verify with targeted BDD scenarios covering fresh install, "我的审批", AI confirm/reject, and end-to-end smart continuation after approval/action events.

Rollback is low risk because there is no schema change. If needed, the code can revert to prior continuation points and prior seed defaults without data migration.

## Open Questions

- None blocking for proposal/design purposes. The exact helper/function boundaries can be chosen during implementation as long as the trigger ownership and authorization semantics remain as specified.

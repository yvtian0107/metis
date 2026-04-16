## Context

Announcement management is implemented inside `NotificationService` because announcements are modeled as broadcast notifications (`type = "announcement"`). The repository already supports listing with `users` JOIN, CRUD operations, and cascading delete of read records. The missing piece is service-layer test coverage to verify these behaviors.

## Goals / Non-Goals

**Goals:**
- Add `internal/service/notification_test.go` with TDD-style tests for all announcement service methods.
- Follow the existing kernel test pattern: in-memory SQLite, real repository, `samber/do` injector.

**Non-Goals:**
- No new service or handler abstractions — `NotificationService` already exists.
- No frontend, API route, or model changes.

## Decisions

### Reuse `NotificationService` vs. split `AnnouncementService`
**Decision:** Keep tests in `notification_test.go` targeting `NotificationService` methods.
**Rationale:** The announcement feature is intentionally a specialization of the notification system. Splitting it now would create an artificial seam with no business value.

### Test DB tables
**Decision:** Migrate `User`, `Notification`, and `NotificationRead`.
**Rationale:** `ListAnnouncements` JOINs `users`, and `DeleteAnnouncement` cascades to `notification_reads`. All three tables must be present for complete testing.

### Assertion style
**Decision:** Use the same direct `t.Fatalf` style as `user_test.go`, `settings_test.go`, etc.
**Rationale:** Consistency with the existing kernel test suite.

## Risks / Trade-offs

- **[Risk] Low-value refactoring temptation** → We are intentionally NOT extracting an `AnnouncementService`. The tests verify the existing abstraction boundary.

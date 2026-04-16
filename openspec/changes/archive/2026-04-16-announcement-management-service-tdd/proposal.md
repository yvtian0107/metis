## Why

The announcement management module is the last kernel feature without dedicated service-layer TDD coverage. While `NotificationService` already implements announcement CRUD, there are no automated tests verifying `ListAnnouncements`, `CreateAnnouncement`, `UpdateAnnouncement`, or `DeleteAnnouncement`. Adding comprehensive TDD-style tests will complete the kernel service test matrix and protect against regressions.

## What Changes

- Create `internal/service/notification_test.go` with TDD-style tests for announcement operations using in-memory SQLite.
- Test `ListAnnouncements` pagination, keyword filtering, and `CreatorUsername` resolution via `users` join.
- Test `CreateAnnouncement` sets correct `type`, `source`, `target_type`, and `created_by`.
- Test `UpdateAnnouncement` persists changes and returns `ErrNotificationNotFound` for missing IDs.
- Test `DeleteAnnouncement` removes the record and associated read tracking, returning `ErrNotificationNotFound` for missing IDs.

## Capabilities

### New Capabilities
- `announcement-management-service-test`: Service-layer test coverage for announcement listing, creation, update, and deletion.

### Modified Capabilities
- (none)

## Impact

- `internal/service/notification_test.go` (new)
- No handler or IOC changes required — `NotificationService` already exists and is wired.

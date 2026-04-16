## 1. Test Infrastructure

- [x] 1.1 Create `internal/service/notification_test.go` with `newTestDBForNotification`, `newNotificationServiceForTest`, and `seedUser`, `seedAnnouncement`, `seedNotificationRead` helpers using in-memory SQLite and real `NotificationRepo`.
- [x] 1.2 Ensure `AutoMigrate` covers `User`, `Notification`, and `NotificationRead`.

## 2. Announcement Listing Tests

- [x] 2.1 Implement `TestNotificationServiceListAnnouncements_Pagination` to verify page size and total count.
- [x] 2.2 Implement `TestNotificationServiceListAnnouncements_KeywordFilter` to verify keyword search on title.
- [x] 2.3 Implement `TestNotificationServiceListAnnouncements_CreatorUsername` to verify `users` join resolves creator username.

## 3. Announcement Creation Tests

- [x] 3.1 Implement `TestNotificationServiceCreateAnnouncement_StoresCorrectFields` to verify type, source, target_type, title, content, and created_by.

## 4. Announcement Update Tests

- [x] 4.1 Implement `TestNotificationServiceUpdateAnnouncement_Success` to verify title and content are persisted.
- [x] 4.2 Implement `TestNotificationServiceUpdateAnnouncement_NotFound` to verify `ErrNotificationNotFound` for missing IDs.

## 5. Announcement Deletion Tests

- [x] 5.1 Implement `TestNotificationServiceDeleteAnnouncement_Success` to verify record and read tracking are removed.
- [x] 5.2 Implement `TestNotificationServiceDeleteAnnouncement_NotFound` to verify `ErrNotificationNotFound` for missing IDs.

## 6. Verification

- [x] 6.1 Run `go test ./internal/service/ -run TestNotificationService -v` and ensure all tests pass.
- [x] 6.2 Run `go test ./...` to confirm no regressions.

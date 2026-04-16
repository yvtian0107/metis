## ADDED Requirements

### Requirement: Announcement service test infrastructure
The system SHALL provide a test harness for announcement operations in `NotificationService` using an in-memory SQLite database and real `NotificationRepo`, consistent with other kernel service tests.

#### Scenario: Setup test database
- **WHEN** an announcement service test initializes
- **THEN** it SHALL migrate the `users`, `notifications`, and `notification_reads` tables into a shared-memory SQLite database

#### Scenario: Setup DI container
- **WHEN** an announcement service test needs dependencies
- **THEN** it SHALL use `samber/do` to provide the database, `NotificationRepo`, and `NotificationService`

### Requirement: Test announcement listing
The service-layer test suite SHALL verify that `ListAnnouncements` returns paginated results with keyword filtering and creator username resolution.

#### Scenario: List announcements with pagination
- **WHEN** multiple announcements exist and `ListAnnouncements` is called with page and page size
- **THEN** it returns the correct page of items and the total count

#### Scenario: List announcements with keyword filter
- **WHEN** `ListAnnouncements` is called with a keyword matching some announcement titles
- **THEN** only matching announcements are returned

#### Scenario: List announcements includes creator username
- **WHEN** an announcement is created by a user and `ListAnnouncements` is called
- **THEN** the response includes the creator's username via the `users` join

### Requirement: Test announcement creation
The service-layer test suite SHALL verify that `CreateAnnouncement` creates a broadcast notification with the correct metadata.

#### Scenario: Create announcement stores correct fields
- **WHEN** `CreateAnnouncement` is called with title, content, and created_by
- **THEN** the stored notification has `type = "announcement"`, `source = "announcement"`, `target_type = "all"`, and the provided title, content, and created_by

### Requirement: Test announcement update
The service-layer test suite SHALL verify that `UpdateAnnouncement` persists changes and handles missing records.

#### Scenario: Update announcement persists changes
- **WHEN** `UpdateAnnouncement` is called with a new title and content for an existing announcement
- **THEN** the stored notification reflects the updated title and content

#### Scenario: Update missing announcement returns not found
- **WHEN** `UpdateAnnouncement` is called for a non-existent ID
- **THEN** it returns `ErrNotificationNotFound`

### Requirement: Test announcement deletion
The service-layer test suite SHALL verify that `DeleteAnnouncement` removes the record and associated read tracking, and handles missing records.

#### Scenario: Delete announcement removes record and read tracking
- **WHEN** `DeleteAnnouncement` is called for an existing announcement that has `notification_reads` records
- **THEN** the announcement and all its read records are removed

#### Scenario: Delete missing announcement returns not found
- **WHEN** `DeleteAnnouncement` is called for a non-existent ID
- **THEN** it returns `ErrNotificationNotFound`

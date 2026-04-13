## MODIFIED Requirements

### Requirement: Backend DB operation errors MUST be handled explicitly
All database operations in service and handler layers SHALL either check the returned error and act on it, or explicitly log the error with `slog.Error`. No DB operation error SHALL be silently discarded. This includes Org App's `demoteCurrentPrimary` which SHALL NOT use `_ =` to discard errors.

#### Scenario: Compile status update fails
- **WHEN** `kbRepo.Update(kb)` fails during compile error recovery (setting status to "error")
- **THEN** the system SHALL log the update failure with `slog.Error` including the KB ID and original compile error

#### Scenario: Cascade delete partially fails
- **WHEN** `KnowledgeBaseService.Delete` is called and one of `DeleteByKbID` (edges/nodes/sources) fails
- **THEN** the system SHALL return the first encountered error and not proceed with deleting the KB record itself

#### Scenario: Service FindByID distinguishes not-found from DB error
- **WHEN** a service's `Get` method calls `repo.FindByID` and receives an error
- **THEN** the system SHALL return `ErrXxxNotFound` only if the error is `gorm.ErrRecordNotFound`, and return the original error wrapped for all other cases

#### Scenario: Org assignment demote error propagated
- **WHEN** `demoteCurrentPrimary` returns an error during `AddUserPosition` or `UpdateUserPosition`
- **THEN** the service method SHALL return the error and abort the operation, not discard it with `_ =`

### Requirement: Frontend mutations MUST display errors to users
All `useMutation` calls in the frontend SHALL include an `onError` handler that displays the error message to the user via toast notification.

#### Scenario: Delete user fails
- **WHEN** a user deletion API call returns an error
- **THEN** the system SHALL display a toast error with the error message

#### Scenario: Save role permissions fails
- **WHEN** saving role permissions returns an error
- **THEN** the system SHALL display a toast error with the error message

#### Scenario: Org assignment remove fails with not-found
- **WHEN** removing an assignment returns HTTP 404
- **THEN** the system SHALL display a toast error with a user-friendly message

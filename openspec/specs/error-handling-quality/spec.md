# Spec: Error Handling Quality

## Purpose

Define consistent error handling standards across the backend and frontend to ensure errors are never silently discarded, N+1 query patterns are eliminated, unimplemented features return appropriate status codes, and users always receive actionable error feedback.

## Requirements

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

### Requirement: N+1 queries MUST be eliminated in list endpoints
All list/search API endpoints SHALL use batch queries instead of per-item queries within loops.

#### Scenario: Knowledge node list with edge counts
- **WHEN** `GET /api/v1/ai/knowledge/:kbId/nodes` returns N nodes
- **THEN** the system SHALL execute at most 2 SQL queries total (one for nodes, one for all edge counts), not N+1

#### Scenario: Knowledge full graph with edge counts
- **WHEN** `GET /api/v1/ai/knowledge/:kbId/graph` returns the full graph
- **THEN** the system SHALL compute edge counts from the already-loaded edges in memory, executing exactly 2 SQL queries (nodes + edges)

#### Scenario: User list with OAuth connections
- **WHEN** `GET /api/v1/users` returns N users
- **THEN** the system SHALL load all connections in a single batch query, not N individual queries

### Requirement: Unimplemented features MUST NOT return success
API endpoints that are not yet implemented SHALL return HTTP 501 with a clear error message. They SHALL NOT return success responses with placeholder data.

#### Scenario: MCP SSE connection test
- **WHEN** `POST /api/v1/ai/mcp-servers/:id/test` is called for an SSE transport server
- **THEN** the system SHALL return HTTP 501 with message indicating the feature is not yet implemented

#### Scenario: GitHub skill import
- **WHEN** `POST /api/v1/ai/skills/install-github` is called
- **THEN** the system SHALL return HTTP 501 with message indicating the feature is not yet implemented

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

### Requirement: Audit action naming MUST use namespace format
All audit log entries SHALL use `<resource>.<verb>` format for the `audit_action` field (e.g., `knowledgeBase.create`, `mcpServer.update`).

#### Scenario: AI app handler creates audit entry
- **WHEN** any AI app handler sets `audit_action` on the Gin context
- **THEN** the value SHALL follow the `<resource>.<verb>` format consistent with kernel handlers

### Requirement: API client error messages MUST be language-neutral
Hardcoded fallback error messages in the frontend API client SHALL be in English, not in a specific locale. Server-side responses provide the localized message.

#### Scenario: Password expired fallback message
- **WHEN** a 409 response body cannot be parsed
- **THEN** the fallback error message SHALL be in English ("Password has expired"), not Chinese

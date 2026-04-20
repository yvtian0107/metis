## MODIFIED Requirements

### Requirement: Classic engine code organization
The `classic.go` monolithic file (57KB) SHALL be split into multiple files by responsibility. All functions SHALL remain in the `engine` package with unchanged signatures and behavior.

#### Scenario: File split does not change behavior
- **WHEN** classic engine files are reorganized
- **THEN** all existing unit tests and BDD tests SHALL pass without modification

#### Scenario: File organization by responsibility
- **WHEN** the split is complete
- **THEN** the files SHALL be organized as:
  - `classic_core.go` — Start/Progress/Cancel entry points and graph traversal
  - `classic_nodes.go` — Per-node-type processing functions
  - `classic_activity.go` — Activity creation, update, and query helpers
  - `classic_token.go` — ExecutionToken tree operations
  - `classic_notify.go` — Notification dispatch logic
  - `classic_helpers.go` — Type aliases, JSON helpers, and small utility functions

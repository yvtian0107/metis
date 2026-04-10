## ADDED Requirements

### Requirement: Build-time version injection
The build system SHALL inject version metadata into the Go binary via ldflags at compile time.

#### Scenario: Tagged commit
- **WHEN** the current git commit has an exact tag (e.g., `v1.2.0`)
- **THEN** the `Version` variable SHALL be set to the tag value

#### Scenario: Untagged commit
- **WHEN** the current git commit has no exact tag
- **THEN** the `Version` variable SHALL be set to `nightly-YYYYMMDD-<7-char commit hash>` using the build date and short commit hash

#### Scenario: Development mode
- **WHEN** the server is built via `make dev` (without ldflags)
- **THEN** the `Version` variable SHALL default to `dev`

### Requirement: Version package
The system SHALL provide an `internal/version` package exposing build metadata as package-level variables.

#### Scenario: Version variables available
- **WHEN** the binary is compiled with ldflags
- **THEN** the package SHALL expose `Version`, `GitCommit`, and `BuildTime` as string variables

#### Scenario: Default values without ldflags
- **WHEN** the binary is compiled without ldflags (development mode)
- **THEN** `Version` SHALL be `"dev"`, `GitCommit` SHALL be `""`, `BuildTime` SHALL be `""`

# Capability: cli-seed-command

## Purpose
Provides a CLI subcommand (`./server seed`) that runs the full seed pipeline (kernel + all Apps) without starting the HTTP server, enabling operators to re-seed an existing database on demand.

## Requirements

### Requirement: CLI seed subcommand
The server binary SHALL support a `seed` subcommand: `./server seed`. When invoked, it SHALL load the config file, connect to the database, initialize the Casbin enforcer, run `seed.Sync()` for kernel data, then call `App.Seed(db, enforcer, true)` for all registered Apps, and exit with code 0 on success.

#### Scenario: Seed subcommand runs full seed
- **WHEN** the user runs `./server seed`
- **THEN** the system SHALL connect to the configured database, run kernel seed.Sync(), call App.Seed with install=true for each App, log results, and exit

#### Scenario: Seed subcommand respects config flag
- **WHEN** the user runs `./server seed -config /path/to/config.yml`
- **THEN** the system SHALL load the specified config file instead of the default

#### Scenario: Seed subcommand fails on missing config
- **WHEN** the user runs `./server seed` and no config.yml exists
- **THEN** the system SHALL exit with a non-zero code and an error message indicating config is required

#### Scenario: Seed subcommand exits after completion
- **WHEN** the seed subcommand completes successfully
- **THEN** the system SHALL NOT start the HTTP server; it SHALL exit immediately

### Requirement: Subcommand detection
The `main()` function SHALL detect subcommands by checking `os.Args` before flag parsing. When `os.Args[1] == "seed"`, it SHALL enter the seed branch. All other cases SHALL follow the existing startup flow unchanged.

#### Scenario: No subcommand starts server normally
- **WHEN** the user runs `./server` or `./server -port 9000`
- **THEN** the system SHALL start the HTTP server as before (no behavioral change)

#### Scenario: Unknown subcommand shows error
- **WHEN** the user runs `./server unknown`
- **THEN** the system SHALL print an error message and exit with non-zero code

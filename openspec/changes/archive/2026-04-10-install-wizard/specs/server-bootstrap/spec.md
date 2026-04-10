## MODIFIED Requirements

### Requirement: samber/do IOC container
The system SHALL use samber/do v2 as the dependency injection container for managing service lifecycle. In **normal mode** (installed), the container SHALL register all kernel and app providers at startup as before. In **install mode** (not installed), the container SHALL register only minimal providers needed for installation: database.DB (with default SQLite or from metis.yaml), SysConfigRepo, SysConfigService, and InstallHandler. After installation completes, the install handler SHALL register remaining providers and routes into the same container (hot switch).

#### Scenario: Service registration in normal mode
- **WHEN** the application starts and the system is installed
- **THEN** all kernel services (database, repositories, services, handlers, scheduler engine) SHALL be registered first, followed by each registered App's providers, all resolved lazily on first use

#### Scenario: Service registration in install mode
- **WHEN** the application starts and the system is not installed
- **THEN** only database.DB, SysConfigRepo, SysConfigService, and InstallHandler SHALL be registered. No auth, casbin, scheduler, or business handlers SHALL be registered.

#### Scenario: Hot switch after installation
- **WHEN** the install handler completes installation successfully
- **THEN** it SHALL register all remaining kernel and app providers, run app seeds, register all routes, and start the scheduler engine — all within the same process

#### Scenario: App provider registration
- **WHEN** optional Apps are registered in the global registry
- **THEN** main.go SHALL call `a.Providers(injector)` for each App, allowing App services to reference kernel services via `do.MustInvoke`

### Requirement: Graceful shutdown
The system SHALL shut down gracefully on SIGTERM or SIGINT signals.

#### Scenario: Receive termination signal
- **WHEN** the process receives SIGTERM or SIGINT
- **THEN** the system SHALL stop accepting new requests, stop the scheduler engine (if running), complete in-flight requests, close database connections, and exit cleanly

### Requirement: Gin engine with standard middleware
The system SHALL initialize a Gin engine with slog-based request logging and panic recovery middleware. In **install mode**, only install-related routes and SPA static assets SHALL be registered. In **normal mode**, the full route tree (public + authenticated groups) SHALL be registered as before.

#### Scenario: Request logging
- **WHEN** any HTTP request is processed
- **THEN** the middleware SHALL log method, path, status code, and latency using slog

#### Scenario: Panic recovery
- **WHEN** a handler panics during request processing
- **THEN** the middleware SHALL recover, log the error, and return a 500 response

#### Scenario: Install mode routes
- **WHEN** the system is in install mode
- **THEN** the Gin engine SHALL only register `/api/v1/install/*` routes and SPA static asset serving. No JWT, Casbin, or business routes SHALL be registered.

#### Scenario: Normal mode routes
- **WHEN** the system is in normal mode
- **THEN** the system SHALL organize routes into public and authenticated groups with JWTAuth + CasbinAuth middleware, and call `a.Routes(apiGroup)` for each App

### Requirement: Server port configuration
The system SHALL listen on port 8080 by default. The port SHALL be configurable via the `server_port` key in SystemConfig (stored in DB). When the system is not installed, it SHALL always use port 8080.

#### Scenario: Default port
- **WHEN** the system starts and no `server_port` config exists in SystemConfig
- **THEN** the server SHALL listen on port 8080

#### Scenario: Custom port from DB
- **WHEN** the system starts and `server_port` is set to `9090` in SystemConfig
- **THEN** the server SHALL listen on port 9090

#### Scenario: Install mode port
- **WHEN** the system starts in install mode
- **THEN** the server SHALL listen on port 8080 (hardcoded default, DB not yet initialized)

### Requirement: Makefile build commands
The Makefile SHALL provide commands for development and production builds. The `init-dev-user` target SHALL be removed.

#### Scenario: Dev mode
- **WHEN** `make dev` is run
- **THEN** the Go server SHALL start with all modules (no build tags needed). The developer SHALL complete the install wizard in the browser on first run.

#### Scenario: Production build
- **WHEN** `make build` is run
- **THEN** the system SHALL build the frontend (full modules), then compile the Go binary with embedded assets into a single executable

#### Scenario: Edition build
- **WHEN** `make build EDITION=edition_lite APPS=system` is run
- **THEN** the system SHALL generate a minimal frontend registry, build the frontend, then compile the Go binary with `-tags edition_lite`

### Requirement: Scheduler engine startup
The scheduler engine SHALL be started only after installation is complete. In install mode, the scheduler SHALL NOT be started.

#### Scenario: Engine starts in normal mode
- **WHEN** the application starts in normal mode and all TaskDefs have been registered
- **THEN** `engine.Start()` SHALL be called

#### Scenario: Engine starts after hot switch
- **WHEN** installation completes and the system hot-switches to normal mode
- **THEN** `engine.Start()` SHALL be called as part of the hot switch sequence

#### Scenario: No engine in install mode
- **WHEN** the application starts in install mode
- **THEN** the scheduler engine SHALL NOT be created or started

## REMOVED Requirements

### Requirement: Makefile init-dev-user target
**Reason**: Replaced by install wizard. Developers complete the install wizard on first run.
**Migration**: Run `make dev` + `make web-dev`, then open browser and complete the install wizard (30 seconds).

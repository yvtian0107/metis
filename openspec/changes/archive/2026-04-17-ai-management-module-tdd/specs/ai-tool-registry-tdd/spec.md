## ADDED Requirements

### Requirement: Tool registry test infrastructure
The system SHALL provide a test harness for `ToolService`, `MCPServerService`, and `SkillService` using an in-memory SQLite database with `ai_tools`, `ai_mcp_servers`, and `ai_skills` tables, plus a deterministic test encryption key.

#### Scenario: Setup test database
- **WHEN** a tool registry test initializes
- **THEN** it SHALL migrate `ai_tools`, `ai_mcp_servers`, and `ai_skills` tables into a shared-memory SQLite database

### Requirement: Test builtin tool service
The service-layer test suite SHALL verify listing and toggling builtin tools.

#### Scenario: List all builtin tools
- **WHEN** `ToolService.List` is called after seeding tools
- **THEN** it returns all seeded tools

#### Scenario: Toggle tool active state
- **WHEN** `ToggleActive` is called with `isActive=false`
- **THEN** the tool's `IsActive` field is updated to false

### Requirement: Test MCP server validation
The service-layer test suite SHALL verify transport validation rules for MCP servers.

#### Scenario: Reject MCP server without URL for SSE transport
- **WHEN** `Create` is called with transport="sse" and an empty URL
- **THEN** it returns `ErrSSERequiresURL`

#### Scenario: Reject MCP server without command for STDIO transport
- **WHEN** `Create` is called with transport="stdio" and an empty command
- **THEN** it returns `ErrSTDIORequiresCommand`

#### Scenario: Accept valid SSE transport
- **WHEN** `Create` is called with transport="sse" and a non-empty URL
- **THEN** the MCP server is persisted

#### Scenario: Encrypt and decrypt MCP auth config
- **WHEN** `Create` is called with authType="api_key" and a JSON auth config string
- **THEN** the auth config is encrypted; `DecryptAuthConfig` returns the original plaintext

#### Scenario: Mask MCP auth config
- **WHEN** `MaskAuthConfig` is called on an MCP server with JSON auth config `{"key":"sk-1234567890abcdef"}`
- **THEN** it returns a JSON string with the value masked as `sk-****` + last 4 chars

### Requirement: Test skill service
The service-layer test suite SHALL verify skill import and metadata.

#### Scenario: Import a GitHub skill
- **WHEN** `ImportGitHub` is called with a valid GitHub URL
- **THEN** a skill is created with sourceType="github" and the provided sourceUrl

#### Scenario: Skill response includes tool count
- **WHEN** a skill with a 2-item `ToolsSchema` array is saved
- **THEN** `ToResponse` returns `ToolCount=2`

## ADDED Requirements

### Requirement: Knowledge source as independent entity
The system SHALL manage knowledge sources (files, URLs, text, FAQ) as independent entities decoupled from any specific knowledge base or knowledge graph. Each source SHALL have: id, title, format (pdf/docx/xlsx/pptx/markdown/text/url), content, file reference, extract_status (pending/extracting/ready/error), content_hash, byte_size, and timestamps.

#### Scenario: Upload file source
- **WHEN** user uploads a PDF/DOCX/XLSX/PPTX/Markdown/TXT file
- **THEN** system SHALL create a source record with extract_status=pending and asynchronously extract content to Markdown

#### Scenario: Add URL source
- **WHEN** user adds a URL with optional crawl_depth (0/1/2) and url_pattern
- **THEN** system SHALL create a source record and asynchronously crawl and extract content

#### Scenario: Manual text source
- **WHEN** user creates a source with inline text or FAQ content
- **THEN** system SHALL store the content directly with extract_status=ready

#### Scenario: Source extraction completion
- **WHEN** async extraction completes successfully
- **THEN** system SHALL update extract_status to ready and store the extracted Markdown content

#### Scenario: Source extraction failure
- **WHEN** async extraction fails
- **THEN** system SHALL update extract_status to error with error_message

### Requirement: Source CRUD API
The system SHALL provide REST endpoints under `/api/v1/ai/knowledge/sources` with JWT + Casbin auth:
- `POST /` — upload file or add URL/text source
- `GET /` — list sources with pagination, format filter, status filter, keyword search
- `GET /:id` — get source detail including extracted content
- `DELETE /:id` — delete source (blocked if referenced by any asset, or cascade-remove references)

#### Scenario: List sources with filter
- **WHEN** user requests `GET /api/v1/ai/knowledge/sources?format=pdf&status=ready`
- **THEN** system SHALL return only PDF sources with ready status

#### Scenario: Delete referenced source
- **WHEN** user deletes a source that is referenced by one or more knowledge assets
- **THEN** system SHALL return a 409 error listing the referencing assets

#### Scenario: Source reference tracking
- **WHEN** user views a source detail
- **THEN** system SHALL include a list of knowledge assets (knowledge bases and knowledge graphs) that reference this source

### Requirement: Source-asset M:N association
The system SHALL support many-to-many relationships between sources and knowledge assets via an association table. One source MAY be referenced by multiple knowledge bases and knowledge graphs simultaneously.

#### Scenario: Associate source with multiple assets
- **WHEN** user adds source S1 to knowledge base KB1 and knowledge graph KG1
- **THEN** both KB1 and KG1 SHALL reference S1 without duplicating the source data

#### Scenario: Source update notification
- **WHEN** a source's content changes (re-crawl or re-upload)
- **THEN** system SHALL mark all referencing assets as needing rebuild (status change to stale or notification)

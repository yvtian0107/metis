## ADDED Requirements

### Requirement: Stream protocol encoder abstraction
The system SHALL expose a protocol-agnostic stream encoder boundary between internal AI execution events and external SSE payloads. The encoder boundary SHALL accept the unified Gateway event stream and produce protocol-specific output without changing Gateway orchestration, persistence, or executor behavior.

#### Scenario: Gateway uses encoder abstraction
- **WHEN** Gateway forwards execution events to an SSE response
- **THEN** it SHALL do so through an encoder abstraction rather than directly embedding Vercel-specific line construction in orchestration logic

#### Scenario: Encoder preserves existing Vercel output
- **WHEN** the default encoder is selected for the current chat endpoint
- **THEN** it SHALL produce the same Vercel UI stream semantics currently expected by the frontend

### Requirement: Protocol abstraction does not require new public endpoint
The protocol encoder abstraction SHALL be introducible without adding a new public API route. Existing chat routes SHALL continue to use the default encoder until a future change adds additional protocols.

#### Scenario: No new endpoint required
- **WHEN** this change is implemented
- **THEN** the system SHALL keep using the existing `/api/v1/ai/sessions/:sid/stream` route for chat streaming

#### Scenario: Future protocol can be added without Gateway rewrite
- **WHEN** a future change adds another stream protocol such as AGUI
- **THEN** the new protocol SHALL be implementable by adding a new encoder and route binding without rewriting Gateway orchestration and persistence logic

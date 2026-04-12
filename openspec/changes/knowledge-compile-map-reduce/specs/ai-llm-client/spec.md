## MODIFIED Requirements

### Requirement: Unified LLM client interface
The system SHALL provide a unified LLM client at `internal/llm/` that abstracts over multiple provider protocols. The client interface SHALL support Chat, ChatStream, and Embedding operations. The HTTP client timeout SHALL be set to 300 seconds (5 minutes) to accommodate large-input scenarios such as knowledge compilation, while relying on the caller's context deadline as the primary timeout control mechanism.

#### Scenario: Create client from provider config
- **WHEN** a caller provides protocol, base_url, and api_key
- **THEN** `llm.NewClient(protocol, baseURL, apiKey)` returns the appropriate protocol implementation
- **THEN** the underlying HTTP client SHALL have a timeout of 300 seconds

#### Scenario: OpenAI protocol client
- **WHEN** client is created with protocol `openai`
- **THEN** it uses `sashabaranov/go-openai` library for all API calls
- **THEN** the HTTP client timeout SHALL be 300 seconds

#### Scenario: Anthropic protocol client
- **WHEN** client is created with protocol `anthropic`
- **THEN** it uses `anthropics/anthropic-sdk-go` library for all API calls
- **THEN** the HTTP client timeout SHALL be 300 seconds

#### Scenario: Context deadline governs actual timeout
- **WHEN** caller passes a context with a deadline shorter than 300 seconds
- **THEN** the request SHALL respect the context deadline and cancel accordingly

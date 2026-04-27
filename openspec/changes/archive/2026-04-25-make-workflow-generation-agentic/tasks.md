## 1. Backend Generation Semantics

- [x] 1.1 Update `WorkflowGenerateService.buildGenerateResponse()` so a parsable workflow_json is saved even when validation errors contain Level="blocking"
- [x] 1.2 Ensure saved generation responses include `saved=true`, updated `service`, `healthCheck`, and the full validation `errors`
- [x] 1.3 Update `WorkflowGenerateHandler.Generate()` so responses with workflowJson and validation errors return HTTP 200 instead of HTTP 400
- [x] 1.4 Keep error responses for empty collaboration spec, missing path engine config, LLM upstream failure, and unrecoverable JSON extraction failure

## 2. Workflow Validation

- [x] 2.1 Change `detectDeadEnds()` to collect all end nodes and run reverse reachability from every end node
- [x] 2.2 Add regression coverage proving multiple independent end nodes are accepted when each branch reaches one end
- [x] 2.3 Add regression coverage proving an end node is not reported as unable to reach another end node
- [x] 2.4 Keep dead-end detection for non-end nodes that cannot reach any end node

## 3. Service Health And API Tests

- [x] 3.1 Add service-level generation test for blocking validation issues returning a successful GenerateResponse with saved=true and errors populated
- [x] 3.2 Add handler test proving `POST /api/v1/itsm/workflows/generate` returns HTTP 200 for a parsable workflow_json with blocking validation issues
- [x] 3.3 Add or adjust health-check test proving saved blocking issues surface as reference path fail status
- [x] 3.4 Update existing tests that expected generation blocking issues to produce HTTP 400

## 4. Frontend Experience

- [x] 4.1 Extend `WorkflowGenerateResponse` typing with `saved`, `errors[].level`, `service`, and `healthCheck`
- [x] 4.2 Update `GenerateWorkflowButton` success handling so responses with errors show a non-blocking warning toast instead of going through mutation error handling
- [x] 4.3 Ensure generation success refreshes or updates current service detail cache and service list cache so the workflow graph and health check reflect the generated draft
- [x] 4.4 Add or update i18n copy for "generated but needs confirmation" without presenting the result as a hard failure

## 5. Verification

- [x] 5.1 Run targeted Go tests for `internal/app/itsm/engine` validator coverage
- [x] 5.2 Run targeted Go tests for `internal/app/itsm/definition` workflow generation coverage
- [x] 5.3 Run frontend lint/build checks affected by the service definition page changes
- [x] 5.4 Run `go build -tags dev ./cmd/server`

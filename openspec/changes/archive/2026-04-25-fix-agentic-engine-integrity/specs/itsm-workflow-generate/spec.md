## ADDED Requirements

### Requirement: Blocking errors prevent workflow persistence
workflow_generate_service SHALL check ValidationError.Level before persisting. If any error has Level="blocking", the service SHALL NOT save the workflow to ServiceDefinition and SHALL return HTTP 400 with the error list. If all errors have Level="warning", the workflow SHALL be saved normally with warnings included in the response.

#### Scenario: Blocking validation error prevents save
- **WHEN** workflow generation produces a valid JSON but ValidateWorkflow returns a blocking error (e.g., cycle detected)
- **THEN** the workflow is NOT saved to the database and HTTP 400 is returned with the blocking error details

#### Scenario: Warning-only errors allow save
- **WHEN** workflow generation produces a valid JSON and ValidateWorkflow returns only warning-level errors (e.g., formSchema field reference mismatch)
- **THEN** the workflow IS saved to ServiceDefinition and the response includes the warnings

#### Scenario: No errors — normal save
- **WHEN** workflow generation produces a valid JSON and ValidateWorkflow returns no errors
- **THEN** the workflow is saved normally

## MODIFIED Requirements

### Requirement: 解析结果保存
生成的 workflow_json 在保存前 SHALL 经过验证。如果验证结果中包含任何 Level="blocking" 的错误，workflow SHALL NOT 被保存到 ServiceDefinition，API SHALL 返回 400 并携带错误列表。如果所有错误均为 Level="warning"，workflow SHALL 正常保存并在响应中附带警告。保存后 SHALL 调用 RefreshPublishHealthCheck。

#### Scenario: 保存工作流到服务定义
- **WHEN** 生成结果通过验证（无 blocking 错误）
- **THEN** 系统将 workflow_json 写入 ServiceDefinition 并触发 RefreshPublishHealthCheck

#### Scenario: Blocking 错误阻止保存
- **WHEN** 生成结果包含 blocking 级别验证错误
- **THEN** 系统不保存 workflow_json，返回 400 状态码及错误详情

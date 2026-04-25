## ADDED Requirements

### Requirement: Backend validation activated in variable_writer
writeFormBindings() in variable_writer.go SHALL call form.ValidateFormData(schema, data) before writing any process variables. If validation returns errors, the function SHALL NOT write any variables, SHALL return the validation errors to the caller, and the caller SHALL record a timeline event with type "form_validation_failed" including the field-level error details. The activity completion SHALL still proceed but with form data rejected.

#### Scenario: Valid form data passes and variables are written
- **WHEN** writeFormBindings is called with form data that passes ValidateFormData
- **THEN** all form field values are written to process variables normally

#### Scenario: Invalid form data rejected
- **WHEN** writeFormBindings is called with form data where a required field is missing
- **THEN** no process variables are written, validation errors are returned, and a timeline event records the failure

#### Scenario: Partial validation failure rejects all fields
- **WHEN** writeFormBindings is called with form data where field A is valid but field B fails validation
- **THEN** neither field A nor field B is written to process variables (atomic rejection)

#### Scenario: Validation failure does not block activity completion
- **WHEN** form validation fails during activity completion
- **THEN** the activity can still be marked as completed but its form data is not propagated to process variables

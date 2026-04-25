## ADDED Requirements

### Requirement: Frontend dynamic Zod schema generation
The system SHALL dynamically generate a Zod validation schema from a FormSchema definition. Each field's validation rules SHALL map to corresponding Zod validators. The generated schema SHALL be memoized (useMemo) to avoid recomputation when the FormSchema has not changed.

#### Scenario: Required field generates z.string().min(1)
- **WHEN** a text field has `required: true` or validation rule `{"rule":"required"}`
- **THEN** the Zod schema SHALL include a non-empty string validator for that field

#### Scenario: Number min/max validation
- **WHEN** a number field has validation `[{"rule":"min","value":1},{"rule":"max","value":100}]`
- **THEN** the Zod schema SHALL include `z.number().min(1).max(100)` for that field

#### Scenario: Pattern validation
- **WHEN** a text field has validation `[{"rule":"pattern","value":"^[A-Z]+$","message":"仅限大写字母"}]`
- **THEN** the Zod schema SHALL include `z.string().regex(/^[A-Z]+$/, "仅限大写字母")`

#### Scenario: Hidden fields excluded from validation
- **WHEN** a field is hidden by visibility conditions
- **THEN** the Zod schema SHALL mark that field as optional, not requiring validation

### Requirement: Backend Go schema-based validation
The system SHALL provide a Go function `ValidateFormData(schema FormSchema, data map[string]any) []ValidationError` that validates submitted form data against the schema. ValidationError SHALL contain `field` (string) and `message` (string).

#### Scenario: Required field missing
- **WHEN** a required field "title" is missing from the submitted data
- **THEN** ValidateFormData SHALL return `[{field:"title", message:"此字段为必填项"}]`

#### Scenario: String too short
- **WHEN** field "name" has minLength=3 validation and submitted value is "ab"
- **THEN** ValidateFormData SHALL return `[{field:"name", message:"..."}]` with the configured message

#### Scenario: All fields valid
- **WHEN** all submitted field values pass their validation rules
- **THEN** ValidateFormData SHALL return an empty slice

#### Scenario: Email validation
- **WHEN** a field with email validation receives "not-an-email"
- **THEN** ValidateFormData SHALL return a validation error for that field

### Requirement: Consistent validation behavior
Frontend Zod validation and backend Go validation SHALL produce equivalent results for the same schema and data. Both SHALL check the same rule set: required, minLength, maxLength, min, max, pattern, email, url.

#### Scenario: Same error on frontend and backend
- **WHEN** a required text field is submitted empty
- **THEN** both frontend (Zod) and backend (Go) SHALL reject the value

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

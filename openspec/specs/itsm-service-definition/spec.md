## MODIFIED Requirements

### Requirement: ServiceDefinition model form reference
ServiceDefinition SHALL store the intake form schema inline via `intake_form_schema` (JSONField, TEXT, nullable) instead of referencing an external FormDefinition via `form_id`. The `form_id` column SHALL be removed. ServiceDefinitionResponse SHALL include `intakeFormSchema` (JSONField) instead of `formId`.

#### Scenario: Create service with inline form schema
- **WHEN** a CreateServiceDefinition request includes `intakeFormSchema: {"version":1,"fields":[...]}`
- **THEN** the system SHALL store the schema inline and return the created service with the embedded schema

#### Scenario: Create service without form
- **WHEN** a CreateServiceDefinition request omits `intakeFormSchema`
- **THEN** the system SHALL accept the request with `intake_form_schema=NULL`

#### Scenario: Invalid form schema rejected
- **WHEN** a CreateServiceDefinition request includes an `intakeFormSchema` that fails ValidateSchema()
- **THEN** the system SHALL return HTTP 400 with validation errors

### Requirement: Workflow node form reference
WorkflowDef NodeData SHALL store form schema inline via `formSchema` (json.RawMessage) instead of referencing a FormDefinition via `formId`. The `formId` field SHALL be removed from NodeData struct. The engine SHALL read formSchema directly from the node data when creating activities.

#### Scenario: Classic engine reads inline form schema
- **WHEN** the ClassicEngine encounters a form node with `formSchema: {"version":1,"fields":[...]}`
- **THEN** it SHALL copy the schema directly into the created activity's form_schema field

#### Scenario: Node without form schema
- **WHEN** the engine encounters a form/user_task node with no formSchema
- **THEN** it SHALL create the activity without form_schema

## MODIFIED Requirements

### Requirement: Ticket creation triggers variable initialization
When the classic engine creates a ticket (Start → first node), if the service has `intakeFormSchema` with fields that have binding properties, the engine SHALL write those form values as process variables before proceeding to the first activity node. The engine SHALL read the form schema from `ServiceDefinition.IntakeFormSchema` instead of querying FormDefinition by formId.

#### Scenario: Start with bound form fields
- **WHEN** a ticket is created for a service whose `intakeFormSchema` has fields with binding
- **AND** the user submits form data
- **THEN** process variables are initialized from bound fields before the first activity is created

#### Scenario: Start without form
- **WHEN** a ticket is created for a service that has no `intakeFormSchema` (NULL)
- **THEN** no process variables are created at ticket creation time

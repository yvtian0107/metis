## Requirements

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

### Requirement: ServiceDefinition 关联知识库
ServiceDefinition SHALL 新增 `knowledge_base_ids` 字段（TEXT, JSON 数组, nullable），存储关联的知识库 ID 列表。创建和更新 API SHALL 支持该字段的读写。

#### Scenario: 创建服务时关联知识库
- **WHEN** CreateServiceDefinition 请求包含 `knowledgeBaseIds: [1, 3]`
- **THEN** 系统 SHALL 存储 `knowledge_base_ids` 为 `[1,3]` 并在响应中返回

#### Scenario: 创建服务不关联知识库
- **WHEN** CreateServiceDefinition 请求未包含 `knowledgeBaseIds`
- **THEN** 系统 SHALL 接受请求，`knowledge_base_ids` 为 NULL

#### Scenario: 更新服务的知识库关联
- **WHEN** UpdateServiceDefinition 请求包含 `knowledgeBaseIds: [2]`
- **THEN** 系统 SHALL 覆盖更新 `knowledge_base_ids` 为 `[2]`

#### Scenario: 知识库 ID 容错
- **WHEN** `knowledge_base_ids` 中包含不存在的知识库 ID
- **THEN** 存储时 SHALL 不做校验（写入时容忍），搜索时忽略不存在的 KB

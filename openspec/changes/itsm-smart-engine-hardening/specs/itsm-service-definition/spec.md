## ADDED Requirements

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

## MODIFIED Requirements

### Requirement: decision.knowledge_search 工具
该工具 SHALL 搜索服务关联的知识库，返回与查询相关的知识片段。复用现有 `KnowledgeSearcher` 接口。工具 SHALL 从 ServiceDefinition 的 `knowledge_base_ids` 字段获取关联知识库 ID 列表，传递给 KnowledgeSearcher 进行搜索。

参数：
- `query` (string, required): 搜索查询文本
- `limit` (integer, optional, default 3): 返回结果数量上限

返回字段：
- `results`: 知识结果数组（title, content, score）
- `count`: 实际返回数量

#### Scenario: 搜索有结果
- **WHEN** Agent 调用 `decision.knowledge_search` 且服务关联的知识库中有匹配内容
- **THEN** 返回结果 SHALL 包含按 score 降序排列的知识片段，每项含 title、content 摘要和相关度 score

#### Scenario: KnowledgeSearcher 不可用
- **WHEN** Agent 调用 `decision.knowledge_search` 但 KnowledgeSearcher 为 nil（AI App 知识模块未启用）
- **THEN** 工具 SHALL 返回 `{"results": [], "count": 0, "message": "知识搜索不可用"}`，不视为错误

#### Scenario: 服务无关联知识库
- **WHEN** Agent 调用 `decision.knowledge_search` 但当前服务的 `knowledge_base_ids` 为空或 NULL
- **THEN** 工具 SHALL 返回空结果 `{"results": [], "count": 0}`

#### Scenario: 知识库 ID 部分失效
- **WHEN** Agent 调用 `decision.knowledge_search` 且 `knowledge_base_ids` 中包含已删除的知识库 ID
- **THEN** 工具 SHALL 忽略不存在的 KB ID，仅搜索仍存在的知识库，返回有效结果

## Why

当前 ITSM 智能引擎服务定义中的"知识库"字段（`knowledgeBaseIds`）引用的是全局 AI 知识库，这在设计上是错误的。服务定义需要的是自己专属的附件知识文档（PDF、Word、Markdown 等），这些文档在上传后被解析为文本，在工单处理时作为补充上下文注入给 Agent。这与 bklite-cloud 的已验证设计一致。

## What Changes

- **新增 `ServiceKnowledgeDocument` 模型**：服务定义专属的知识文档，支持文件上传，包含解析状态机（pending → processing → completed → failed）和 `parsed_text` 字段
- **新增文档解析包 `internal/pkg/docparse/`**：纯 Go 实现的文档文本提取，支持 TXT、Markdown、PDF、DOCX、XLSX、PPTX 格式，可被 ITSM 和 AI 模块共享
- **新增知识文档 CRUD API**：`/api/v1/itsm/services/:id/knowledge-documents`，包含文件上传、列表、删除等接口
- **新增异步解析调度任务**：上传文档后异步提取文本内容
- **新增前端"附件知识"卡片组件**：替换当前 `SmartServiceConfig` 中的全局知识库多选
- **BREAKING** 移除 `ServiceDefinition.KnowledgeBaseIDs` 字段及相关前端逻辑

## Capabilities

### New Capabilities
- `itsm-service-knowledge-doc`: 服务定义专属知识文档的完整生命周期管理——上传、解析、存储、运行时注入

### Modified Capabilities
- `itsm-smart-engine`: 智能引擎运行时上下文构建方式变更，从引用全局 AI 知识库改为使用服务自身解析后的知识文档文本

## Impact

- **后端模型**：新增 `ServiceKnowledgeDocument` 表，移除 `ServiceDefinition` 的 `knowledge_base_ids` JSON 字段
- **后端 API**：新增 `/api/v1/itsm/services/:id/knowledge-documents` 系列接口
- **后端包**：新增 `internal/pkg/docparse/` 共享文档解析包
- **调度器**：新增 `itsm-doc-parse` 异步任务
- **前端**：`SmartServiceConfig` 组件重构，移除全局知识库选择，新增文件上传卡片
- **权限**：新增 `itsm:service-knowledge:create/delete` 权限
- **依赖**：新增 Go 纯文本解析库（`ledongthuc/pdf`、`xuri/excelize` 等）

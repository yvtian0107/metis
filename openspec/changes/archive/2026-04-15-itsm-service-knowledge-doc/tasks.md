## 1. 文档解析共享包

- [x] 1.1 创建 `internal/pkg/docparse/` 包，定义 `Parse(filePath string) (string, error)` 入口函数，根据文件扩展名分发到对应解析器
- [x] 1.2 实现 TXT/MD 解析器（直接 `os.ReadFile`）
- [x] 1.3 实现 PDF 解析器（引入 `ledongthuc/pdf` 依赖，逐页提取文本）
- [x] 1.4 实现 DOCX 解析器（`archive/zip` + `encoding/xml`，提取 `word/document.xml` 中的 `<w:t>` 文本）
- [x] 1.5 实现 XLSX 解析器（引入 `xuri/excelize/v2`，遍历工作表逐行提取）
- [x] 1.6 实现 PPTX 解析器（`archive/zip` + `encoding/xml`，提取 `ppt/slides/slide*.xml` 中的 `<a:t>` 文本）

## 2. 后端模型与仓库

- [x] 2.1 创建 `ServiceKnowledgeDocument` 模型（model_knowledge_doc.go），包含 service_id、file_name、file_path、file_size、file_type、parse_status、parsed_text、parse_error 字段
- [x] 2.2 在 ITSM App 的 `Models()` 中注册 ServiceKnowledgeDocument 以进行 AutoMigrate
- [x] 2.3 创建 knowledge_doc_repository.go，实现 Create、ListByServiceID、Delete、UpdateParseResult 方法
- [x] 2.4 从 ServiceDefinition 模型中移除 `KnowledgeBaseIDs` 字段

## 3. 后端服务与 API

- [x] 3.1 创建 knowledge_doc_service.go，实现上传（保存文件 + 创建记录 + 提交异步任务）、列表、删除业务逻辑
- [x] 3.2 创建 knowledge_doc_handler.go，实现三个 API 端点（POST 上传、GET 列表、DELETE 删除），包含文件类型和大小校验
- [x] 3.3 在 ITSM App 的 `Routes()` 中注册知识文档路由 `services/:id/knowledge-documents`
- [x] 3.4 在 ITSM App 的 `Providers()` 中注册 KnowledgeDocRepository 和 KnowledgeDocService

## 4. 异步解析任务

- [x] 4.1 创建 `itsm-doc-parse` Scheduler 任务定义，handler 从 payload 读取 document_id，调用 docparse.Parse 并更新记录
- [x] 4.2 在 ITSM App 的 `Tasks()` 中注册该任务

## 5. Seed 与权限

- [x] 5.1 在 ITSM seed 中添加知识文档 API 路由的 Casbin 策略（复用 `itsm:service:update` 权限，不需要独立菜单权限）
- [x] 5.2 在 seed 中添加知识文档 API 路由的 Casbin 策略

## 6. 前端：移除旧知识库逻辑

- [x] 6.1 从 `smart-service-config.tsx` 中移除全局知识库多选组件（knowledgeBaseIds 相关 props 和 UI）
- [x] 6.2 从 `api.ts` 中移除 `fetchKnowledgeBases` 引用（如 ITSM 专用的）
- [x] 6.3 从服务创建/编辑表单中移除 `knowledgeBaseIds` 字段

## 7. 前端：附件知识组件

- [x] 7.1 在 `api.ts` 中添加知识文档 API 函数（uploadKnowledgeDoc、fetchKnowledgeDocs、deleteKnowledgeDoc）
- [x] 7.2 创建 `ServiceKnowledgeCard` 组件（文档列表 + 上传按钮 + 删除确认 + 解析状态展示）
- [x] 7.3 在服务编辑页面中集成 `ServiceKnowledgeCard`（仅 engine_type=smart 时显示）
- [x] 7.4 在服务创建页面中集成 `ServiceKnowledgeCard`（创建成功后可上传文档，或提示先保存服务）

## 8. 前端：国际化

- [x] 8.1 在 `locales/zh-CN.json` 和 `locales/en.json` 中添加知识文档相关翻译键（上传、删除、解析状态等）

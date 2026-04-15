## ADDED Requirements

### Requirement: 服务知识文档模型
系统 SHALL 提供 `ServiceKnowledgeDocument` 模型，支持为每个服务定义上传和管理专属知识文档。

模型字段：
- `id`: 主键（BaseModel）
- `service_id`: 外键关联 ServiceDefinition（NOT NULL）
- `file_name`: 原始文件名（NOT NULL）
- `file_path`: 服务器存储路径（NOT NULL）
- `file_size`: 文件大小（字节）
- `file_type`: 文件 MIME 类型
- `parse_status`: 解析状态枚举（`pending` | `processing` | `completed` | `failed`）
- `parsed_text`: 解析后的纯文本内容（TEXT 类型，可为空）
- `parse_error`: 解析失败时的错误信息
- `created_at`、`updated_at`、`deleted_at`：BaseModel 标准时间戳

#### Scenario: 创建知识文档记录
- **WHEN** 管理员为某个服务上传一个文件
- **THEN** 系统 SHALL 创建 ServiceKnowledgeDocument 记录，`parse_status` 初始为 `pending`，同时提交 `itsm-doc-parse` 异步任务

#### Scenario: 文档关联服务
- **WHEN** 查询某个服务的知识文档
- **THEN** 系统 SHALL 返回所有 `service_id` 匹配的文档列表，按 `created_at` 降序排列

#### Scenario: 删除知识文档
- **WHEN** 管理员删除一个知识文档
- **THEN** 系统 SHALL 软删除该记录（GORM soft delete），不删除物理文件

### Requirement: 文档上传与管理 API
系统 SHALL 提供 RESTful API 管理服务知识文档。

API 端点：
- `POST /api/v1/itsm/services/:id/knowledge-documents` — 上传文档（multipart/form-data）
- `GET /api/v1/itsm/services/:id/knowledge-documents` — 列出服务的所有知识文档
- `DELETE /api/v1/itsm/services/:id/knowledge-documents/:docId` — 删除知识文档

#### Scenario: 上传文档
- **WHEN** 管理员通过 POST 接口上传文件（支持 PDF、DOCX、XLSX、PPTX、TXT、MD）
- **THEN** 系统 SHALL 保存文件到磁盘，创建 ServiceKnowledgeDocument 记录，提交异步解析任务，返回文档记录（含 id、file_name、parse_status）

#### Scenario: 上传不支持的文件类型
- **WHEN** 上传的文件 MIME 类型不在支持列表中
- **THEN** 系统 SHALL 返回 400 错误 "不支持的文件类型"

#### Scenario: 文件大小限制
- **WHEN** 上传的文件大小超过 10MB
- **THEN** 系统 SHALL 返回 400 错误 "文件大小超过限制"

#### Scenario: 列出知识文档
- **WHEN** 通过 GET 接口查询服务的知识文档
- **THEN** 系统 SHALL 返回文档列表，每项包含 id、file_name、file_size、file_type、parse_status、parse_error、created_at

#### Scenario: 删除知识文档
- **WHEN** 通过 DELETE 接口删除文档
- **THEN** 系统 SHALL 软删除记录，返回 200 成功

#### Scenario: 权限控制
- **WHEN** 非授权用户尝试操作知识文档
- **THEN** 系统 SHALL 通过 Casbin 策略 `itsm:service-knowledge:create` 和 `itsm:service-knowledge:delete` 进行权限检查

### Requirement: 文档解析引擎
系统 SHALL 提供 `internal/pkg/docparse/` 共享包，支持从多种文档格式中提取纯文本。

支持格式：
| 格式 | 扩展名 | 解析方式 |
|------|--------|----------|
| 纯文本 | .txt | 直接读取 |
| Markdown | .md | 直接读取 |
| PDF | .pdf | `ledongthuc/pdf` 纯 Go 解析 |
| Word | .docx | ZIP + XML 解析 |
| Excel | .xlsx | `xuri/excelize/v2` |
| PowerPoint | .pptx | ZIP + XML 解析 |

#### Scenario: 解析 TXT/MD 文件
- **WHEN** 解析引擎收到 .txt 或 .md 文件
- **THEN** 引擎 SHALL 直接读取文件全部内容作为 parsed_text

#### Scenario: 解析 PDF 文件
- **WHEN** 解析引擎收到 .pdf 文件
- **THEN** 引擎 SHALL 使用纯 Go PDF 库提取所有页面的文本内容，按页拼接

#### Scenario: 解析 DOCX 文件
- **WHEN** 解析引擎收到 .docx 文件
- **THEN** 引擎 SHALL 解压 ZIP，解析 `word/document.xml`，提取所有 `<w:t>` 标签的文本内容

#### Scenario: 解析 XLSX 文件
- **WHEN** 解析引擎收到 .xlsx 文件
- **THEN** 引擎 SHALL 遍历所有工作表，逐行逐单元格提取文本，每行用制表符分隔，每表用换行分隔

#### Scenario: 解析 PPTX 文件
- **WHEN** 解析引擎收到 .pptx 文件
- **THEN** 引擎 SHALL 解压 ZIP，解析 `ppt/slides/slide*.xml`，提取所有 `<a:t>` 标签的文本内容

#### Scenario: 解析失败处理
- **WHEN** 文件解析过程中发生错误（文件损坏、格式不符等）
- **THEN** 引擎 SHALL 返回错误信息，调用方负责更新 parse_status 为 `failed` 和 parse_error

### Requirement: 异步文档解析任务
系统 SHALL 注册 `itsm-doc-parse` 异步任务到 Scheduler，负责解析上传的知识文档。

#### Scenario: 任务触发
- **WHEN** 新建 ServiceKnowledgeDocument 记录后
- **THEN** 系统 SHALL 向 Scheduler 提交 `itsm-doc-parse` 异步任务，payload 包含 document_id

#### Scenario: 任务执行成功
- **WHEN** Scheduler 执行 `itsm-doc-parse` 且解析成功
- **THEN** 系统 SHALL 更新 `parse_status` 为 `completed`，将提取的文本存入 `parsed_text` 字段

#### Scenario: 任务执行失败
- **WHEN** Scheduler 执行 `itsm-doc-parse` 且解析失败
- **THEN** 系统 SHALL 更新 `parse_status` 为 `failed`，将错误信息存入 `parse_error` 字段

#### Scenario: 解析中状态
- **WHEN** `itsm-doc-parse` 任务开始执行
- **THEN** 系统 SHALL 先更新 `parse_status` 为 `processing`，再执行实际解析

### Requirement: 前端附件知识管理组件
系统 SHALL 在服务定义编辑页面提供"附件知识"卡片组件，替换当前的全局知识库多选。

#### Scenario: 展示已上传文档列表
- **WHEN** 管理员查看智能服务定义的编辑页面
- **THEN** 系统 SHALL 在"附件知识"卡片中展示该服务已上传的知识文档列表，每项显示文件名、文件大小、解析状态

#### Scenario: 上传新文档
- **WHEN** 管理员点击上传按钮并选择文件
- **THEN** 系统 SHALL 通过 API 上传文件，上传成功后刷新文档列表，新文档显示"解析中"状态

#### Scenario: 删除已上传文档
- **WHEN** 管理员点击文档列表中的删除按钮并确认
- **THEN** 系统 SHALL 调用删除 API，成功后从列表中移除该文档

#### Scenario: 解析状态实时展示
- **WHEN** 文档处于 `pending` 或 `processing` 状态
- **THEN** 系统 SHALL 展示对应的状态标签（待解析 / 解析中 / 已完成 / 失败），失败时展示错误信息

#### Scenario: 组件仅在智能引擎下显示
- **WHEN** 服务定义的 `engine_type` 不是 `smart`
- **THEN** 系统 SHALL 不显示"附件知识"卡片

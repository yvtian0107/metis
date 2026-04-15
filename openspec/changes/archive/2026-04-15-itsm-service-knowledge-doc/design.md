## Context

当前 ITSM 智能引擎的服务定义通过 `knowledge_base_ids` JSON 字段引用全局 AI 知识库（`/api/v1/ai/knowledge-bases`），在 Agent 决策时通过 KnowledgeService 做语义检索。这个设计存在两个问题：

1. **概念错位**：全局 AI 知识库是 AI 模块的知识图谱系统（包含编译、节点、边等复杂结构），ITSM 服务需要的只是简单的参考文档（操作手册、FAQ、流程说明）
2. **使用门槛高**：管理员需要先在 AI 模块创建知识库、上传并等待编译完成，才能在 ITSM 中引用

bklite-cloud 的已验证设计采用服务专属知识文档模式：每个服务定义可上传自己的附件文档（PDF、Word、Markdown 等），系统解析为纯文本后，在 Agent 决策时直接注入为上下文。

## Goals / Non-Goals

**Goals:**
- 为每个服务定义提供专属知识文档管理能力（上传、解析、删除）
- 支持常见文档格式的纯文本提取（TXT、MD、PDF、DOCX、XLSX、PPTX）
- 提取的文本在 Agent 决策时作为 knowledge_context 注入 system prompt
- 提供可复用的文档解析包，供 ITSM 和未来其他模块使用
- 移除对全局 AI 知识库的引用

**Non-Goals:**
- 不做语义检索或向量嵌入——直接全文注入（文档数量和大小有限，不需要 RAG）
- 不替换 AI 知识库模块本身——那是独立的知识图谱系统
- 不支持图片/音频/视频等非文本内容提取
- 不做文档版本管理

## Decisions

### 1. 服务专属文档 vs 引用全局知识库

**选择**：服务专属文档（ServiceKnowledgeDocument 模型，FK 到 ServiceDefinition）

**替代方案**：保留全局 AI 知识库引用，但简化 UI
- 拒绝原因：概念不匹配，AI 知识库的编译/图谱机制对 ITSM 场景过于复杂

### 2. 全文注入 vs 语义检索

**选择**：全文注入——将所有已解析文档的 `parsed_text` 拼接注入 system prompt

**理由**：
- 每个服务的附件文档数量有限（通常 3-10 个），总文本量可控
- 避免引入向量数据库依赖
- 简单可靠，减少出错环节

**替代方案**：使用 AI 知识库的语义检索能力
- 拒绝原因：增加不必要的复杂性和依赖

### 3. 文档解析位置

**选择**：新建 `internal/pkg/docparse/` 共享包

**理由**：
- AI 模块的 `ai-source-extract` 任务目前只支持 txt/md，PDF/DOCX 等标记为 TODO
- 共享包可同时服务 ITSM 和 AI 模块
- 保持 CGO_ENABLED=0 兼容，使用纯 Go 库

**解析库选型**：
| 格式 | 库 | 说明 |
|------|-----|------|
| TXT/MD | 标准库 `os.ReadFile` | 直接读取 |
| PDF | `ledongthuc/pdf` | 纯 Go，无 CGO |
| DOCX | 手动 ZIP+XML 解析 | `archive/zip` + `encoding/xml` |
| XLSX | `xuri/excelize/v2` | 纯 Go，已成熟 |
| PPTX | 手动 ZIP+XML 解析 | 类似 DOCX |

### 4. 异步解析 vs 同步解析

**选择**：异步——通过 Scheduler 的 `itsm-doc-parse` 任务解析

**理由**：
- PDF/DOCX 解析可能耗时，不应阻塞上传请求
- 与现有 AI 模块的 `ai-source-extract` 模式一致
- 提供解析状态反馈（pending → processing → completed → failed）

### 5. 文件存储

**选择**：复用现有的文件上传基础设施（`/api/v1/upload`），ServiceKnowledgeDocument 记录存储路径

**理由**：
- 项目已有文件上传功能
- 无需新建存储层

## Risks / Trade-offs

- **纯 Go PDF 解析质量** → `ledongthuc/pdf` 对复杂 PDF（扫描件、加密、表格密集）支持有限。Mitigation：记录解析失败并允许重试，管理员可检查 parsed_text 是否完整
- **DOCX/PPTX 手动解析** → 自行解析 OOXML 格式可能遗漏边缘情况。Mitigation：先实现基础文本提取，后续可替换为更成熟的库
- **上下文窗口限制** → 大量文档全文注入可能超过 LLM 上下文窗口。Mitigation：设定单文档大小上限（如 1MB 原始文件），总注入文本截断保护
- **BREAKING 变更** → 移除 `knowledge_base_ids` 字段。Mitigation：该功能尚未在生产环境使用，直接移除不影响现有用户

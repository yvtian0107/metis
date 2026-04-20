## 1. 数据模型与迁移

- [x] 1.1 创建 `ai_knowledge_assets` 统一资产表（id, name, description, category, type, status, config, compile_model_id, embedding_provider_id, embedding_model_id, auto_build, source_count, timestamps），替代现有 `ai_knowledge_bases`
- [x] 1.2 创建独立 `ai_knowledge_sources` 素材表（id, title, format, content, source_url, crawl_depth, url_pattern, file_name, byte_size, extract_status, content_hash, error_message, timestamps），去掉 `kb_id` 外键
- [x] 1.3 创建 `ai_knowledge_asset_sources` 关联表（asset_id, source_id），实现 M:N 引用
- [x] 1.4 创建 `ai_rag_chunks` 表（id, asset_id, source_id, content, summary, metadata JSON, embedding vector, chunk_index, parent_chunk_id, timestamps）用于 RAG 知识库存储
- [x] 1.5 扩展 Agent 绑定：新增 `ai_agent_knowledge_graphs` 关联表，保留 `ai_agent_knowledge_bases`，两者均引用 `ai_knowledge_assets` 但按 category 过滤
- [x] 1.6 编写数据迁移脚本：将现有 `ai_knowledge_bases` 记录转为 `category=kg, type=concept_map` 的资产；将现有 `ai_knowledge_sources` 迁移到独立素材表并建立关联；FalkorDB 图名从 `kb_<id>` 重命名为 `kg_<id>`

## 2. 后端核心抽象层

- [x] 2.1 定义 `KnowledgeEngine` 接口（Build, Rebuild, Search, ContentStats）和 `RecallResult`、`KnowledgeUnit`、`KnowledgeRelation` 统一协议结构体
- [x] 2.2 实现引擎注册表：按 `category:type` 键注册和查找引擎，启动时自动注册所有可用引擎
- [x] 2.3 定义 `KnowledgeAsset` model 和 repository（GORM），支持按 category 过滤查询
- [x] 2.4 实现类型元数据注册表：每种 type 注册 display_name、description、default_config_schema、icon，提供 `GET /api/v1/ai/knowledge/types` 接口

## 3. 素材管理模块

- [x] 3.1 实现 `KnowledgeSourceService`：创建、列表、详情、删除素材，包含引用检查（被引用时不可删除）
- [x] 3.2 实现 `KnowledgeSourceRepository`：独立素材表的 GORM 持久层
- [x] 3.3 从现有 `knowledge_extract_service.go` 中剥离文档提取逻辑（PDF/DOCX/XLSX/PPTX → Markdown），改为素材池独立服务
- [x] 3.4 实现素材上传 handler：文件上传 + URL 添加 + 手工录入，异步提取任务入队
- [x] 3.5 实现素材 CRUD handler：`/api/v1/ai/knowledge/sources` 的 GET/POST/DELETE，含分页、格式过滤、状态过滤
- [x] 3.6 实现素材引用追踪：查询素材被哪些资产引用，返回引用列表

## 4. 知识图谱引擎（迁移现有能力）

- [x] 4.1 将现有 `knowledge_compile_service.go` 的 Map-Reduce 编译逻辑封装为 `ConceptMapEngine`，实现 `KnowledgeEngine` 接口
- [x] 4.2 将现有 `knowledge_compile_longdoc.go` 的长文档管线（Scan→Extract→Merge）迁入 `ConceptMapEngine`
- [x] 4.3 将现有 `knowledge_graph_repository.go` 的 FalkorDB 操作适配新模型：图名改为 `kg_<asset_id>`
- [x] 4.4 将现有 `knowledge_embedding_service.go` 的图谱嵌入逻辑适配新资产模型
- [x] 4.5 将现有编译进度追踪适配新资产模型（phases: preparing → source_reading → calling_llm → node_writing → embedding → completed）
- [x] 4.6 将现有 lint 检查（孤立节点、稀疏节点、矛盾关系）和编译日志迁入新模型
- [x] 4.7 实现图谱 CRUD handler：`/api/v1/ai/knowledge/graphs` 全套接口（CRUD + compile/recompile/progress + sources + nodes + graph + logs + search）
- [x] 4.8 实现图谱 Search：向量召回 + 1-2 hop 图扩展，返回统一 `RecallResult`（含 Relations）

## 5. RAG 知识库引擎（新建）

- [x] 5.1 实现 `NaiveChunkEngine`：按段落/固定长度切块，生成 embedding，存入 `ai_rag_chunks` 表，实现 `KnowledgeEngine` 接口
- [x] 5.2 实现 chunk 存储层：GORM repository for `ai_rag_chunks`，含 pgvector 向量列操作
- [x] 5.3 实现 RAG Search：向量检索 + 全文检索 + hybrid 模式，返回统一 `RecallResult`
- [x] 5.4 实现 RAG 构建进度追踪（phases: preparing → chunking → embedding → indexing → completed）
- [x] 5.5 实现增量构建：只处理新增素材的 chunk，不重建已有 chunk
- [x] 5.6 实现知识库 CRUD handler：`/api/v1/ai/knowledge/bases` 全套接口（CRUD + build/rebuild/progress + sources + chunks + search）

## 6. 素材-资产关联

- [x] 6.1 实现 `AssetSourceService`：管理资产与素材的 M:N 关联（添加/移除/列表）
- [x] 6.2 实现素材变更通知：素材内容变更时，标记所有引用资产为 stale 状态
- [x] 6.3 实现资产详情中的素材选择接口：从素材池中选择并关联到当前资产

## 7. Agent 绑定适配

- [x] 7.1 扩展 Agent model：新增 `knowledge_graph_ids` 字段，保留 `knowledge_base_ids`
- [x] 7.2 扩展 Agent CRUD handler：创建/更新时支持同时绑定知识库和知识图谱
- [x] 7.3 适配 Sidecar 统一检索接口：接收 Agent 绑定的所有资产 ID，按 category 路由到对应引擎，合并返回

## 8. FalkorDB 适配

- [x] 8.1 FalkorDB 图命名改为 `kg_<asset_id>`，更新所有引用点
- [x] 8.2 FalkorDB 降级处理：未配置时只禁用知识图谱功能，RAG 知识库正常可用
- [x] 8.3 确保 FalkorDB 连接断开时只影响图谱操作，不影响 RAG 操作

## 9. Embedding 适配

- [x] 9.1 拆分 embedding 服务为两条路径：图谱 embedding（FalkorDB HNSW）和 RAG embedding（pgvector）
- [x] 9.2 实现 per-asset embedding 配置：每个资产独立的 provider + model 配置
- [x] 9.3 RAG 向量索引管理：构建完成后确保 pgvector 索引存在

## 10. 前端 — 知识管理页

- [x] 10.1 创建素材管理列表页：DataTable 展示素材（名称、格式、状态、引用数、大小、更新时间），支持上传/添加URL/手工录入
- [x] 10.2 实现文件上传组件（复用现有 source-upload 组件并适配）
- [x] 10.3 实现 URL 添加表单（复用现有 url-add-form 组件并适配）
- [x] 10.4 实现素材详情：展示提取内容预览和引用资产列表
- [x] 10.5 实现状态 Badge 组件：提取状态（待提取/提取中/可用/失败）

## 11. 前端 — 知识库页

- [x] 11.1 创建知识库列表页：DataTable 展示知识库（名称、类型、状态、素材数、chunk 数、更新时间）
- [x] 11.2 创建知识库新建 Sheet 表单：名称、描述、类型选择（卡片式 + 描述）、embedding 模型配置
- [x] 11.3 创建知识库详情页骨架：概览 / 素材 / 内容 / 检索测试 / 设置 五个 Tab
- [x] 11.4 实现概览 Tab：基本信息、统计、构建进度、绑定的 Agent
- [x] 11.5 实现素材 Tab：从素材池中选择/移除素材
- [x] 11.6 实现内容 Tab：chunk 分页列表，展示内容预览、来源、索引
- [x] 11.7 实现检索测试 Tab：输入问题 → 展示 KnowledgeUnit 卡片（标题、内容、分数、来源）
- [x] 11.8 实现设置 Tab：embedding 配置、type 参数、auto-build 开关、危险操作（删除）
- [x] 11.9 实现构建进度组件：进度条 + 阶段 + 处理项，2 秒刷新

## 12. 前端 — 知识图谱页

- [x] 12.1 创建知识图谱列表页：DataTable 展示图谱（名称、类型、状态、素材数、节点数、边数、更新时间）
- [x] 12.2 创建知识图谱新建 Sheet 表单：名称、描述、类型选择（卡片式）、编译模型、embedding 模型配置
- [x] 12.3 创建知识图谱详情页骨架：概览 / 素材 / 内容 / 检索测试 / 日志 / 设置 六个 Tab
- [x] 12.4 实现概览 Tab：基本信息、节点/边统计、编译进度、绑定的 Agent
- [x] 12.5 实现素材 Tab：从素材池中选择/移除素材（复用知识库的素材选择组件）
- [x] 12.6 实现内容 Tab：复用现有 knowledge-graph-view（力导向图）和 node-table-view 组件
- [x] 12.7 实现检索测试 Tab：输入问题 → 展示 KnowledgeUnit 卡片 + 关系图谱（复用现有 recall-panel 逻辑）
- [x] 12.8 实现日志 Tab：复用现有 compile-logs-tab 组件并适配新数据结构
- [x] 12.9 实现设置 Tab：编译模型、embedding 模型、type 参数（target_content_length 等）、auto-compile 开关
- [x] 12.10 实现编译进度组件：复用知识库的构建进度组件，阶段不同

## 13. 前端 — 菜单与路由

- [x] 13.1 更新 AI 模块菜单：知识下新增三个子菜单（知识管理、知识库、知识图谱）
- [x] 13.2 配置前端路由：`/ai/knowledge/sources`、`/ai/knowledge/bases`、`/ai/knowledge/graphs` 及各详情页路由
- [x] 13.3 更新 Agent 配置页的知识绑定 UI：支持同时选择知识库和知识图谱
- [x] 13.4 删除旧的知识库页面和组件

## 14. 清理与验证

- [x] 14.1 删除旧 `ai_knowledge_bases` 相关的 handler/service/repository 文件
- [x] 14.2 删除旧前端知识库页面（`web/src/apps/ai/pages/knowledge/` 下的旧文件）
- [x] 14.3 更新 Casbin 白名单：新增知识相关 API 路径
- [x] 14.4 更新 Sidecar 路由：适配新的检索接口路径
- [x] 14.5 端到端验证：素材上传 → 关联到知识库/图谱 → 构建 → 检索测试 → Agent 绑定 → 对话中召回

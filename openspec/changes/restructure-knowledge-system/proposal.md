## Why

当前"知识库"是一个大一统单体：素材上传、图谱编译、向量召回、来源管理全部揉在一个 `KnowledgeBase` 实体里，且整个系统硬编码为图谱模式（`compileMethod = knowledge_graph`）。这导致：

1. 无法支持经典 NaiveRAG（切块 + 向量检索），只有图谱一条路
2. 素材绑死在单个知识库下，同一份文档无法同时被 RAG 和图谱消费
3. 新增一种知识加工方式需要改动大量 if/switch，扩展性差
4. 前端详情页天然等于"图谱页"，无法适配其他知识形态

需要把"知识库"拆成三个独立关注点：素材管理、NaiveRAG 知识库、知识图谱，并引入统一引擎接口支持多策略扩展。

## What Changes

- **新增"知识管理"（素材池）**：将 `KnowledgeSource` 从 KnowledgeBase 下独立出来，成为一等公民。素材与知识库/图谱之间改为 M:N 引用关系，同一份素材可被多个知识库和图谱共用
- **新增"知识库"（NaiveRAG）**：支持经典文档切块 + 向量/全文检索，内置多种切块策略（标准切块、父子切块、摘要优先、QA 抽取等），使用独立向量存储
- **重构"知识图谱"**：从现有 KnowledgeBase 中剥离，成为独立实体。保留现有 FalkorDB + Map-Reduce 能力，并支持多种图谱构建策略（概念图谱、实体图谱、轻量图谱等）
- **统一引擎接口**：定义 `KnowledgeEngine` 接口（Build / Rebuild / Search），按 `category + type` 路由到具体实现，新增策略只需实现接口 + 注册
- **BREAKING**：`ai_knowledge_bases` 表结构重建；`KnowledgeSource` 表去掉 `kb_id` 外键，改用关联表；现有 API 路径变更；前端知识相关页面全部重写
- **菜单重组**：知识下拆为"知识管理""知识库""知识图谱"三个平级入口
- **Agent 绑定扩展**：Agent 可同时绑定多个知识库和多个知识图谱，运行时按类型路由检索
- **Memory 不动**：记忆保持独立体系，不纳入本次重构

## Capabilities

### New Capabilities
- `knowledge-source-pool`: 独立素材池管理——素材上传/URL抓取/提取/预览/引用追踪，与知识库和图谱解耦
- `knowledge-base-rag`: NaiveRAG 知识库——文档切块、向量索引、全文索引、多种切块策略、检索测试
- `knowledge-graph-restructured`: 重构后的知识图谱——从 KnowledgeBase 剥离，独立实体，多种图谱构建策略
- `knowledge-engine-interface`: 统一引擎接口——Build/Rebuild/Search 的抽象层，按 category+type 路由
- `knowledge-ui-restructured`: 重构后的知识 UI——知识管理页、知识库页、知识图谱页三个独立入口及各自详情页

### Modified Capabilities
- `ai-agent`: Agent 知识绑定从单一 `AgentKnowledgeBase` 扩展为同时绑定知识库 + 知识图谱
- `knowledge-falkordb`: 降级为图谱专用存储层，不再承担通用知识存储职责
- `knowledge-embedding`: 拆分为图谱 embedding（FalkorDB HNSW）和 RAG embedding（独立向量存储）两条路径

## Impact

- **后端**：`internal/app/ai/` 下 knowledge 相关的 17 个文件需要重构或重写；新增 RAG 引擎、素材管理、引擎路由层
- **前端**：`web/src/apps/ai/pages/knowledge/` 下所有页面和组件重写；新增知识管理页、知识库列表/详情页
- **数据库**：`ai_knowledge_bases` 表结构重建；新增 `ai_knowledge_sources`（独立）、`ai_knowledge_base_sources`（M:N）、`ai_rag_chunks` 等表；需要数据迁移
- **存储依赖**：新增向量数据库依赖（pgvector / Qdrant / Milvus）用于 RAG；FalkorDB 仅服务图谱
- **API**：知识相关 API 路径全部变更；Sidecar 检索接口需要支持按类型路由
- **Agent 配置**：Agent 的知识绑定 UI 和数据结构需要适配新的多类型绑定

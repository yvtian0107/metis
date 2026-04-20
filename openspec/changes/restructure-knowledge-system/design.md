## Context

当前知识系统的全部能力集中在一个 `KnowledgeBase` 实体里，该实体同时承担素材管理、图谱编译、向量嵌入、召回检索四种职责。后端 17 个文件围绕这个单体展开，前端详情页硬编码为图谱视图。`AgentMemory` 作为独立子系统存在，不参与本次重构。

现有技术栈：Go + Gin（后端）、React + Vite（前端）、FalkorDB（图存储）、GORM（关系存储）、任务调度器（异步编译）。

## Goals / Non-Goals

**Goals:**
- 将素材管理从知识库中剥离为独立模块，支持 M:N 复用
- 新增 NaiveRAG 知识库能力（文档切块 + 向量检索），与图谱并列
- 知识库和知识图谱各自支持多种加工策略（type），创建时选定、不可变更
- 定义统一 `KnowledgeEngine` 接口，新增策略只需实现接口 + 注册
- 前端拆分为三个独立页面：知识管理、知识库、知识图谱
- Agent 可同时绑定多个知识库和多个知识图谱

**Non-Goals:**
- 不重构 Memory 系统（保持独立演进）
- 不引入 Pipeline 编排引擎（当前阶段用 Strategy 接口即可）
- 不做跨库联合检索的 UI（Agent 运行时后端自动合并即可）
- 不做素材版本管理
- 不做知识库/图谱之间的自动推荐或关联

## Decisions

### 1. 素材与知识库/图谱的关系：M:N 引用

**选择**：引入 `ai_knowledge_asset_sources` 关联表，素材与知识库/图谱之间为多对多引用关系。

**替代方案**：
- N:1（现状）：素材绑死在单个 KB 下，无法复用。排除。
- 复制模式：选择素材时复制一份到目标下。维护成本高，同一文档存多份。排除。

**理由**：用户核心诉求是"同一批文档同时建 RAG 和图谱"，引用模式最自然。素材变更时，通过关联关系通知所有引用方触发增量重建。

### 2. 统一实体表 vs 分表

**选择**：知识库和知识图谱共用一张 `ai_knowledge_assets` 表，通过 `category` 字段（`kb` / `kg`）区分。

**替代方案**：
- 分两张表 `ai_knowledge_bases` + `ai_knowledge_graphs`：字段重复度高（name、description、status、model 配置等），且 Agent 绑定需要两套关联表。排除。

**理由**：两者共享 80% 以上字段（元信息、状态机、模型配置、embedding 配置），分表带来的隔离收益不足以抵消重复成本。`category + type` 组合足以路由到正确的引擎实现。

### 3. 引擎接口设计：Strategy 模式

**选择**：定义 `KnowledgeEngine` 接口，按 `category:type` 键注册实现。

```go
type KnowledgeEngine interface {
    Build(ctx context.Context, asset *KnowledgeAsset, sources []*KnowledgeSource) error
    Rebuild(ctx context.Context, asset *KnowledgeAsset) error
    Search(ctx context.Context, asset *KnowledgeAsset, query *RecallQuery) (*RecallResult, error)
    ContentStats(ctx context.Context, asset *KnowledgeAsset) (*ContentStats, error)
}
```

**替代方案**：
- Pipeline 编排（DAG 式 step 组合）：灵活但过度设计，当前只有 3-4 种策略。排除。
- 大 switch/case：不可扩展。排除。

**理由**：Strategy 模式在当前规模下最务实。新增一种策略 = 实现接口 + 注册一行代码 + 前端加一个选项卡片。

### 4. RAG 向量存储：pgvector

**选择**：NaiveRAG 的 chunk 向量使用 pgvector（PostgreSQL 扩展），不使用 FalkorDB。

**替代方案**：
- FalkorDB 统一存储：FalkorDB 的向量能力是为图节点设计的，不适合大量平铺 chunk。排除。
- 独立向量数据库（Qdrant/Milvus）：引入额外运维依赖。如果用户已有 PostgreSQL，pgvector 零额外依赖。排除（当前阶段）。

**理由**：项目已依赖 PostgreSQL/MySQL，pgvector 是最低成本的向量存储方案。未来如果需要更高性能，可以通过引擎接口替换实现，不影响上层。

### 5. 素材提取：复用现有能力

**选择**：`knowledge_extract_service.go` 的文档提取逻辑（PDF/DOCX/XLSX/PPTX → Markdown）直接复用，从 KnowledgeBase 上下文中解耦，改为素材池的独立服务。

**理由**：提取逻辑与加工方式无关，属于素材层能力。

### 6. 统一检索返回协议

**选择**：所有引擎的 Search 返回统一 `RecallResult` 结构：

```go
type RecallResult struct {
    Items     []KnowledgeUnit    // 统一知识单元
    Relations []KnowledgeRelation // 可选，仅图谱返回
    Sources   []SourceRef         // 引用原文
    Debug     *RecallDebug        // 可选，调试信息
}
```

**理由**：上层（Agent prompt 注入、前端展示）不应该关心底层是 chunk 还是 node。图谱只是比 RAG 多一个 `Relations` 字段。

### 7. 前端菜单与页面结构

**选择**：

```
知识
├─ 知识管理    → 素材池列表页
├─ 知识库      → RAG 知识库列表页
└─ 知识图谱    → 图谱列表页
```

每个详情页统一 tab 结构：概览 / 素材 / 内容 / 检索测试 / 设置。
"内容" tab 按类型渲染不同视图（chunk 列表 vs 图谱可视化）。

**替代方案**：
- 合并为一个"知识资产"列表：详情页内部差异太大（chunk vs 图谱），用户心智模型也不同。排除。

### 8. 创建时选定类型，不可变更

**选择**：知识库/图谱创建时选择 `type`（如 `naive_chunk` / `concept_map`），创建后不可更改。

**理由**：不同 type 的底层数据结构不同（chunk 表 vs 图节点），运行时切换需要全量迁移，成本远高于新建一个。

## Risks / Trade-offs

- **数据迁移风险**：现有 `ai_knowledge_bases` 数据需要迁移到新表结构，现有图谱数据需要保留。→ 提供迁移脚本，将现有记录转为 `category=kg, type=concept_map`。
- **pgvector 依赖**：MySQL 用户无法使用 pgvector。→ 先以 pgvector 为默认实现；通过引擎接口预留替换能力，未来可支持其他向量存储。
- **前端工作量大**：三个页面 + 详情页重写。→ 知识图谱详情页大量复用现有图谱组件（图谱可视化、节点表、召回面板），降低重写量。
- **API 断裂**：所有知识相关 API 路径变更。→ Sidecar 是内部通信，版本锁定；管理端 API 前后端同步更新。
- **Type 扩展的前端成本**：每新增一种 type 需要前端加选项卡片。→ type 元数据走后端接口返回（名称、描述、图标），前端只需渲染列表。

## Migration Plan

1. **Phase 0 — 新表结构**：创建 `ai_knowledge_assets`、`ai_knowledge_asset_sources`、`ai_rag_chunks` 等新表
2. **Phase 1 — 数据迁移**：将现有 `ai_knowledge_bases` 迁移为 `category=kg, type=concept_map` 的资产；现有 `ai_knowledge_sources` 迁移到独立素材表并建立关联
3. **Phase 2 — 后端重构**：实现引擎接口、素材管理服务、RAG 引擎；图谱引擎包装现有编译逻辑
4. **Phase 3 — 前端重写**：知识管理页、知识库页、知识图谱页
5. **Phase 4 — Agent 适配**：扩展 Agent 绑定支持知识库 + 图谱；Sidecar 检索接口适配
6. **Phase 5 — 清理**：删除旧表、旧 API、旧前端页面

回滚策略：Phase 0-1 通过 migration down 回滚；Phase 2-4 通过 feature flag 控制新旧路由切换。

## Open Questions

1. **pgvector vs 其他向量存储**：当前选择 pgvector 是否满足性能需求？是否需要预留 Qdrant/Milvus 适配？
2. **素材变更通知机制**：素材更新后，是自动触发所有引用方重建，还是只通知（让用户手动触发）？
3. **Type 元数据管理**：策略类型是硬编码在代码里，还是做成可注册的配置？
4. **图谱编译的长文档管线**：现有 `knowledge_compile_longdoc.go` 的 Scan→Extract→Merge 三阶段是否在重构中保持不变？

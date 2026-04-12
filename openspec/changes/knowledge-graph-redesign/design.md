## Context

当前知识图谱编译系统采用 Map-Reduce 架构：MAP 阶段逐来源提取概念节点，REDUCE 阶段合并并处理增量更新。核心问题：

1. **空壳节点泛滥** — Prompt 鼓励 `content: null` 节点，边解析自动创建无内容幽灵节点
2. **长文档截断** — MAP 阶段 8000 字硬截断，PDF 书籍等长来源的后半部分知识全部丢失
3. **边类型过细** — 4 种边类型（related/contradicts/extends/part_of）LLM 难以准确区分，extends 和 part_of 几乎无法可靠标注
4. **index 节点污染图谱** — 元数据节点混在知识节点中

存储层使用 FalkorDB（图数据库），节点和边通过 Cypher 查询操作。模型层定义在 `knowledge_model.go`，编译核心在 `knowledge_compile_service.go`。

## Goals / Non-Goals

**Goals:**
- 确保每个知识节点都是有实质内容的、可独立阅读的 Wiki 文章
- 支持 PDF 书籍等超长来源的完整知识提取（不丢信息）
- 简化边类型，提高 LLM 标注准确率
- 提供可配置的编译参数（内容长度等）

**Non-Goals:**
- 不改变 FalkorDB 存储架构
- 不修改来源提取流程（ai-source-extract）
- 不增加 BM25/混合检索/reranking（不是 RAG 系统）
- 不自动迁移/清理已有空壳节点（下次重编译自然覆盖）
- 不修改知识库 CRUD API

## Decisions

### D1: 节点内容强制非空

**决策**：content 字段从 `*string`（nullable）改为 `string`（必填），编译 prompt 明确禁止空内容节点。

**替代方案**：保留 nullable 但在写入时过滤 — 不够彻底，LLM 仍会浪费 token 输出空节点。

**理由**：从源头（prompt）和终点（写入校验）双重保证。LLM 的 token 应该花在写实质文章上，而不是产出空壳。

**影响**：
- `KnowledgeNode.Content` 类型从 `*string` 改为 `string`
- `mapNodeOutput.Content` 和 `compileNodeOutput.Content` 同步改为 `string`
- `writeCompileOutput` 增加防御性检查：`if len(n.Content) < minContentLength { skip }`
- FalkorDB Cypher 查询中 `CASE WHEN $content IS NOT NULL` 逻辑简化

### D2: 消灭幽灵节点

**决策**：边解析时，目标节点不存在则跳过该边，不创建空壳节点。

**当前行为**（`knowledge_compile_service.go:667-678`）：
```go
if _, err := s.graphRepo.FindNodeByTitle(kbID, rel.Concept); err != nil {
    emptyNode := &KnowledgeNode{Title: rel.Concept, ...}
    s.graphRepo.UpsertNodeByTitle(kbID, emptyNode)
}
```

**新行为**：
```go
if _, err := s.graphRepo.FindNodeByTitle(kbID, ref); err != nil {
    continue // 跳过，不创建幽灵节点
}
```

**理由**：幽灵节点是图谱质量的最大杀手。如果一个概念重要到需要在图中存在，它应该由编译 prompt 作为完整节点输出，而不是作为边的副产品被自动创建。

### D3: 边类型简化为 2 种

**决策**：只保留 `related` 和 `contradicts`。

**替代方案**：保留 4 种 — LLM 对 extends/part_of 的标注准确率低，用户也不按这些类型过滤。

**理由**：`contradicts` 有独特价值（高亮知识冲突），`related` 覆盖所有其他关系。减少边类型让 LLM 的判断更可靠。层级关系（"X 是 Y 的子概念"）通过节点 content 中的文本描述表达。

**影响**：
- 移除 `EdgeRelationExtends` 和 `EdgeRelationPartOf` 常量
- Prompt 中只列 2 种关系类型
- 前端图谱视图移除 extends/part_of 的颜色区分，统一为 related 样式
- 已有的 extends/part_of 边在查询时映射为 related

### D4: 移除 index 节点

**决策**：不再在图中生成 `NodeTypeIndex` 类型节点。

**理由**：
- index 节点的导航功能已由 `analyzeCascadeImpact` 程序化实现
- 节点列表已有前端 UI（知识节点列表页）
- 改良后所有节点都有内容，"有无内容"标记失去意义
- index 节点混在知识节点中污染图谱拓扑

**影响**：
- 移除 `generateIndexNode` 方法
- 移除 `NodeTypeIndex` 常量
- `KnowledgeNode.NodeType` 只剩 `"concept"` 一种值（保留字段以备将来扩展）
- 前端图谱视图移除 index 节点的特殊渲染

### D5: LLM 输出结构简化

**决策**：将 `related` 数组（包含 concept + relation）简化为两个独立列表。

**新的 LLM 输出格式**：
```json
{
  "nodes": [
    {
      "title": "Concept Name",
      "summary": "One-line description",
      "content": "完整 Wiki 文章（必填）",
      "references": ["Other Concept A", "Other Concept B"],
      "contradicts": ["Conflicting Concept"],
      "sources": ["Source Title"]
    }
  ],
  "updated_nodes": [...]
}
```

**理由**：`references` 是纯字符串列表（默认创建 related 边），`contradicts` 是单独列表（创建 contradicts 边）。比原来的 `[{concept, relation}]` 结构更简洁，LLM 更不容易输出格式错误。

### D6: 长文档三阶段处理（Scan → Gather → Write）

**决策**：来源内容超过单次 LLM 上下文窗口 40% 时，启用三阶段处理。

**阶段说明**：
1. **CHUNK** — 按自然边界（章节标题 > 段落 > 固定长度）切分，每块 ≤ maxChunkSize
2. **SCAN** — 逐块轻量提取（只输出 title + summary，可并行），建立 concept → chunk_indices 映射
3. **MERGE** — 合并去重所有 chunk 的概念列表
4. **GATHER** — 对每个概念，从原文中收集所有相关段落组成 evidence bundle
5. **WRITE** — 对每个概念，用 evidence bundle（原文！）写完整文章（可并行）

**替代方案 A**：滑动窗口摘要 — 信息逐步衰减，第 N 次压缩后早期细节丢失殆尽。
**替代方案 B**：层级 Map-Reduce — 每层压缩仍有信息损失，且跨章节概念可能在两边的摘要里都被压掉。

**方案 C（采用）的优势**：WRITE 阶段 LLM 看到的是原文段落而非被压缩的摘要，信息无损。SCAN 和 WRITE 阶段均可并行。

**evidence bundle 超限降级**：按概念在段落中的出现密度排序，取 top-K 段落控制在 maxChunkSize 内，被截段落保留首尾各 2 句作为 snippet。

### D7: 编译配置项

**决策**：在 `KnowledgeBase` 上新增可选的 `CompileConfig` JSON 字段。

```go
type CompileConfig struct {
    TargetContentLength int `json:"targetContentLength"` // 目标内容长度（字符），默认 4000
    MinContentLength    int `json:"minContentLength"`    // 最小内容长度，低于此值不创建节点，默认 200
    MaxChunkSize        int `json:"maxChunkSize"`        // 长文档分块大小，0=自动（模型窗口×40%），默认 0
}
```

通过已有的 `PUT /api/v1/ai/knowledge-bases/:id` 接口传递，无需新增 API。

## Risks / Trade-offs

**[节点数量减少]** → 强制有内容会让节点总数显著减少（可能减少 50-70%），图谱看起来"小"了。
→ 缓解：这是预期效果。少而精的节点比多而空的节点更有价值。用户通过节点内容质量而非数量感知价值。

**[长文档 LLM 调用量增加]** → 三阶段处理的总调用量 = N_chunks + N_concepts，比单次调用多。
→ 缓解：SCAN 阶段输出极短（只要 title + summary），成本低。SCAN 和 WRITE 均可并行，总延迟可控。

**[已有空壳节点的处理]** → 不做自动清理，重编译时自然覆盖。
→ 风险：用户在重编译前仍会看到空壳节点。可接受，因为编译是用户主动触发的操作。

**[Content 类型变更]** → `*string` 改 `string` 影响 FalkorDB 读写和 API 序列化。
→ 缓解：FalkorDB 中已存的 null/空 content 在读取时映射为空字符串。API response 的 `hasContent` 字段保留用于前端向后兼容。

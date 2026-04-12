export type CompileStatus = "idle" | "compiling" | "completed" | "error"
export type CompileMethod = "knowledge_graph"
export type ExtractStatus = "pending" | "completed" | "error"
export type CompileStage = "preparing" | "calling_llm" | "writing_nodes" | "generating_embeddings" | "completed" | "idle"

export interface ProgressCounter {
  total: number
  done: number
}

export interface CompileProgress {
  stage: CompileStage
  sources: ProgressCounter
  nodes: ProgressCounter
  embeddings: ProgressCounter
  currentItem: string
}

export interface KnowledgeBaseDetail {
  id: number
  name: string
  description: string
  sourceCount: number
  nodeCount: number
  edgeCount: number
  compileStatus: CompileStatus
  compileMethod: CompileMethod
  compileModelId: number
  embeddingProviderId: number | null
  embeddingModelId: string
  autoCompile: boolean
  createdAt: string
  updatedAt: string
}

export interface SourceItem {
  id: number
  title: string
  format: string
  extractStatus: ExtractStatus
  byteSize: number
  sourceType: string
  createdAt: string
}

export interface NodeItem {
  id: string
  title: string
  summary: string
  nodeType: string
  hasContent: boolean
  edgeCount: number
  content?: string
  sourceIds?: number[]
  score?: number
}

export interface EdgeItem {
  fromNodeId: string
  toNodeId: string
  relation: string
  description?: string
}

export interface GraphResponse {
  nodes: NodeItem[]
  edges: EdgeItem[]
}

export interface CascadeDetail {
  nodeTitle: string
  updateType: "content" | "relationship" | "contradiction" | "merge"
  reason: string
  sourcesAdded?: number[]
}

export interface CascadeLog {
  primaryNodes: string[]
  cascadeUpdates: CascadeDetail[]
}

export interface LogItem {
  id: number
  action: string
  modelId: string
  nodesCreated: number
  nodesUpdated: number
  edgesCreated: number
  lintIssues: number
  errorMessage: string
  cascadeDetails?: CascadeLog
  createdAt: string
}

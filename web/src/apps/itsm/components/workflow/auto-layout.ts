import dagre from "@dagrejs/dagre"
import { Position, type Node, type Edge } from "@xyflow/react"
import { type NodeType, WORKFLOW_NODE_DIMENSIONS } from "./types"

export type WorkflowLayoutDirection = "TB" | "LR"

interface LayoutOptions {
  direction?: WorkflowLayoutDirection
  nodesep?: number
  ranksep?: number
  marginx?: number
  marginy?: number
}

const DEFAULT_LAYOUT_OPTIONS: Required<LayoutOptions> = {
  direction: "LR",
  nodesep: 44,
  ranksep: 92,
  marginx: 32,
  marginy: 32,
}

const VIEWER_LAYOUT_OPTIONS: Required<LayoutOptions> = {
  direction: "LR",
  nodesep: 96,
  ranksep: 132,
  marginx: 64,
  marginy: 64,
}

function workflowNodeType(node: Node): NodeType {
  return ((node.data as { nodeType?: NodeType } | undefined)?.nodeType ?? node.type ?? "form") as NodeType
}

function workflowNodeDimensions(node: Node) {
  const nodeType = workflowNodeType(node)
  return WORKFLOW_NODE_DIMENSIONS[nodeType] ?? { width: 240, height: 96 }
}

function layoutPositions(direction: WorkflowLayoutDirection) {
  return direction === "TB"
    ? { sourcePosition: Position.Bottom, targetPosition: Position.Top }
    : { sourcePosition: Position.Right, targetPosition: Position.Left }
}

export function applyWorkflowAutoLayout(
  nodes: Node[],
  edges: Edge[],
  options: LayoutOptions = {},
): Node[] {
  const { direction, nodesep, ranksep, marginx, marginy } = { ...DEFAULT_LAYOUT_OPTIONS, ...options }
  const positions = layoutPositions(direction)
  const g = new dagre.graphlib.Graph()
  g.setDefaultEdgeLabel(() => ({}))
  g.setGraph({
    rankdir: direction,
    nodesep,
    ranksep,
    marginx,
    marginy,
    ranker: "network-simplex",
  })

  for (const node of nodes) {
    const dim = workflowNodeDimensions(node)
    g.setNode(node.id, { width: dim.width, height: dim.height })
  }

  for (const edge of [...edges].sort((a, b) => a.id.localeCompare(b.id))) {
    g.setEdge(edge.source, edge.target)
  }

  dagre.layout(g)

  return nodes.map((node) => {
    const pos = g.node(node.id)
    const dim = workflowNodeDimensions(node)
    return {
      ...node,
      ...positions,
      position: {
        x: pos.x - dim.width / 2,
        y: pos.y - dim.height / 2,
      },
      data: {
        ...(node.data as Record<string, unknown> | undefined),
        _layoutDirection: direction,
      },
    }
  })
}

export function applyDagreLayout(
  nodes: Node[],
  edges: Edge[],
  direction: WorkflowLayoutDirection = "LR",
  options: Omit<LayoutOptions, "direction"> = {},
): Node[] {
  return applyWorkflowAutoLayout(nodes, edges, { ...options, direction })
}

export function applyViewerLayout(nodes: Node[], edges: Edge[]): Node[] {
  return applyWorkflowAutoLayout(nodes, edges, VIEWER_LAYOUT_OPTIONS)
}

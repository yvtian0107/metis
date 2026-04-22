import dagre from "@dagrejs/dagre"
import type { Node, Edge } from "@xyflow/react"
import { type NodeType, WORKFLOW_NODE_DIMENSIONS } from "./types"

export function applyDagreLayout(
  nodes: Node[],
  edges: Edge[],
  direction: "TB" | "LR" = "LR",
): Node[] {
  const g = new dagre.graphlib.Graph()
  g.setDefaultEdgeLabel(() => ({}))
  g.setGraph({ rankdir: direction, nodesep: 44, ranksep: 92, marginx: 32, marginy: 32 })

  for (const node of nodes) {
    const nodeType = (node.data as { nodeType?: NodeType }).nodeType ?? "form"
    const dim = WORKFLOW_NODE_DIMENSIONS[nodeType] ?? { width: 240, height: 96 }
    g.setNode(node.id, { width: dim.width, height: dim.height })
  }

  for (const edge of edges) {
    g.setEdge(edge.source, edge.target)
  }

  dagre.layout(g)

  return nodes.map((node) => {
    const pos = g.node(node.id)
    const nodeType = (node.data as { nodeType?: NodeType }).nodeType ?? "form"
    const dim = WORKFLOW_NODE_DIMENSIONS[nodeType] ?? { width: 240, height: 96 }
    return {
      ...node,
      position: {
        x: pos.x - dim.width / 2,
        y: pos.y - dim.height / 2,
      },
    }
  })
}

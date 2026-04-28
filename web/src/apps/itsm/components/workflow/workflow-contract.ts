import { MarkerType, type Edge, type Node } from "@xyflow/react"
import type { NodeType, WFEdgeData, WFNodeData } from "./types"

let nodeId = 0

export function getWorkflowNodeId() {
  return `node_${Date.now()}_${++nodeId}`
}

export function defaultNodeData(nodeType: NodeType, label: string): WFNodeData {
  return {
    label,
    nodeType,
    ...(nodeType === "wait" || nodeType === "timer" ? { wait_mode: nodeType === "timer" ? "timer" : "signal" } : {}),
    ...(nodeType === "parallel" || nodeType === "inclusive" ? { gateway_direction: "fork" } : {}),
  }
}

export function defaultWorkflowData(t: (key: string) => string): { nodes: Node[]; edges: Edge[] } {
  return {
    nodes: [
      {
        id: "start",
        type: "start",
        position: { x: 0, y: 120 },
        data: defaultNodeData("start", t("workflow.node.start")) as unknown as Record<string, unknown>,
      },
      {
        id: "end",
        type: "end",
        position: { x: 360, y: 120 },
        data: defaultNodeData("end", t("workflow.node.end")) as unknown as Record<string, unknown>,
      },
    ],
    edges: [
      {
        id: "edge_start_end",
        source: "start",
        target: "end",
        type: "workflow",
        markerEnd: { type: MarkerType.ArrowClosed },
        data: { outcome: "completed", default: true } satisfies WFEdgeData as Record<string, unknown>,
      },
    ],
  }
}

export function normalizeWorkflowData(data: { nodes?: Node[]; edges?: Edge[] } | undefined): { nodes: Node[]; edges: Edge[] } | undefined {
  if (!data?.nodes?.length) return undefined

  const nodes = data.nodes.map((node) => {
    const nodeData = (node.data ?? {}) as unknown as WFNodeData
    const nodeType = nodeData.nodeType ?? (node.type as NodeType)
    return {
      ...node,
      type: nodeType,
      data: {
        ...nodeData,
        nodeType,
        ...(nodeType === "parallel" || nodeType === "inclusive" ? { gateway_direction: nodeData.gateway_direction ?? "fork" } : {}),
      } as unknown as Record<string, unknown>,
    }
  })

  const edges = (data.edges ?? []).map((edge) => {
    const edgeData = (edge.data ?? {}) as unknown as WFEdgeData
    return {
      ...edge,
      type: "workflow",
      markerEnd: { type: MarkerType.ArrowClosed },
      data: {
        ...edgeData,
        default: edgeData.default ?? edgeData.isDefault ?? false,
        isDefault: undefined,
      } as unknown as Record<string, unknown>,
    }
  })

  return { nodes, edges }
}

export function collectDraftIssues(nodes: Node[], edges: Edge[]) {
  const issues: Array<{ nodeId?: string; edgeId?: string; message: string }> = []
  const startNodes = nodes.filter((node) => ((node.data ?? {}) as unknown as WFNodeData).nodeType === "start" || node.type === "start")
  const endNodes = nodes.filter((node) => ((node.data ?? {}) as unknown as WFNodeData).nodeType === "end" || node.type === "end")
  const nodeIds = new Set(nodes.map((node) => node.id))
  const incoming = new Map<string, number>()
  const outgoing = new Map<string, Edge[]>()

  for (const edge of edges) {
    if (!nodeIds.has(edge.source) || !nodeIds.has(edge.target)) {
      issues.push({ edgeId: edge.id, message: "连线引用了不存在的节点" })
      continue
    }
    incoming.set(edge.target, (incoming.get(edge.target) ?? 0) + 1)
    const list = outgoing.get(edge.source) ?? []
    list.push(edge)
    outgoing.set(edge.source, list)
  }

  if (startNodes.length !== 1) issues.push({ message: "工作流必须包含一个开始节点" })
  if (endNodes.length < 1) issues.push({ message: "工作流必须包含结束节点" })

  for (const node of nodes) {
    const data = (node.data ?? {}) as unknown as WFNodeData
    const nodeType = data.nodeType ?? (node.type as NodeType)
    if (nodeType !== "start" && (incoming.get(node.id) ?? 0) === 0) {
      issues.push({ nodeId: node.id, message: "节点不可达" })
    }
    if ((nodeType === "form" || nodeType === "process") && !data.participants?.length) {
      issues.push({ nodeId: node.id, message: "人工节点缺少参与人" })
    }
    if ((nodeType === "parallel" || nodeType === "inclusive") && data.gateway_direction !== "fork" && data.gateway_direction !== "join") {
      issues.push({ nodeId: node.id, message: "网关缺少方向" })
    }
    if (nodeType === "action" && !data.action_id) {
      issues.push({ nodeId: node.id, message: "动作节点缺少 action_id" })
    }
    if (nodeType === "script" && !data.assignments?.some((item) => item.variable && item.expression)) {
      issues.push({ nodeId: node.id, message: "脚本节点缺少变量赋值" })
    }
    if (nodeType === "subprocess" && !data.subprocess_def) {
      issues.push({ nodeId: node.id, message: "子流程节点缺少 subprocess_def" })
    }
    if (nodeType === "wait" && data.wait_mode === "timer" && !data.duration) {
      issues.push({ nodeId: node.id, message: "定时等待缺少 duration" })
    }
    if (nodeType === "exclusive") {
      const out = outgoing.get(node.id) ?? []
      if (out.length < 2) {
        issues.push({ nodeId: node.id, message: "排他网关至少需要两条出边" })
      }
      for (const edge of out) {
        const edgeData = (edge.data ?? {}) as WFEdgeData
        if (!(edgeData.default ?? edgeData.isDefault) && !edgeData.condition) {
          issues.push({ nodeId: node.id, edgeId: edge.id, message: "分支连线缺少条件" })
        }
      }
    }
  }

  if (startNodes[0] && (outgoing.get(startNodes[0].id)?.length ?? 0) !== 1) {
    issues.push({ nodeId: startNodes[0].id, message: "开始节点必须有一条出边" })
  }

  return issues
}

export function prepareWorkflowForSave(nodes: Node[], edges: Edge[]): { nodes: Node[]; edges: Edge[] } {
  const cleanNodes = nodes.map((node) => ({
    ...node,
    data: stripUndefined({
      ...(node.data as Record<string, unknown>),
      _workflowState: undefined,
      _layoutDirection: undefined,
    }),
  }))

  const cleanEdges = edges.map((edge) => {
    const data = { ...((edge.data ?? {}) as Record<string, unknown>) }
    const isDefault = data.isDefault as boolean | undefined
    delete data.readonly
    delete data.visited
    delete data.failed
    delete data.isDefault
    return {
      ...edge,
      data: stripUndefined({
        ...data,
        default: (data.default as boolean | undefined) ?? isDefault ?? false,
      }),
    }
  })

  return { nodes: cleanNodes, edges: cleanEdges }
}

function stripUndefined<T extends Record<string, unknown>>(value: T): T {
  return Object.fromEntries(Object.entries(value).filter(([, v]) => v !== undefined)) as T
}

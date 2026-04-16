import type { NodeType } from "../types"
import { EventNode } from "./event-node"
import { TaskNode } from "./task-node"
import { GatewayNode } from "./gateway-node"
import { SubprocessNode } from "./subprocess-node"

const EVENT_TYPES = new Set<NodeType>(["start", "end", "timer", "signal"])
const TASK_TYPES = new Set<NodeType>(["form", "approve", "process", "action", "script", "notify"])
const GATEWAY_TYPES = new Set<NodeType>(["exclusive", "parallel", "inclusive"])

function resolveNodeComponent(nodeType: NodeType) {
  if (EVENT_TYPES.has(nodeType)) return EventNode
  if (TASK_TYPES.has(nodeType)) return TaskNode
  if (GATEWAY_TYPES.has(nodeType)) return GatewayNode
  if (nodeType === "subprocess") return SubprocessNode
  // wait and any unknown types fallback to task
  return TaskNode
}

// Build nodeTypes map: each NodeType gets its own entry mapping to the right renderer
// eslint-disable-next-line @typescript-eslint/no-explicit-any
export const nodeTypes: Record<string, React.ComponentType<any>> = {}
for (const nt of ["start", "end", "timer", "signal", "form", "approve", "process", "action", "script", "notify", "exclusive", "parallel", "inclusive", "subprocess", "wait"] as NodeType[]) {
  nodeTypes[nt] = resolveNodeComponent(nt)
}
// Legacy: "workflow" type maps to TaskNode for backward compat
nodeTypes["workflow"] = TaskNode

export { EventNode, TaskNode, GatewayNode, SubprocessNode }

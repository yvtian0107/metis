// Shared types for workflow editor and viewer
export const NODE_TYPES = [
  "start", "end", "form", "process", "action", "exclusive", "notify", "wait",
  "timer", "signal", "parallel", "inclusive", "subprocess", "script",
] as const

export type NodeType = (typeof NODE_TYPES)[number]

export interface Participant {
  type: string // "user" | "position" | "department" | "position_department" | "requester_manager"
  id?: string | number
  name?: string
  value?: string
  // position_department fields (LLM output)
  department_code?: string
  position_code?: string
}

export interface GatewayCondition {
  field: string
  operator: "equals" | "not_equals" | "contains_any" | "gt" | "lt" | "gte" | "lte" | "is_empty" | "is_not_empty"
  value: unknown
}

export interface SimpleCondition {
  field: string
  operator: GatewayCondition["operator"]
  value: unknown
}

export interface ConditionGroup {
  logic: "and" | "or"
  conditions: Array<SimpleCondition | ConditionGroup>
}

export interface VariableMapping {
  source: string
  target: string
}

export interface ScriptAssignment {
  variable: string
  expression: string
}

export interface WFNodeData {
  label: string
  nodeType: NodeType
  _workflowState?: "active" | "completed" | "failed" | "cancelled" | "idle"
  // form / process
  participants?: Participant[]
  formSchema?: unknown
  // process
  executionMode?: "single" | "parallel" | "sequential"
  // parallel / inclusive gateway
  gateway_direction?: "fork" | "join"
  // action
  action_id?: number
  // exclusive gateway
  // (conditions are on edges)
  // notify
  channel_id?: number
  template?: string
  // wait / timer
  wait_mode?: "signal" | "timer"
  duration?: string // e.g. "PT1H"
  // variable mapping
  inputMapping?: VariableMapping[]
  outputMapping?: VariableMapping[]
  // script
  assignments?: ScriptAssignment[]
  // subprocess
  subprocess_def?: unknown
  subprocessExpanded?: boolean
}

export interface WFEdgeData {
  outcome?: string
  default?: boolean
  isDefault?: boolean
  condition?: GatewayCondition | ConditionGroup
  readonly?: boolean
  visited?: boolean
  failed?: boolean
}

export const WORKFLOW_NODE_DIMENSIONS: Record<NodeType, { width: number; height: number }> = {
  start: { width: 240, height: 76 },
  end: { width: 240, height: 76 },
  timer: { width: 240, height: 76 },
  signal: { width: 240, height: 76 },
  form: { width: 240, height: 96 },
  process: { width: 240, height: 96 },
  action: { width: 240, height: 96 },
  script: { width: 240, height: 96 },
  notify: { width: 240, height: 96 },
  exclusive: { width: 240, height: 84 },
  parallel: { width: 240, height: 84 },
  inclusive: { width: 240, height: 84 },
  subprocess: { width: 260, height: 126 },
  wait: { width: 240, height: 84 },
}

export const NODE_COLORS: Record<NodeType, string> = {
  start: "#2563eb",
  end: "#dc2626",
  form: "#2563eb",
  process: "#4f46e5",
  action: "#0891b2",
  exclusive: "#ea580c",
  notify: "#be185d",
  wait: "#4f46e5",
  timer: "#7c3aed",
  signal: "#0f766e",
  parallel: "#0f766e",
  inclusive: "#ca8a04",
  subprocess: "#475569",
  script: "#334155",
}

// Helpers for condition format migration
export function isConditionGroup(c: GatewayCondition | ConditionGroup | undefined): c is ConditionGroup {
  return !!c && "logic" in c
}

export function toConditionGroup(c: GatewayCondition | ConditionGroup | undefined): ConditionGroup | undefined {
  if (!c) return undefined
  if (isConditionGroup(c)) return c
  return { logic: "and", conditions: [{ field: c.field, operator: c.operator, value: c.value }] }
}

const OP_LABELS: Record<string, string> = {
  equals: "=", not_equals: "≠", contains_any: "∈",
  gt: ">", lt: "<", gte: "≥", lte: "≤",
  is_empty: "为空", is_not_empty: "非空",
}

function shortField(f: string): string {
  return f.replace(/^form\./, "")
}

function shortValue(v: unknown): string {
  const s = String(v ?? "")
  return s.length > 16 ? `${s.slice(0, 16)}…` : s
}

function formatSingleCondition(c: { field: string; operator: string; value: unknown }): string {
  const op = OP_LABELS[c.operator] ?? c.operator
  if (c.operator === "is_empty" || c.operator === "is_not_empty") {
    return `${shortField(c.field)} ${op}`
  }
  return `${shortField(c.field)} ${op} ${shortValue(c.value)}`
}

export function conditionSummary(c: GatewayCondition | ConditionGroup | undefined): string {
  if (!c) return ""
  if (!isConditionGroup(c)) return formatSingleCondition(c)
  return c.conditions
    .map((item) => isConditionGroup(item) ? `(${conditionSummary(item)})` : formatSingleCondition(item))
    .join(c.logic === "and" ? " 且 " : " 或 ")
}

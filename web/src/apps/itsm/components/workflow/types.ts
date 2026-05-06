import { type EditorNodeType } from "../../contract"
import type { FormSchema } from "../form-engine"
import type { Edge, Node } from "@xyflow/react"

// Shared types for workflow editor and viewer
export const NODE_TYPES = [
  "start", "end", "form", "process", "action", "exclusive", "notify", "wait",
  "timer", "signal", "parallel", "inclusive", "subprocess", "script",
] as const satisfies readonly EditorNodeType[]

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
  _layoutDirection?: "TB" | "LR"
  _workflowState?: "active" | "completed" | "failed" | "cancelled" | "idle"
  // form / process
  participants?: Participant[]
  formSchema?: FormSchema
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
  subprocess_def?: WorkflowData
  subprocessExpanded?: boolean
}

export interface WorkflowNode {
  id: string
  type?: NodeType
  position?: { x: number; y: number }
  data?: WFNodeData
}

export interface WorkflowEdge {
  id: string
  source: string
  target: string
  data?: WFEdgeData
}

export interface WorkflowData {
  nodes: Node[]
  edges: Edge[]
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

export interface ConditionDisplayDict {
  fieldLabels: Record<string, string>
  valueLabels: Record<string, string>
}

export interface ConditionDisplayLocale {
  operatorLabels: Record<GatewayCondition["operator"], string>
  logicLabels: { and: string; or: string }
  valueSeparator: string
}

export interface EdgeLabelLocale {
  defaultLabel: string
  outcomeLabels: Record<string, string>
}

export interface EdgeLabelDisplay {
  primary: string
  raw: string
  isCompleted: boolean
  kind: "outcome" | "condition" | "default" | "empty"
}

type WorkflowNodeLike = { data?: Record<string, unknown> }

type FormFieldLike = {
  key?: unknown
  label?: unknown
  options?: unknown
}

function asString(value: unknown): string {
  return String(value ?? "")
}

function normalizeFieldKey(field: string): string {
  return field.replace(/^form\./, "")
}

function toValueList(value: unknown): string[] {
  if (Array.isArray(value)) return value.map((item) => asString(item))
  return [asString(value)]
}

function normalizeOption(option: unknown): { label: string; value: string } | null {
  if (option && typeof option === "object") {
    const typed = option as { label?: unknown; value?: unknown }
    if (typed.label !== undefined && typed.value !== undefined) {
      return { label: asString(typed.label), value: asString(typed.value) }
    }
  }
  if (typeof option === "string" || typeof option === "number" || typeof option === "boolean") {
    const text = asString(option)
    return { label: text, value: text }
  }
  return null
}

function mapSingleConditionForDisplay(
  condition: { field: string; operator: GatewayCondition["operator"]; value: unknown },
  dict: ConditionDisplayDict,
  locale: ConditionDisplayLocale,
): string {
  const normalizedField = normalizeFieldKey(condition.field)
  const fieldLabel = dict.fieldLabels[normalizedField] ?? normalizedField
  const operatorLabel = locale.operatorLabels[condition.operator] ?? condition.operator
  if (condition.operator === "is_empty" || condition.operator === "is_not_empty") {
    return `${fieldLabel} ${operatorLabel}`
  }
  const mappedValues = toValueList(condition.value).map((value) => dict.valueLabels[value] ?? value)
  const joinedValue = mappedValues.join(locale.valueSeparator)
  return `${fieldLabel} ${operatorLabel} ${joinedValue}`
}

export function createEmptyConditionDisplayDict(): ConditionDisplayDict {
  return { fieldLabels: {}, valueLabels: {} }
}

export function buildConditionDisplayDictFromNodes(nodes: WorkflowNodeLike[]): ConditionDisplayDict {
  const fieldLabels: Record<string, string> = {}
  const valueLabels: Record<string, string> = {}
  for (const node of nodes) {
    const nodeData = node.data as WFNodeData | undefined
    const rawSchema = nodeData?.formSchema
    if (!rawSchema || typeof rawSchema !== "object") continue
    const fields = (rawSchema as { fields?: unknown }).fields
    if (!Array.isArray(fields)) continue
    for (const field of fields as FormFieldLike[]) {
      if (!field || typeof field !== "object") continue
      const key = asString(field.key)
      if (!key) continue
      const label = asString(field.label)
      if (label) fieldLabels[key] = label
      if (!Array.isArray(field.options)) continue
      for (const option of field.options) {
        const normalized = normalizeOption(option)
        if (!normalized) continue
        if (normalized.label) valueLabels[normalized.value] = normalized.label
      }
    }
  }
  return { fieldLabels, valueLabels }
}

export function formatConditionForDisplay(
  condition: GatewayCondition | ConditionGroup | undefined,
  dict: ConditionDisplayDict,
  locale: ConditionDisplayLocale,
): string {
  if (!condition) return ""
  if (!isConditionGroup(condition)) {
    return mapSingleConditionForDisplay(condition, dict, locale)
  }
  const logicLabel = condition.logic === "and" ? locale.logicLabels.and : locale.logicLabels.or
  return condition.conditions
    .map((item) => (isConditionGroup(item)
      ? `(${formatConditionForDisplay(item, dict, locale)})`
      : mapSingleConditionForDisplay(item, dict, locale)))
    .join(` ${logicLabel} `)
}

export function buildEdgeLabelDisplay(
  edgeData: WFEdgeData | undefined,
  dict: ConditionDisplayDict,
  conditionLocale: ConditionDisplayLocale,
  edgeLocale: EdgeLabelLocale,
): EdgeLabelDisplay {
  const outcome = edgeData?.outcome
  if (outcome) {
    return {
      primary: edgeLocale.outcomeLabels[outcome] ?? outcome,
      raw: outcome,
      isCompleted: outcome === "completed" || outcome === "submitted",
      kind: "outcome",
    }
  }
  if (edgeData?.condition) {
    return {
      primary: formatConditionForDisplay(edgeData.condition, dict, conditionLocale),
      raw: conditionSummary(edgeData.condition),
      isCompleted: false,
      kind: "condition",
    }
  }
  if (edgeData?.default ?? edgeData?.isDefault) {
    return {
      primary: edgeLocale.defaultLabel,
      raw: "/",
      isCompleted: false,
      kind: "default",
    }
  }
  return { primary: "", raw: "", isCompleted: false, kind: "empty" }
}

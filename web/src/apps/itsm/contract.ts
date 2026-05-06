export const TICKET_STATUSES = [
  "submitted",
  "waiting_human",
  "approved_decisioning",
  "rejected_decisioning",
  "decisioning",
  "executing_action",
  "completed",
  "rejected",
  "withdrawn",
  "cancelled",
  "failed",
] as const

export type TicketStatus = (typeof TICKET_STATUSES)[number]

export const TICKET_OUTCOMES = [
  "",
  "approved",
  "rejected",
  "fulfilled",
  "withdrawn",
  "cancelled",
  "failed",
] as const

export type TicketOutcome = (typeof TICKET_OUTCOMES)[number]

export const ACTIVITY_STATUSES = [
  "pending",
  "in_progress",
  "approved",
  "rejected",
  "transferred",
  "delegated",
  "claimed_by_other",
  "completed",
  "cancelled",
  "failed",
] as const

export type ActivityStatus = (typeof ACTIVITY_STATUSES)[number]

export const ENGINE_TYPES = ["classic", "smart"] as const
export type EngineType = (typeof ENGINE_TYPES)[number]

export const WORKFLOW_NODE_TYPES = [
  "start",
  "end",
  "form",
  "approve",
  "process",
  "action",
  "notify",
  "wait",
  "exclusive",
  "parallel",
  "inclusive",
  "script",
  "subprocess",
  "timer",
  "signal",
  "b_timer",
  "b_error",
] as const

export type WorkflowNodeType = (typeof WORKFLOW_NODE_TYPES)[number]
export type EditorNodeType = Exclude<WorkflowNodeType, "approve" | "b_timer" | "b_error">

export const SERVICE_DESK_STAGES = [
  "idle",
  "candidates_ready",
  "service_selected",
  "service_loaded",
  "awaiting_confirmation",
  "confirmed",
  "submitted",
] as const

export type ServiceDeskStage = (typeof SERVICE_DESK_STAGES)[number]

export const SURFACE_TYPES = ["itsm.draft_form"] as const
export type SurfaceType = (typeof SURFACE_TYPES)[number]

export type StatusTone = "success" | "destructive" | "secondary" | "progress" | "warning"
export type SmartState =
  | "terminal"
  | "ai_disabled"
  | "waiting_ai_confirmation"
  | "action_running"
  | "waiting_human"
  | "ai_reasoning"
  | "ai_decided"
export type RecoveryActionCode = "retry" | "handoff_human" | "withdraw"
export type MonitorSeverity = "blocked" | "risk" | "info"
export type RiskLevel = "blocked" | "risk" | "normal"
export type DecisionQualityDimension = "service" | "department"

export interface WorkflowNodeCapability {
  type: WorkflowNodeType
  executable: boolean
  requiredFields?: string[]
  disabledReason?: string
}

export interface WorkflowCapability {
  version: string
  nodeTypes: Record<WorkflowNodeType, WorkflowNodeCapability>
}

import { type TicketMonitorReason, type TicketMonitorSummary } from "../../api"

export const AUDIT_METRICS = [
  { code: "active_total", labelKey: "monitor.activeTotal", summaryKey: "activeTotal" },
  { code: "blocked_total", labelKey: "monitor.stuckTotal", summaryKey: "stuckTotal" },
  { code: "risk_total", labelKey: "monitor.riskTotal", summaryKey: "riskTotal" },
  { code: "sla_risk_total", labelKey: "monitor.slaRiskTotal", summaryKey: "slaRiskTotal" },
  { code: "ai_incident_total", labelKey: "monitor.aiIncidentTotal", summaryKey: "aiIncidentTotal" },
  { code: "completed_today_total", labelKey: "monitor.completedTodayTotal", summaryKey: "completedTodayTotal" },
  { code: "smart_active_total", labelKey: "monitor.smartActiveTotal", summaryKey: "smartActiveTotal" },
  { code: "classic_active_total", labelKey: "monitor.classicActiveTotal", summaryKey: "classicActiveTotal" },
] as const

export type AuditMetricCode = typeof AUDIT_METRICS[number]["code"]
export type AuditEvidenceEntry = ReturnType<typeof auditEvidenceEntries>[number]

export type AuditReasonRow =
  | {
    type: "structured"
    message: string
    severity: string
    metricCode: string
    ruleCode: string
    evidence: AuditEvidenceEntry[]
  }
  | {
    type: "legacy"
    message: string
    severity: "blocked"
    metricCode: ""
    ruleCode: ""
    evidence: AuditEvidenceEntry[]
  }

export function auditMetricFilter(metricCode: AuditMetricCode) {
  return { metricCode }
}

export function auditMetricValue(summary: TicketMonitorSummary | null | undefined, metricCode: AuditMetricCode) {
  const metric = AUDIT_METRICS.find((item) => item.code === metricCode)
  return metric && summary ? summary[metric.summaryKey] : 0
}

export function nextAuditMetricSelection(current: AuditMetricCode | null, metricCode: AuditMetricCode) {
  return current === metricCode ? null : metricCode
}

export function auditEvidenceEntries(evidence: Record<string, unknown> | null | undefined) {
  return Object.entries(evidence ?? {})
    .sort(([a], [b]) => a.localeCompare(b))
    .map(([field, value]) => ({ field, value: auditEvidenceValue(value) }))
}

export function auditReasonRows(
  monitorReasons: TicketMonitorReason[] | null | undefined,
  stuckReasons: string[] | null | undefined,
): AuditReasonRow[] {
  if (monitorReasons?.length) {
    return monitorReasons.map((reason) => ({
      type: "structured",
      message: reason.message,
      severity: reason.severity,
      metricCode: reason.metricCode,
      ruleCode: reason.ruleCode,
      evidence: auditEvidenceEntries(reason.evidence),
    }))
  }
  return (stuckReasons ?? []).map((message) => ({
    type: "legacy",
    message,
    severity: "blocked",
    metricCode: "",
    ruleCode: "",
    evidence: [],
  }))
}

function auditEvidenceValue(value: unknown): string {
  if (value === null || value === undefined) return "nil"
  if (typeof value === "boolean") return value ? "true" : "false"
  if (typeof value === "number") return Number.isInteger(value) ? String(value) : value.toFixed(2)
  return String(value)
}

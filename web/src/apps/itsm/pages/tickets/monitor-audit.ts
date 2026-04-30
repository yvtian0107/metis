export const AUDIT_METRICS = [
  { code: "active_total", labelKey: "monitor.activeTotal" },
  { code: "blocked_total", labelKey: "monitor.stuckTotal" },
  { code: "risk_total", labelKey: "monitor.riskTotal" },
  { code: "sla_risk_total", labelKey: "monitor.slaRiskTotal" },
  { code: "ai_incident_total", labelKey: "monitor.aiIncidentTotal" },
  { code: "completed_today_total", labelKey: "monitor.completedTodayTotal" },
  { code: "smart_active_total", labelKey: "monitor.smartActiveTotal" },
  { code: "classic_active_total", labelKey: "monitor.classicActiveTotal" },
] as const

export type AuditMetricCode = typeof AUDIT_METRICS[number]["code"]

export function auditMetricFilter(metricCode: AuditMetricCode) {
  return { metricCode }
}

export function auditEvidenceEntries(evidence: Record<string, unknown> | null | undefined) {
  return Object.entries(evidence ?? {})
    .sort(([a], [b]) => a.localeCompare(b))
    .map(([field, value]) => ({ field, value: auditEvidenceValue(value) }))
}

function auditEvidenceValue(value: unknown): string {
  if (value === null || value === undefined) return "nil"
  if (typeof value === "boolean") return value ? "true" : "false"
  if (typeof value === "number") return Number.isInteger(value) ? String(value) : value.toFixed(2)
  return String(value)
}

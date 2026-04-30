import { describe, expect, test } from "bun:test"

import {
  AUDIT_METRICS,
  auditEvidenceEntries,
  auditMetricFilter,
} from "./monitor-audit"

describe("ticket monitor audit helpers", () => {
  test("maps metric cards to reproducible backend filters", () => {
    expect(auditMetricFilter("blocked_total")).toEqual({ metricCode: "blocked_total" })
    expect(auditMetricFilter("risk_total")).toEqual({ metricCode: "risk_total" })
    expect(auditMetricFilter("sla_risk_total")).toEqual({ metricCode: "sla_risk_total" })
    expect(auditMetricFilter("completed_today_total")).toEqual({ metricCode: "completed_today_total" })
    expect(auditMetricFilter("smart_active_total")).toEqual({ metricCode: "smart_active_total" })
  })

  test("defines only auditable top-level monitor metrics", () => {
    expect(AUDIT_METRICS.map((metric) => metric.code)).toEqual([
      "active_total",
      "blocked_total",
      "risk_total",
      "sla_risk_total",
      "ai_incident_total",
      "completed_today_total",
      "smart_active_total",
      "classic_active_total",
    ])
  })

  test("turns structured evidence into inspectable rows", () => {
    const rows = auditEvidenceEntries({
      threshold_minutes: 30,
      deadline_field: "sla_resolution_deadline",
      action_failed: true,
      empty_value: null,
    })

    expect(rows).toEqual([
      { field: "action_failed", value: "true" },
      { field: "deadline_field", value: "sla_resolution_deadline" },
      { field: "empty_value", value: "nil" },
      { field: "threshold_minutes", value: "30" },
    ])
  })
})

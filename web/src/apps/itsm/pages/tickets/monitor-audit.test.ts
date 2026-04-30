import { describe, expect, test } from "bun:test"

import {
  AUDIT_METRICS,
  auditEvidenceEntries,
  auditMetricFilter,
  auditMetricValue,
  auditReasonRows,
  nextAuditMetricSelection,
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

  test("maps metric definitions to summary values", () => {
    const summary = {
      activeTotal: 12,
      stuckTotal: 2,
      riskTotal: 3,
      slaRiskTotal: 4,
      aiIncidentTotal: 5,
      completedTodayTotal: 6,
      smartActiveTotal: 7,
      classicActiveTotal: 8,
    }

    expect(AUDIT_METRICS.map((metric) => auditMetricValue(summary, metric.code))).toEqual([
      12,
      2,
      3,
      4,
      5,
      6,
      7,
      8,
    ])
  })

  test("toggles metric drill-down selection", () => {
    expect(nextAuditMetricSelection(null, "blocked_total")).toBe("blocked_total")
    expect(nextAuditMetricSelection("blocked_total", "blocked_total")).toBeNull()
    expect(nextAuditMetricSelection("blocked_total", "risk_total")).toBe("risk_total")
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

  test("uses structured monitor reasons before legacy stuck reasons", () => {
    const rows = auditReasonRows(
      [
        {
          metricCode: "sla_risk_total",
          ruleCode: "sla_resolution_due_soon",
          severity: "risk",
          message: "解决 SLA 距离截止小于 30 分钟",
          evidence: {
            deadline_field: "sla_resolution_deadline",
            threshold_minutes: 30,
          },
        },
      ],
      ["旧的卡单文本"],
    )

    expect(rows).toEqual([
      {
        type: "structured",
        message: "解决 SLA 距离截止小于 30 分钟",
        severity: "risk",
        metricCode: "sla_risk_total",
        ruleCode: "sla_resolution_due_soon",
        evidence: [
          { field: "deadline_field", value: "sla_resolution_deadline" },
          { field: "threshold_minutes", value: "30" },
        ],
      },
    ])
  })
})

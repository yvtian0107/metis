import { describe, expect, test } from "bun:test"
import { getTicketStatusView } from "./ticket-status"
import { type TicketItem } from "../api"

function ticket(overrides: Partial<TicketItem> = {}): TicketItem {
  return {
    id: 1,
    code: "TICK-000001",
    title: "VPN access",
    description: "",
    serviceId: 1,
    serviceName: "VPN",
    engineType: "smart",
    status: "submitted",
    outcome: "",
    statusLabel: "已提交",
    statusTone: "secondary",
    lastHumanOutcome: "",
    decisioningReason: "",
    priorityId: 1,
    priorityName: "Normal",
    priorityColor: "#64748b",
    requesterId: 7,
    requesterName: "alice",
    assigneeId: null,
    assigneeName: "",
    currentActivityId: null,
    source: "agent",
    agentSessionId: null,
    aiFailureCount: 0,
    formData: {},
    workflowJson: {},
    slaStatus: "normal",
    slaResponseDeadline: null,
    slaResolutionDeadline: null,
    finishedAt: null,
    canAct: false,
    canOverride: false,
    createdAt: "2026-04-30T00:00:00Z",
    updatedAt: "2026-04-30T00:00:00Z",
    ...overrides,
  }
}

describe("getTicketStatusView", () => {
  test("keeps the i18n key for a known status", () => {
    expect(getTicketStatusView(ticket({ status: "submitted" }))).toMatchObject({
      key: "statusSubmitted",
      label: "已提交",
      variant: "secondary",
    })
  })

  test("uses the backend label for an unknown status", () => {
    expect(getTicketStatusView(ticket({
      status: "waiting_external_confirmation",
      statusLabel: "等待外部确认",
      statusTone: "warning",
    }))).toMatchObject({
      key: undefined,
      label: "等待外部确认",
      variant: "outline",
    })
  })

  test("falls back to the raw status when an unknown status has no label", () => {
    expect(getTicketStatusView(ticket({
      status: "legacy_pending",
      statusLabel: "",
      statusTone: "unknown-tone",
    }))).toMatchObject({
      key: undefined,
      label: "legacy_pending",
      variant: "secondary",
    })
  })
})

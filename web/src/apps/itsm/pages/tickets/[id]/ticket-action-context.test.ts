import { describe, expect, test } from "bun:test"
import { buildTicketActionContext } from "./ticket-action-context"
import { type ActivityItem, type TicketItem } from "../../../api"

function ticket(overrides: Partial<TicketItem> = {}): TicketItem {
  return {
    id: 1,
    code: "TICK-000001",
    title: "VPN access",
    description: "",
    serviceId: 1,
    serviceName: "VPN",
    engineType: "smart",
    status: "waiting_human",
    outcome: "",
    statusLabel: "等待人工",
    statusTone: "warning",
    lastHumanOutcome: "",
    decisioningReason: "",
    priorityId: 1,
    priorityName: "Normal",
    priorityColor: "#64748b",
    requesterId: 7,
    requesterName: "alice",
    assigneeId: null,
    assigneeName: "",
    currentActivityId: 11,
    source: "agent",
    agentSessionId: null,
    aiFailureCount: 0,
    formData: {},
    workflowJson: {},
    slaStatus: "normal",
    slaResponseDeadline: null,
    slaResolutionDeadline: null,
    finishedAt: null,
    smartState: "waiting_human",
    canAct: false,
    canOverride: false,
    createdAt: "2026-04-30T00:00:00Z",
    updatedAt: "2026-04-30T00:00:00Z",
    ...overrides,
  }
}

function activity(overrides: Partial<ActivityItem> = {}): ActivityItem {
  return {
    id: 11,
    ticketId: 1,
    name: "审批",
    activityType: "approve",
    status: "pending",
    nodeId: "approve-1",
    executionMode: "single",
    sequenceOrder: 1,
    formSchema: {},
    formData: {},
    transitionOutcome: "",
    aiDecision: null,
    aiReasoning: null,
    aiConfidence: null,
    overriddenBy: null,
    canAct: false,
    startedAt: null,
    finishedAt: null,
    createdAt: "2026-04-30T00:00:00Z",
    ...overrides,
  }
}

describe("buildTicketActionContext", () => {
  test("allows processing from a direct detail-page load when the activity can act", () => {
    const ctx = buildTicketActionContext({
      ticket: ticket({ canAct: true }),
      activities: [activity({ canAct: true })],
      currentUserId: 42,
      canAssignPermission: false,
      canCancelPermission: false,
    })

    expect(ctx.canProcess).toBe(true)
    expect(ctx.selectedActionableActivity?.id).toBe(11)
  })

  test("selects a canAct activity before the first pending human activity", () => {
    const firstPending = activity({ id: 11, name: "其他人审批", canAct: false })
    const myPending = activity({ id: 12, name: "我的审批", canAct: true })

    const ctx = buildTicketActionContext({
      ticket: ticket({ currentActivityId: 11 }),
      activities: [firstPending, myPending],
      currentUserId: 42,
      canAssignPermission: false,
      canCancelPermission: false,
    })

    expect(ctx.displayHumanActivity?.id).toBe(11)
    expect(ctx.selectedActionableActivity?.id).toBe(12)
    expect(ctx.canProcess).toBe(true)
  })

  test("keeps the current human activity for display while submitting the actionable activity", () => {
    const currentBlocked = activity({ id: 11, name: "当前卡住节点", canAct: false })
    const actionable = activity({ id: 12, name: "可处理节点", canAct: true })

    const ctx = buildTicketActionContext({
      ticket: ticket({ currentActivityId: 11 }),
      activities: [currentBlocked, actionable],
      currentUserId: 42,
      canAssignPermission: true,
      canCancelPermission: true,
    })

    expect(ctx.displayHumanActivity?.id).toBe(11)
    expect(ctx.selectedActionableActivity?.id).toBe(12)
    expect(ctx.canProcess).toBe(true)
  })

  test("returns all actionable activities for the UI to disambiguate", () => {
    const ctx = buildTicketActionContext({
      ticket: ticket(),
      activities: [
        activity({ id: 21, name: "节点 A", canAct: true }),
        activity({ id: 22, name: "节点 B", canAct: true }),
      ],
      currentUserId: 42,
      canAssignPermission: false,
      canCancelPermission: false,
    })

    expect(ctx.actionableActivities.map((item) => item.id)).toEqual([21, 22])
    expect(ctx.selectedActionableActivity?.id).toBe(21)
  })

  test("allows withdrawal only for the requester while the submitted ticket is actionable", () => {
    const base = {
      activities: [],
      canAssignPermission: false,
      canCancelPermission: false,
    }

    expect(buildTicketActionContext({
      ...base,
      ticket: ticket({ requesterId: 7, status: "submitted", smartState: "waiting_human" }),
      currentUserId: 7,
    }).canWithdraw).toBe(true)

    expect(buildTicketActionContext({
      ...base,
      ticket: ticket({ requesterId: 7, status: "submitted", smartState: "waiting_human" }),
      currentUserId: 8,
    }).canWithdraw).toBe(false)

    expect(buildTicketActionContext({
      ...base,
      ticket: ticket({ requesterId: 7, status: "approved_decisioning", smartState: "ai_reasoning" }),
      currentUserId: 7,
    }).canWithdraw).toBe(false)
  })

  test("allows management only with permission on active non-decisioning tickets", () => {
    const base = {
      activities: [],
      currentUserId: 1,
    }

    const active = buildTicketActionContext({
      ...base,
      ticket: ticket({ status: "waiting_human", smartState: "waiting_human" }),
      canAssignPermission: true,
      canCancelPermission: true,
    })
    expect(active.canAssign).toBe(true)
    expect(active.canCancel).toBe(true)

    const terminal = buildTicketActionContext({
      ...base,
      ticket: ticket({ status: "completed", smartState: "terminal" }),
      canAssignPermission: true,
      canCancelPermission: true,
    })
    expect(terminal.canAssign).toBe(false)
    expect(terminal.canCancel).toBe(false)

    const decisioning = buildTicketActionContext({
      ...base,
      ticket: ticket({ status: "decisioning", smartState: "ai_reasoning" }),
      canAssignPermission: true,
      canCancelPermission: true,
    })
    expect(decisioning.canAssign).toBe(false)
    expect(decisioning.canCancel).toBe(false)
  })
})

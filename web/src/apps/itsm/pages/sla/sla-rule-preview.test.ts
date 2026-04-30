import { describe, expect, test } from "bun:test"
import { buildSLARulePreviewRows } from "./sla-rule-preview"

const sla = {
  id: 1,
  name: "标准",
  code: "standard",
  description: "",
  responseMinutes: 240,
  resolutionMinutes: 1440,
  isActive: true,
  createdAt: "",
  updatedAt: "",
}

const activePriority = {
  id: 10,
  name: "紧急",
  code: "P0",
  value: 1,
  color: "#ff0000",
  description: "",
  isActive: true,
  createdAt: "",
  updatedAt: "",
}

const inactivePriority = {
  ...activePriority,
  id: 11,
  name: "停用优先级",
  code: "PX",
  isActive: false,
}

const channel = { id: 5, name: "企业微信", type: "wecom" }

describe("SLA rule preview", () => {
  test("summarizes notify, reassign and priority escalation targets", () => {
    const rows = buildSLARulePreviewRows({
      slas: [sla],
      rulesBySlaId: {
        1: [
          {
            id: 1,
            slaId: 1,
            triggerType: "response_timeout",
            level: 1,
            waitMinutes: 30,
            actionType: "notify",
            targetConfig: { recipients: [{ type: "requester_manager", name: "提交人上级" }], channelId: 5 },
            isActive: true,
            createdAt: "",
            updatedAt: "",
          },
          {
            id: 2,
            slaId: 1,
            triggerType: "resolution_timeout",
            level: 2,
            waitMinutes: 60,
            actionType: "reassign",
            targetConfig: { assigneeCandidates: [{ type: "user", value: "7", name: "白宇飞" }] },
            isActive: true,
            createdAt: "",
            updatedAt: "",
          },
          {
            id: 3,
            slaId: 1,
            triggerType: "resolution_timeout",
            level: 3,
            waitMinutes: 90,
            actionType: "escalate_priority",
            targetConfig: { priorityId: 10 },
            isActive: true,
            createdAt: "",
            updatedAt: "",
          },
        ],
      },
      priorities: [activePriority],
      channels: [channel],
    })

    expect(rows.map((row) => row.targetSummary)).toEqual([
      "提交人上级 / 企业微信",
      "白宇飞",
      "紧急 / P0",
    ])
    expect(rows.every((row) => row.riskCodes.length === 0)).toBe(true)
  })

  test("marks inactive and duplicate-level rules as risky", () => {
    const rows = buildSLARulePreviewRows({
      slas: [sla],
      rulesBySlaId: {
        1: [
          {
            id: 1,
            slaId: 1,
            triggerType: "response_timeout",
            level: 1,
            waitMinutes: 30,
            actionType: "notify",
            targetConfig: { recipients: [{ type: "requester_manager" }], channelId: 5 },
            isActive: false,
            createdAt: "",
            updatedAt: "",
          },
          {
            id: 2,
            slaId: 1,
            triggerType: "response_timeout",
            level: 1,
            waitMinutes: 45,
            actionType: "notify",
            targetConfig: { recipients: [{ type: "requester_manager" }], channelId: 5 },
            isActive: true,
            createdAt: "",
            updatedAt: "",
          },
        ],
      },
      priorities: [activePriority],
      channels: [channel],
    })

    expect(rows[0].riskCodes).toContain("inactive")
    expect(rows[0].riskCodes).toContain("duplicate_level")
    expect(rows[1].riskCodes).toContain("duplicate_level")
  })

  test("detects missing targets and inactive target priorities", () => {
    const rows = buildSLARulePreviewRows({
      slas: [sla],
      rulesBySlaId: {
        1: [
          {
            id: 1,
            slaId: 1,
            triggerType: "response_timeout",
            level: 1,
            waitMinutes: 30,
            actionType: "notify",
            targetConfig: { recipients: [], channelId: 999 },
            isActive: true,
            createdAt: "",
            updatedAt: "",
          },
          {
            id: 2,
            slaId: 1,
            triggerType: "resolution_timeout",
            level: 1,
            waitMinutes: 60,
            actionType: "reassign",
            targetConfig: { assigneeCandidates: [] },
            isActive: true,
            createdAt: "",
            updatedAt: "",
          },
          {
            id: 3,
            slaId: 1,
            triggerType: "resolution_timeout",
            level: 2,
            waitMinutes: 90,
            actionType: "escalate_priority",
            targetConfig: { priorityId: 11 },
            isActive: true,
            createdAt: "",
            updatedAt: "",
          },
        ],
      },
      priorities: [activePriority, inactivePriority],
      channels: [channel],
    })

    expect(rows[0].riskCodes).toEqual(["notify_missing_recipients", "notify_missing_channel"])
    expect(rows[1].riskCodes).toEqual(["reassign_missing_candidates"])
    expect(rows[2].riskCodes).toEqual(["priority_inactive"])
  })
})

import { describe, expect, test } from "bun:test"

import {
  createServiceDeskWorkspaceActions,
  recoverInitialPromptDraft,
  resolveServiceDeskStaffingState,
} from "./service-desk-behavior"

describe("service desk page behavior", () => {
  test("recovers the initial prompt draft inside the created session after auto-send fails", () => {
    const image = { file: new File(["x"], "vpn.png", { type: "image/png" }), preview: "data:image/png;base64,x" }
    const images = [image]

    const draft = recoverInitialPromptDraft("我想申请 VPN", images)

    expect(draft.input).toBe("我想申请 VPN")
    expect(draft.images).toEqual([image])
    expect(draft.images).not.toBe(images)
  })

  test("exposes retry and continue actions for service desk stream failures", () => {
    const calls: string[] = []
    const actions = createServiceDeskWorkspaceActions({
      regenerate: () => calls.push("regenerate"),
      clearError: () => calls.push("clearError"),
      continueGeneration: () => calls.push("continueGeneration"),
      cancel: () => calls.push("cancel"),
    })

    actions.retry?.()
    actions.continueGeneration?.()
    actions.cancel?.()

    expect(calls).toEqual(["clearError", "regenerate", "continueGeneration", "cancel"])
  })

  test("requires intake health to pass before enabling service desk sessions", () => {
    expect(
      resolveServiceDeskStaffingState({
        posts: { intake: { agentId: 7, agentName: "服务受理岗" } },
        health: { items: [{ key: "intake", label: "服务受理岗", status: "pass", message: "已上岗" }] },
      }),
    ).toEqual({ ready: true, agentId: 7, agentName: "服务受理岗", reason: "ready" })

    expect(
      resolveServiceDeskStaffingState({
        posts: { intake: { agentId: 7, agentName: "服务受理岗" } },
        health: { items: [{ key: "intake", label: "服务受理岗", status: "fail", message: "工具缺失：itsm.service_match" }] },
      }),
    ).toEqual({
      ready: false,
      agentId: 7,
      agentName: "服务受理岗",
      reason: "unhealthy",
      message: "工具缺失：itsm.service_match",
    })
  })

  test("distinguishes service desk config errors from missing staffing", () => {
    const error = new Error("Forbidden")

    expect(resolveServiceDeskStaffingState(undefined, { loading: false, error })).toEqual({
      ready: false,
      agentId: 0,
      agentName: "IT 服务台",
      reason: "config_error",
      message: "Forbidden",
    })

    expect(resolveServiceDeskStaffingState(undefined)).toEqual({
      ready: false,
      agentId: 0,
      agentName: "IT 服务台",
      reason: "missing",
    })
  })
})

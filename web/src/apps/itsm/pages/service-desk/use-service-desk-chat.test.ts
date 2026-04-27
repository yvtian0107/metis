import { describe, expect, test } from "bun:test"
import type { UIMessage } from "ai"

import {
  doesServiceDeskHistoryCoverLiveMessages,
  shouldProcessServiceDeskHistorySnapshot,
  shouldSyncServiceDeskHistory,
} from "./service-desk-chat-sync"
import type { SessionMessage } from "../../../../lib/api"
import { ensureUniqueUIMessageIds, storedSessionMessageUIId } from "../../../../components/chat-workspace/message-id"

describe("shouldSyncServiceDeskHistory", () => {
  test("does not apply server history while a live run is submitted or streaming", () => {
    for (const status of ["submitted", "streaming"] as const) {
      expect(
        shouldSyncServiceDeskHistory({
          status,
          hasServerSnapshot: true,
          serverSignature: "server-new",
          localSignature: "local-live",
        }),
      ).toBe(false)
    }
  })

  test("applies server history only after the chat is idle and the snapshot changed", () => {
    for (const status of ["ready", "error"] as const) {
      expect(
        shouldSyncServiceDeskHistory({
          status,
          hasServerSnapshot: true,
          serverSignature: "server-new",
          localSignature: "local-old",
        }),
      ).toBe(true)
    }
  })

  test("skips sync without a server snapshot or when signatures already match", () => {
    expect(
      shouldSyncServiceDeskHistory({
        status: "ready",
        hasServerSnapshot: false,
        serverSignature: "server-new",
        localSignature: "local-old",
      }),
    ).toBe(false)
    expect(
      shouldSyncServiceDeskHistory({
        status: "ready",
        hasServerSnapshot: true,
        serverSignature: "same",
        localSignature: "same",
      }),
    ).toBe(false)
  })
})

describe("shouldProcessServiceDeskHistorySnapshot", () => {
  test("does not re-process a stale server snapshot after a live run finishes", () => {
    expect(
      shouldProcessServiceDeskHistorySnapshot({
        status: "ready",
        hasServerSnapshot: true,
        serverCoversLiveMessages: true,
        serverSnapshotKey: "101:empty-history",
        syncedServerSnapshotKey: "101:empty-history",
      }),
    ).toBe(false)
  })

  test("processes a newly fetched server snapshot only when the chat is idle", () => {
    expect(
      shouldProcessServiceDeskHistorySnapshot({
        status: "ready",
        hasServerSnapshot: true,
        serverCoversLiveMessages: true,
        serverSnapshotKey: "101:persisted-history",
        syncedServerSnapshotKey: "101:empty-history",
      }),
    ).toBe(true)
    expect(
      shouldProcessServiceDeskHistorySnapshot({
        status: "streaming",
        hasServerSnapshot: true,
        serverCoversLiveMessages: true,
        serverSnapshotKey: "101:persisted-history",
        syncedServerSnapshotKey: "101:empty-history",
      }),
    ).toBe(false)
  })

  test("skips server snapshots that have not caught up with the live messages", () => {
    expect(
      shouldProcessServiceDeskHistorySnapshot({
        status: "ready",
        hasServerSnapshot: true,
        serverCoversLiveMessages: false,
        serverSnapshotKey: "101:user-only",
        syncedServerSnapshotKey: "101:empty-history",
      }),
    ).toBe(false)
  })
})

describe("doesServiceDeskHistoryCoverLiveMessages", () => {
  test("does not treat a user-only server snapshot as covering a live assistant response", () => {
    const serverMessages = [
      {
        id: "1",
        role: "user",
        parts: [{ type: "text", text: "我想申请 VPN" }],
      },
    ] as UIMessage[]
    const liveMessages = [
      ...serverMessages,
      {
        id: "msg-1",
        role: "assistant",
        parts: [{ type: "text", text: "正在为你匹配服务" }],
      },
    ] as UIMessage[]

    expect(doesServiceDeskHistoryCoverLiveMessages(serverMessages, liveMessages)).toBe(false)
  })

  test("requires persisted tool results before replacing a completed live tool", () => {
    const liveMessages = [
      {
        id: "msg-1",
        role: "assistant",
        parts: [{
          type: "tool-itsm.service_match",
          toolCallId: "call-1",
          state: "output-available",
          input: { query: "VPN" },
          output: { selected_service_id: 5 },
        }],
      },
    ] as UIMessage[]
    const serverToolCallOnly = [
      {
        id: "2",
        role: "assistant",
        metadata: { originalRole: "tool_call", tool_call_id: "call-1" },
        parts: [{ type: "text", text: "" }],
      },
    ] as UIMessage[]
    const serverWithResult = [
      ...serverToolCallOnly,
      {
        id: "3",
        role: "assistant",
        metadata: { originalRole: "tool_result", tool_call_id: "call-1" },
        parts: [{ type: "text", text: "" }],
      },
    ] as UIMessage[]

    expect(doesServiceDeskHistoryCoverLiveMessages(serverToolCallOnly, liveMessages)).toBe(false)
    expect(doesServiceDeskHistoryCoverLiveMessages(serverWithResult, liveMessages)).toBe(true)
  })

  test("covers live surfaces by stable surface id", () => {
    const liveMessages = [
      {
        id: "msg-1",
        role: "assistant",
        parts: [{
          type: "data-ui-surface",
          data: {
            surfaceId: "itsm-draft-form-call-1",
            surfaceType: "itsm.draft_form",
            payload: { status: "ready" },
          },
        }],
      },
    ] as UIMessage[]
    const serverMessages = [
      {
        id: "4",
        role: "assistant",
        parts: [{
          type: "data-ui-surface",
          data: {
            surfaceId: "itsm-draft-form-call-1",
            surfaceType: "itsm.draft_form",
            payload: { status: "ready" },
          },
        }],
      },
    ] as UIMessage[]

    expect(doesServiceDeskHistoryCoverLiveMessages(serverMessages, liveMessages)).toBe(true)
  })
})

describe("storedSessionMessageUIId", () => {
  test("uses stable stored ids that cannot collide with live chat message ids", () => {
    const messages = [
      {
        id: 1,
        sessionId: 101,
        sequence: 1,
        role: "user",
        content: "我想申请 VPN",
        tokenCount: 0,
        createdAt: "2026-04-24T00:00:00Z",
      },
      {
        id: 2,
        sessionId: 101,
        sequence: 2,
        role: "assistant",
        content: "请补充 VPN 账号",
        tokenCount: 0,
        createdAt: "2026-04-24T00:00:01Z",
      },
    ] satisfies SessionMessage[]

    expect(messages.map(storedSessionMessageUIId)).toEqual([
      "stored-101-1-user-1",
      "stored-101-2-assistant-2",
    ])
  })
})

describe("ensureUniqueUIMessageIds", () => {
  test("keeps message ids unchanged when they are already unique", () => {
    const messages = [
      { id: "a", role: "user", parts: [{ type: "text", text: "1" }] },
      { id: "b", role: "assistant", parts: [{ type: "text", text: "2" }] },
    ] as UIMessage[]

    expect(ensureUniqueUIMessageIds(messages)).toBe(messages)
  })

  test("adds stable suffixes to repeated runtime message ids", () => {
    const messages = [
      { id: "same", role: "user", parts: [{ type: "text", text: "1" }] },
      { id: "same", role: "assistant", parts: [{ type: "text", text: "2" }] },
      { id: "same", role: "assistant", parts: [{ type: "text", text: "3" }] },
    ] as UIMessage[]

    expect(ensureUniqueUIMessageIds(messages).map((message) => message.id)).toEqual([
      "same",
      "same#2",
      "same#3",
    ])
  })
})

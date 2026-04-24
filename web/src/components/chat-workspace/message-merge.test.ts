import { describe, expect, test } from "bun:test"
import type { UIMessage } from "ai"
import { mergeTimelineMessages } from "./message-merge"

function storedToolMessage(id: string, originalRole: "tool_call" | "tool_result"): UIMessage {
  return {
    id: `${originalRole}-${id}`,
    role: "assistant",
    metadata: {
      originalRole,
      tool_call_id: id,
      tool_name: originalRole === "tool_call" ? "itsm.service_match" : undefined,
      status: originalRole === "tool_call" ? "running" : "completed",
    },
    parts: [{ type: "text", text: "", state: "done" }],
  }
}

describe("mergeTimelineMessages", () => {
  test("keeps stored tool call and result so timeline can resolve completed status", () => {
    const toolCall = storedToolMessage("call-1", "tool_call")
    const toolResult = storedToolMessage("call-1", "tool_result")

    expect(mergeTimelineMessages([toolCall, toolResult], [])).toEqual([toolCall, toolResult])
  })

  test("prefers live tool state over persisted running tool messages", () => {
    const liveTool: UIMessage = {
      id: "live-assistant",
      role: "assistant",
      parts: [{
        type: "dynamic-tool",
        toolCallId: "call-1",
        toolName: "itsm.service_match",
        state: "output-available",
        input: { query: "VPN" },
        output: { ok: true },
      }],
    } as UIMessage
    const toolCall = storedToolMessage("call-1", "tool_call")
    const toolResult = storedToolMessage("call-1", "tool_result")

    expect(mergeTimelineMessages([toolCall, toolResult], [liveTool])).toEqual([liveTool])
  })
})

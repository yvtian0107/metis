import { describe, expect, test } from "bun:test"
import type { UIMessage } from "ai"

import { collectToolActivitiesFromParts } from "./tool-activities"

describe("collectToolActivitiesFromParts", () => {
  test("merges duplicate tool parts with the same toolCallId into one final activity", () => {
    const message = {
      id: "assistant-1",
      role: "assistant",
      parts: [
        {
          type: "tool-itsm.service_match",
          toolCallId: "call-1",
          state: "input-available",
          input: { query: "VPN" },
        },
        {
          type: "tool-itsm.service_match",
          toolCallId: "call-1",
          state: "output-available",
          output: { selected_service_id: 5 },
        },
      ],
    } as UIMessage

    expect(collectToolActivitiesFromParts(message)).toEqual([
      {
        id: "call-1",
        toolName: "itsm.service_match",
        toolArgs: { query: "VPN" },
        status: "completed",
        errorText: undefined,
        durationMs: undefined,
      },
    ])
  })
})

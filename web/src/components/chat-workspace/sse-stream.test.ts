import { describe, expect, test } from "bun:test"
import type { UIMessageChunk } from "ai"
import { createStreamFromSSE } from "./sse-stream"

const encoder = new TextEncoder()

function encodeSSE(data: string) {
  return encoder.encode(`data: ${data}\n\n`)
}

function timeout(ms: number) {
  return new Promise<"timeout">((resolve) => {
    setTimeout(() => resolve("timeout"), ms)
  })
}

describe("createStreamFromSSE", () => {
  test("paces UI chunks when multiple SSE events arrive in one network chunk", async () => {
    const body = new ReadableStream<Uint8Array>({
      pull(controller) {
        controller.enqueue(encoder.encode([
          `data: ${JSON.stringify({ id: "text-1", type: "text-start" })}`,
          "",
          `data: ${JSON.stringify({ delta: "你", id: "text-1", type: "text-delta" })}`,
          "",
          `data: ${JSON.stringify({ delta: "好", id: "text-1", type: "text-delta" })}`,
          "",
          `data: ${JSON.stringify({ id: "text-1", type: "text-end" })}`,
          "",
          "data: [DONE]",
          "",
        ].join("\n")))
        controller.close()
      },
    })
    const reader = createStreamFromSSE(new Response(body)).getReader()

    const start = await reader.read()
    expect(start.value?.type).toBe("text-start")

    const firstDeltaRead = reader.read()
    expect(await Promise.race([firstDeltaRead, timeout(5)])).toBe("timeout")

    const firstDelta = await firstDeltaRead
    expect(firstDelta.value).toMatchObject({
      type: "text-delta",
      delta: "你",
      id: "text-1",
    })

    const secondDelta = await reader.read()
    const textEnd = await reader.read()
    await reader.cancel()

    expect(secondDelta.value).toMatchObject({
      type: "text-delta",
      delta: "好",
      id: "text-1",
    })
    expect(textEnd.value?.type).toBe("text-end")
  })

  test("emits the first delta before the upstream SSE stream finishes", async () => {
    let releaseSecondChunk: (() => void) | undefined
    const secondChunkReleased = new Promise<void>((resolve) => {
      releaseSecondChunk = resolve
    })
    let pullCount = 0

    const body = new ReadableStream<Uint8Array>({
      async pull(controller) {
        pullCount += 1
        if (pullCount === 1) {
          controller.enqueue(encodeSSE(JSON.stringify({ id: "text-1", type: "text-start" })))
          controller.enqueue(encodeSSE(JSON.stringify({ delta: "你", id: "text-1", type: "text-delta" })))
          return
        }

        await secondChunkReleased
        controller.enqueue(encodeSSE(JSON.stringify({ delta: "好", id: "text-1", type: "text-delta" })))
        controller.enqueue(encodeSSE(JSON.stringify({ id: "text-1", type: "text-end" })))
        controller.enqueue(encodeSSE(JSON.stringify({
          finishReason: "stop",
          type: "finish",
          usage: { promptTokens: 2, completionTokens: 2 },
        })))
        controller.enqueue(encodeSSE("[DONE]"))
        controller.close()
      },
    })
    const usage: Array<{ promptTokens: number; completionTokens: number }> = []
    const reader = createStreamFromSSE(new Response(body), (value) => usage.push(value)).getReader()

    const start = await reader.read()
    expect(start.value?.type).toBe("text-start")

    const firstDelta = await Promise.race([reader.read(), timeout(50)])
    expect(firstDelta).not.toBe("timeout")
    expect((firstDelta as ReadableStreamReadResult<UIMessageChunk>).value).toMatchObject({
      type: "text-delta",
      delta: "你",
      id: "text-1",
    })

    const secondDeltaRead = reader.read()
    expect(await Promise.race([secondDeltaRead, timeout(50)])).toBe("timeout")

    releaseSecondChunk?.()

    const secondDelta = await secondDeltaRead
    expect(secondDelta.value).toMatchObject({
      type: "text-delta",
      delta: "好",
      id: "text-1",
    })

    const textEnd = await reader.read()
    const finish = await reader.read()
    await reader.cancel()

    expect([textEnd.value?.type, finish.value?.type]).toEqual(["text-end", "finish"])
    expect(usage).toEqual([{ promptTokens: 2, completionTokens: 2 }])
  })
})

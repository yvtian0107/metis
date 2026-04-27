import type { UIMessageChunk } from "ai"

const STREAM_CHUNK_MIN_INTERVAL_MS = 24

function delay(ms: number) {
  return new Promise<void>((resolve) => {
    setTimeout(resolve, ms)
  })
}

export function createStreamFromSSE(
  response: Response,
  onUsage?: (usage: { promptTokens: number; completionTokens: number }) => void,
): ReadableStream<UIMessageChunk> {
  const reader = response.body?.getReader()
  if (!reader) {
    throw new Error("No response body")
  }

  const decoder = new TextDecoder()
  let buffer = ""
  const pendingChunks: UIMessageChunk[] = []
  let lastEnqueueAt = 0

  function collectChunk(data: string) {
    if (data === "[DONE]") return

    try {
      const chunk = JSON.parse(data) as UIMessageChunk
      if (
        chunk.type === "finish" &&
        "usage" in chunk &&
        chunk.usage &&
        typeof chunk.usage === "object"
      ) {
        const usage = chunk.usage as { promptTokens?: number; completionTokens?: number }
        onUsage?.({
          promptTokens: usage.promptTokens || 0,
          completionTokens: usage.completionTokens || 0,
        })
      }
      pendingChunks.push(chunk)
    } catch (e) {
      console.error("Failed to parse SSE chunk:", data, e)
    }
  }

  function collectLines(value: Uint8Array) {
    buffer += decoder.decode(value, { stream: true })
    const lines = buffer.split("\n")
    buffer = lines.pop() || ""

    for (const line of lines) {
      const trimmed = line.trim()
      if (!trimmed.startsWith("data: ")) continue
      collectChunk(trimmed.slice(6))
    }
  }

  async function enqueueChunk(controller: ReadableStreamDefaultController<UIMessageChunk>) {
    if (lastEnqueueAt > 0) {
      const elapsed = Date.now() - lastEnqueueAt
      if (elapsed < STREAM_CHUNK_MIN_INTERVAL_MS) {
        await delay(STREAM_CHUNK_MIN_INTERVAL_MS - elapsed)
      }
    }

    controller.enqueue(pendingChunks.shift()!)
    lastEnqueueAt = Date.now()
  }

  return new ReadableStream<UIMessageChunk>({
    async pull(controller) {
      if (pendingChunks.length > 0) {
        await enqueueChunk(controller)
        return
      }

      while (pendingChunks.length === 0) {
        const { done, value } = await reader.read()
        if (done) {
          const remainder = buffer.trim()
          if (remainder.startsWith("data: ")) {
            collectChunk(remainder.slice(6))
            break
          }
          controller.close()
          return
        }

        collectLines(value)
      }

      await enqueueChunk(controller)
    },
    cancel() {
      reader.cancel()
    },
  })
}

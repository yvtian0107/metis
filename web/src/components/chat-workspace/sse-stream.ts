import type { UIMessageChunk } from "ai"

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

  return new ReadableStream<UIMessageChunk>({
    async pull(controller) {
      const { done, value } = await reader.read()
      if (done) {
        controller.close()
        return
      }

      buffer += decoder.decode(value, { stream: true })
      const lines = buffer.split("\n")
      buffer = lines.pop() || ""

      for (const line of lines) {
        const trimmed = line.trim()
        if (!trimmed.startsWith("data: ")) continue
        const data = trimmed.slice(6)
        if (data === "[DONE]") continue

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
          controller.enqueue(chunk)
        } catch (e) {
          console.error("Failed to parse SSE chunk:", data, e)
        }
      }
    },
    cancel() {
      reader.cancel()
    },
  })
}

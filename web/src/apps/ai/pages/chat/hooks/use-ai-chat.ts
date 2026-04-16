"use client";

import { useCallback, useEffect, useMemo, useState } from "react"
import { Chat, useChat, type UseChatHelpers } from "@ai-sdk/react"
import type { ChatTransport, UIMessage, UIMessageChunk } from "ai"
import { sessionApi, type SessionMessage } from "@/lib/api"
import { TOKEN_KEY } from "@/lib/constants"

export function sessionMessagesToUIMessages(messages: SessionMessage[]): UIMessage[] {
  return messages.map((m) => {
    const base: UIMessage = {
      id: String(m.id),
      role: m.role === "user" ? "user" : "assistant",
      metadata: { originalRole: m.role, ...m.metadata },
      parts: [{ type: "text", text: m.content || "", state: "done" }],
    }

    if (m.role === "user" && m.metadata && Array.isArray(m.metadata.images)) {
      const images = m.metadata.images as string[]
      for (const url of images) {
        base.parts.push({ type: "file", url, mediaType: "image/*" })
      }
    }

    return base
  })
}

function createStreamFromSSE(
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
      while (true) {
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
      }
    },
    cancel() {
      reader.cancel()
    },
  })
}

type SendMessagesOptions = Parameters<ChatTransport<UIMessage>["sendMessages"]>[0]
type ReconnectOptions = Parameters<ChatTransport<UIMessage>["reconnectToStream"]>[0]

class SessionTransport implements ChatTransport<UIMessage> {
  private sessionId: number
  private pendingImageUrls: string[] = []
  private onUsage: (usage: { promptTokens: number; completionTokens: number }) => void

  constructor(
    sessionId: number,
    onUsage: (usage: { promptTokens: number; completionTokens: number }) => void,
  ) {
    this.sessionId = sessionId
    this.onUsage = onUsage
  }

  setPendingImageUrls(urls: string[]) {
    this.pendingImageUrls = urls
  }

  async sendMessages(options: SendMessagesOptions) {
    const { trigger, messages, abortSignal } = options

    if (trigger === "submit-message") {
      const lastUserMessage = [...messages].reverse().find((m) => m.role === "user")
      if (lastUserMessage) {
        const text = lastUserMessage.parts
          ?.filter((p): p is { type: "text"; text: string } => p.type === "text")
          .map((p) => p.text)
          .join("") || ""
        const imageUrls = this.pendingImageUrls
        this.pendingImageUrls = []
        await sessionApi.sendMessage(
          this.sessionId,
          text,
          imageUrls.length > 0 ? imageUrls : undefined,
        )
      }
    }

    const token = localStorage.getItem(TOKEN_KEY)
    const url = `${sessionApi.streamUrl(this.sessionId)}${token ? `?token=${encodeURIComponent(token)}` : ""}`
    const res = await fetch(url, {
      signal: abortSignal,
      headers: { Accept: "text/event-stream" },
    })

    if (!res.ok) {
      throw new Error(`Stream request failed: ${res.status}`)
    }

    return createStreamFromSSE(res, this.onUsage)
  }

  async reconnectToStream(options: ReconnectOptions) {
    const { abortSignal } = options as { abortSignal?: AbortSignal }
    const token = localStorage.getItem(TOKEN_KEY)
    const url = `${sessionApi.streamUrl(this.sessionId)}${token ? `?token=${encodeURIComponent(token)}` : ""}`
    const res = await fetch(url, {
      signal: abortSignal,
      headers: { Accept: "text/event-stream" },
    })

    if (!res.ok) {
      throw new Error(`Stream reconnect failed: ${res.status}`)
    }

    return createStreamFromSSE(res, this.onUsage)
  }
}

function fastSnapshot(message: UIMessage): UIMessage {
  return {
    ...message,
    parts: message.parts.map((part) => ({ ...part })),
  } as UIMessage
}

export interface UseAiChatOptions {
  onFinish?: () => void
  onError?: (error: Error) => void
}

export interface UseAiChatReturn extends UseChatHelpers<UIMessage> {
  setPendingImageUrls: (urls: string[]) => void
  lastUsage: { promptTokens: number; completionTokens: number }
}

export function useAiChat(
  sessionId: number,
  initialSessionMessages?: SessionMessage[],
  options?: UseAiChatOptions,
): UseAiChatReturn {
  const [lastUsage, setLastUsage] = useState({ promptTokens: 0, completionTokens: 0 })

  const transport = useMemo(
    () => new SessionTransport(sessionId, setLastUsage),
    [sessionId],
  )

  const setPendingImageUrls = useCallback(
    (urls: string[]) => {
      transport.setPendingImageUrls(urls)
    },
    [transport],
  )

  const chatInstance = useMemo(() => {
    const chat = new Chat<UIMessage>({
      id: String(sessionId),
      transport,
    })
    // Override expensive structuredClone with fast shallow clone
    // to eliminate per-chunk cloning bottleneck during streaming.
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    ;(chat as any).state.snapshot = fastSnapshot
    return chat
  }, [sessionId, transport])

  // Sync dynamic callbacks without recreating the Chat instance
  useEffect(() => {
    ;(chatInstance as any).onFinish = options?.onFinish
    ;(chatInstance as any).onError = options?.onError
  }, [chatInstance, options?.onFinish, options?.onError])

  const chat = useChat({
    chat: chatInstance,
  })

  // Sync server-loaded messages when useChat doesn't pick them up on mount
  const { messages: chatMessages, setMessages: chatSetMessages } = chat
  useEffect(() => {
    if (initialSessionMessages && chatMessages.length === 0) {
      chatSetMessages(sessionMessagesToUIMessages(initialSessionMessages))
    }
  }, [initialSessionMessages, chatMessages.length, chatSetMessages])

  return {
    ...chat,
    setPendingImageUrls,
    lastUsage,
  }
}

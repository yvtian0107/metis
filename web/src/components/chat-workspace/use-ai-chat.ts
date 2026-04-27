"use client";

import { useCallback, useEffect, useMemo, useState } from "react"
import { Chat, useChat, type UseChatHelpers } from "@ai-sdk/react"
import type { ChatTransport, UIMessage } from "ai"
import { sessionApi, type SessionMessage } from "@/lib/api"
import { TOKEN_KEY } from "@/lib/constants"
import { storedSessionMessageUIId } from "./message-id"
import { createStreamFromSSE } from "./sse-stream"

export function sessionMessagesToUIMessages(messages: SessionMessage[]): UIMessage[] {
  const seenIds = new Map<string, number>()
  return messages.map((m) => {
    const baseId = storedSessionMessageUIId(m)
    const seenCount = seenIds.get(baseId) ?? 0
    seenIds.set(baseId, seenCount + 1)
    const base: UIMessage = {
      id: seenCount === 0 ? baseId : `${baseId}-${seenCount + 1}`,
      role: m.role === "user" ? "user" : "assistant",
      metadata: { originalRole: m.role, ...m.metadata },
      parts: [{ type: "text", text: m.content || "", state: "done" }],
    }

    const uiSurface = m.metadata?.ui_surface
    if (uiSurface && typeof uiSurface === "object") {
      base.parts.push({
        type: "data-ui-surface",
        data: uiSurface,
      } as UIMessage["parts"][number])
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

export function createOptimisticUserMessage({
  text,
  images = [],
}: {
  text: string
  images?: string[]
}): UIMessage {
  return {
    id: `optimistic-${Date.now()}-${Math.random().toString(36).slice(2)}`,
    role: "user",
    metadata: images.length > 0 ? { images } : undefined,
    parts: [
      { type: "text", text },
      ...images.map((url) => ({ type: "file" as const, url, mediaType: "image/*" })),
    ],
  }
}

function sessionMessagesSignature(messages: SessionMessage[] | undefined) {
  if (!messages) return ""
  return messages
    .map((message) => {
      const metadata = message.metadata ? JSON.stringify(message.metadata) : ""
      return `${message.id}:${message.sequence}:${message.role}:${message.content}:${metadata}`
    })
    .join("|")
}

function uiMessagesSignature(messages: UIMessage[]) {
  return messages
    .map((message, index) => {
      const meta = message.metadata as { originalRole?: string } | undefined
      const textLength = message.parts
        .filter((part): part is { type: "text"; text: string } => part.type === "text")
        .reduce((total, part) => total + part.text.length, 0)
      return `${message.id}:${index}:${meta?.originalRole ?? message.role}:${textLength}`
    })
    .join("|")
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
      onFinish: options?.onFinish,
      onError: options?.onError,
    })
    // Override expensive structuredClone with fast shallow clone
    // to eliminate per-chunk cloning bottleneck during streaming.
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    ;(chat as any).state.snapshot = fastSnapshot
    return chat
  }, [sessionId, transport, options?.onFinish, options?.onError])

  const chat = useChat({
    chat: chatInstance,
  })

  // Sync server-loaded messages as the idle authoritative history.
  // During submitted/streaming states, the local optimistic and streamed messages own the screen.
  const { messages: chatMessages, setMessages: chatSetMessages, status: chatStatus } = chat
  const serverMessagesSignature = useMemo(
    () => sessionMessagesSignature(initialSessionMessages),
    [initialSessionMessages],
  )
  const localMessagesSignature = useMemo(
    () => uiMessagesSignature(chatMessages),
    [chatMessages],
  )
  useEffect(() => {
    if (!initialSessionMessages) return
    if (chatStatus === "submitted" || chatStatus === "streaming") return

    const nextMessages = sessionMessagesToUIMessages(initialSessionMessages)
    const nextSignature = uiMessagesSignature(nextMessages)
    if (nextSignature !== localMessagesSignature) {
      chatSetMessages(nextMessages)
    }
  }, [
    initialSessionMessages,
    serverMessagesSignature,
    localMessagesSignature,
    chatSetMessages,
    chatStatus,
  ])

  return {
    ...chat,
    setPendingImageUrls,
    lastUsage,
  }
}

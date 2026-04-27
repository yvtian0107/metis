"use client"

import { useEffect, useMemo, useRef } from "react"
import { useChat, Chat, type UseChatHelpers } from "@ai-sdk/react"
import { useAISDKRuntime } from "@assistant-ui/react-ai-sdk"
import { DefaultChatTransport, type UIMessage } from "ai"

import { api, sessionApi, type SessionMessage } from "@/lib/api"
import { sessionMessagesToUIMessages } from "@/components/chat-workspace"
import { ensureUniqueUIMessageIds } from "@/components/chat-workspace/message-id"
import {
  doesServiceDeskHistoryCoverLiveMessages,
  shouldProcessServiceDeskHistorySnapshot,
  shouldSyncServiceDeskHistory,
} from "./service-desk-chat-sync"

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
      const partTypes = message.parts.map((part) => part.type).join(",")
      return `${message.id}:${index}:${meta?.originalRole ?? message.role}:${textLength}:${partTypes}`
    })
    .join("|")
}

function fastSnapshot(message: UIMessage): UIMessage {
  return {
    ...message,
    parts: message.parts.map((part) => ({ ...part })),
  } as UIMessage
}

export interface UseServiceDeskChatOptions {
  onFinish?: () => void
  onError?: (error: Error) => void
}

export function useServiceDeskChat(
  sessionId: number,
  initialSessionMessages?: SessionMessage[],
  options?: UseServiceDeskChatOptions,
) {
  const syncedServerSnapshotRef = useRef("")

  const initialSignature = useMemo(
    () => sessionMessagesSignature(initialSessionMessages),
    [initialSessionMessages],
  )
  const serverMessages = useMemo(
    () => sessionMessagesToUIMessages(initialSessionMessages ?? []),
    [initialSessionMessages],
  )
  const serverMessagesSignature = useMemo(
    () => uiMessagesSignature(serverMessages),
    [serverMessages],
  )

  const transport = useMemo(() => {
    const authenticatedFetch: typeof fetch = (input, init) =>
      api.fetch(input instanceof Request ? input.url : String(input), init)

    return new DefaultChatTransport<UIMessage>({
      api: sessionApi.chatUrl(sessionId),
      fetch: authenticatedFetch,
    })
  }, [sessionId])

  const chatInstance = useMemo(() => {
    const chat = new Chat<UIMessage>({
      id: String(sessionId),
      transport,
      onFinish: options?.onFinish,
      onError: options?.onError,
    })
    ;(chat as unknown as { state: { snapshot: typeof fastSnapshot } }).state.snapshot = fastSnapshot
    return chat
  }, [sessionId, transport, options?.onFinish, options?.onError])

  const chat = useChat({
    chat: chatInstance,
  }) as UseChatHelpers<UIMessage>
  const runtimeMessages = useMemo(
    () => ensureUniqueUIMessageIds(chat.messages),
    [chat.messages],
  )
  const runtimeChat = useMemo(
    () => ({ ...chat, messages: runtimeMessages }) as UseChatHelpers<UIMessage>,
    [chat, runtimeMessages],
  )
  const runtime = useAISDKRuntime(runtimeChat)
  const localMessagesSignature = useMemo(
    () => uiMessagesSignature(chat.messages),
    [chat.messages],
  )

  const { messages: chatMessages, setMessages: setChatMessages, status: chatStatus } = chat
  useEffect(() => {
    const serverSnapshotKey = `${sessionId}:${initialSignature}`
    if (
      !shouldProcessServiceDeskHistorySnapshot({
        status: chatStatus,
        hasServerSnapshot: initialSessionMessages !== undefined,
        serverCoversLiveMessages: doesServiceDeskHistoryCoverLiveMessages(serverMessages, chatMessages),
        serverSnapshotKey,
        syncedServerSnapshotKey: syncedServerSnapshotRef.current,
      })
    ) {
      return
    }

    syncedServerSnapshotRef.current = serverSnapshotKey
    if (
      shouldSyncServiceDeskHistory({
        status: chatStatus,
        hasServerSnapshot: true,
        serverSignature: serverMessagesSignature,
        localSignature: localMessagesSignature,
      })
    ) {
      setChatMessages(serverMessages)
    }
  }, [
    chatStatus,
    chatMessages,
    initialSignature,
    initialSessionMessages,
    localMessagesSignature,
    serverMessages,
    serverMessagesSignature,
    sessionId,
    setChatMessages,
  ])

  return { chat: runtimeChat, runtime }
}

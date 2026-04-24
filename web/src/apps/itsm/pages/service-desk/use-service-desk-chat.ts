"use client"

import { useEffect, useMemo, useRef } from "react"
import { useChat, Chat, type UseChatHelpers } from "@ai-sdk/react"
import { useAISDKRuntime } from "@assistant-ui/react-ai-sdk"
import { DefaultChatTransport, type ChatStatus, type UIMessage } from "ai"

import { api, sessionApi, type SessionMessage } from "@/lib/api"
import { sessionMessagesToUIMessages } from "@/components/chat-workspace"

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

export function shouldSyncServiceDeskHistory({
  status,
  hasServerSnapshot,
  serverSignature,
  localSignature,
}: {
  status: ChatStatus
  hasServerSnapshot: boolean
  serverSignature: string
  localSignature: string
}) {
  if (!hasServerSnapshot) return false
  if (status === "submitted" || status === "streaming") return false
  return serverSignature !== localSignature
}

export function useServiceDeskChat(
  sessionId: number,
  initialSessionMessages?: SessionMessage[],
  options?: UseServiceDeskChatOptions,
) {
  const optionsRef = useRef(options)
  useEffect(() => {
    optionsRef.current = options
  }, [options])

  const initialSignature = useMemo(
    () => sessionMessagesSignature(initialSessionMessages),
    [initialSessionMessages],
  )
  const serverMessages = useMemo(
    () => sessionMessagesToUIMessages(initialSessionMessages ?? []),
    [initialSessionMessages, initialSignature],
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
      onFinish: () => optionsRef.current?.onFinish?.(),
      onError: (error) => optionsRef.current?.onError?.(error),
    })
    ;(chat as unknown as { state: { snapshot: typeof fastSnapshot } }).state.snapshot = fastSnapshot
    return chat
  }, [sessionId, transport])

  const chat = useChat({
    chat: chatInstance,
  }) as UseChatHelpers<UIMessage>
  const runtime = useAISDKRuntime(chat)
  const localMessagesSignature = useMemo(
    () => uiMessagesSignature(chat.messages),
    [chat.messages],
  )

  const { setMessages: setChatMessages, status: chatStatus } = chat
  useEffect(() => {
    if (
      !shouldSyncServiceDeskHistory({
        status: chatStatus,
        hasServerSnapshot: initialSessionMessages !== undefined,
        serverSignature: serverMessagesSignature,
        localSignature: localMessagesSignature,
      })
    ) {
      return
    }

    setChatMessages(serverMessages)
  }, [
    chatStatus,
    initialSessionMessages,
    localMessagesSignature,
    serverMessages,
    serverMessagesSignature,
    setChatMessages,
  ])

  return { chat, runtime }
}

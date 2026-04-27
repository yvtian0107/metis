import type { UIMessage } from "ai"

function messageText(message: UIMessage) {
  return message.parts
    ?.filter((part): part is { type: "text"; text: string } => part.type === "text")
    .map((part) => part.text)
    .join("") || ""
}

function userMessageSignature(message: UIMessage) {
  if (message.role !== "user") return ""
  const text = messageText(message)
  const images = (message.metadata as { images?: string[] } | undefined)?.images ?? []
  const fileImageCount = message.parts?.filter((part) => part.type === "file").length ?? 0
  return `user:${text}::${Math.max(images.length, fileImageCount)}`
}

function storedToolIdentity(meta: { originalRole?: string; tool_call_id?: string }, fallbackId: string) {
  const toolCallId = meta.tool_call_id || fallbackId
  if (meta.originalRole === "tool_call") return `stored-tool-call:${toolCallId}`
  if (meta.originalRole === "tool_result") return `stored-tool-result:${toolCallId}`
  return ""
}

function messageCoverageKeys(message: UIMessage) {
  const meta = message.metadata as {
    originalRole?: string
    tool_call_id?: string
  } | undefined
  const keys: string[] = []

  if (message.role === "user") {
    const signature = userMessageSignature(message)
    if (signature) keys.push(signature)
    return keys
  }

  if (meta?.originalRole === "tool_call" || meta?.originalRole === "tool_result") {
    const storedKey = storedToolIdentity(meta, String(message.id))
    if (storedKey) keys.push(storedKey)
    if (meta.tool_call_id) keys.push(`live-tool:${meta.tool_call_id}`)
    return keys
  }

  const text = messageText(message).trim()
  if (text) keys.push(`assistant-text:${text}`)

  for (const part of message.parts ?? []) {
    if (part.type === "dynamic-tool" || part.type.startsWith("tool-")) {
      const toolCallId = (part as { toolCallId?: string }).toolCallId
      if (toolCallId) keys.push(`live-tool:${toolCallId}`)
    }
    if (part.type.startsWith("data-")) {
      const data = (part as { data?: unknown }).data
      if (data && typeof data === "object" && "surfaceId" in data) {
        const surfaceId = (data as { surfaceId?: unknown }).surfaceId
        if (typeof surfaceId === "string" && surfaceId) {
          keys.push(`surface:${surfaceId}`)
          continue
        }
      }
      keys.push(`${part.type}:${JSON.stringify(data ?? {})}`)
    }
  }

  if (keys.length === 0) keys.push(`${message.role}:${message.id}`)
  return keys
}

function messageIdentityKeys(message: UIMessage) {
  const meta = message.metadata as {
    originalRole?: string
    tool_call_id?: string
  } | undefined

  if (meta?.originalRole === "tool_call" || meta?.originalRole === "tool_result") {
    const storedKey = storedToolIdentity(meta, String(message.id))
    return storedKey ? [storedKey] : [`stored-tool:${message.id}`]
  }

  return messageCoverageKeys(message)
}

function addMessageKeys(keys: Set<string>, message: UIMessage) {
  for (const key of messageIdentityKeys(message)) {
    keys.add(key)
  }
}

function isMessageRepresented(message: UIMessage, keys: Set<string>) {
  const messageKeys = messageCoverageKeys(message)
  return messageKeys.length > 0 && messageKeys.some((key) => keys.has(key))
}

export function mergeTimelineMessages(
  serverMessages: UIMessage[],
  liveMessages: UIMessage[],
  pendingUserMessages: UIMessage[] = [],
) {
  const merged: UIMessage[] = []
  const keys = new Set<string>()

  for (const message of liveMessages) {
    if (isMessageRepresented(message, keys)) continue
    merged.push(message)
    addMessageKeys(keys, message)
  }

  for (const message of serverMessages) {
    if (isMessageRepresented(message, keys)) continue
    merged.push(message)
    addMessageKeys(keys, message)
  }

  for (const message of pendingUserMessages) {
    if (isMessageRepresented(message, keys)) continue
    merged.push(message)
    addMessageKeys(keys, message)
  }

  return merged
}

export function hasUnmatchedPendingUserMessages(messages: UIMessage[], pendingUserMessages: UIMessage[]) {
  if (pendingUserMessages.length === 0) return false
  const keys = new Set<string>()
  for (const message of messages) addMessageKeys(keys, message)
  return pendingUserMessages.some((message) => !isMessageRepresented(message, keys))
}

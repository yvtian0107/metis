import type { ChatStatus, UIMessage } from "ai"

function messageText(message: UIMessage) {
  return message.parts
    ?.filter((part): part is { type: "text"; text: string } => part.type === "text")
    .map((part) => part.text)
    .join("") || ""
}

function userMessageKey(message: UIMessage) {
  const images = (message.metadata as { images?: string[] } | undefined)?.images ?? []
  const fileImageCount = message.parts?.filter((part) => part.type === "file").length ?? 0
  return `user:${messageText(message)}::${Math.max(images.length, fileImageCount)}`
}

function dataPartKey(part: UIMessage["parts"][number], fallback: number) {
  const data = (part as { data?: unknown }).data
  if (data && typeof data === "object" && "surfaceId" in data) {
    const surfaceId = (data as { surfaceId?: unknown }).surfaceId
    if (typeof surfaceId === "string" && surfaceId) return `surface:${surfaceId}`
  }
  return `${part.type}:${JSON.stringify(data ?? {})}:${fallback}`
}

function liveToolPartKeys(part: UIMessage["parts"][number]) {
  if (!(part.type === "dynamic-tool" || part.type.startsWith("tool-"))) return []
  const toolCallId = (part as { toolCallId?: string }).toolCallId
  if (!toolCallId) return []
  const state = (part as { state?: string }).state
  if (state === "output-available" || state === "output-error") {
    return [`tool-call:${toolCallId}`, `tool-result:${toolCallId}`]
  }
  return [`tool-call:${toolCallId}`]
}

function serverMessageCoverageKeys(message: UIMessage) {
  const meta = message.metadata as { originalRole?: string; tool_call_id?: string } | undefined
  if (message.role === "user") return [userMessageKey(message)]
  if (meta?.originalRole === "tool_call" && meta.tool_call_id) return [`tool-call:${meta.tool_call_id}`]
  if (meta?.originalRole === "tool_result" && meta.tool_call_id) return [`tool-result:${meta.tool_call_id}`]

  const keys: string[] = []
  const text = messageText(message).trim()
  if (text) keys.push(`assistant-text:${text}`)
  message.parts?.forEach((part, index) => {
    if (part.type.startsWith("data-")) keys.push(dataPartKey(part, index))
  })
  return keys
}

function liveMessageCoverageKeys(message: UIMessage) {
  if (message.role === "user") return [userMessageKey(message)]

  const keys: string[] = []
  const text = messageText(message).trim()
  if (text) keys.push(`assistant-text:${text}`)
  message.parts?.forEach((part, index) => {
    keys.push(...liveToolPartKeys(part))
    if (part.type.startsWith("data-")) keys.push(dataPartKey(part, index))
  })
  return keys
}

export function doesServiceDeskHistoryCoverLiveMessages(
  serverMessages: UIMessage[],
  liveMessages: UIMessage[],
) {
  const serverKeys = new Set(serverMessages.flatMap(serverMessageCoverageKeys))
  return liveMessages.every((message) => {
    const keys = liveMessageCoverageKeys(message)
    return keys.length === 0 || keys.every((key) => serverKeys.has(key))
  })
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

export function shouldProcessServiceDeskHistorySnapshot({
  status,
  hasServerSnapshot,
  serverCoversLiveMessages,
  serverSnapshotKey,
  syncedServerSnapshotKey,
}: {
  status: ChatStatus
  hasServerSnapshot: boolean
  serverCoversLiveMessages: boolean
  serverSnapshotKey: string
  syncedServerSnapshotKey: string
}) {
  if (!hasServerSnapshot) return false
  if (status === "submitted" || status === "streaming") return false
  if (!serverCoversLiveMessages) return false
  return serverSnapshotKey !== syncedServerSnapshotKey
}

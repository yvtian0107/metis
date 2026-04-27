import type { UIMessage } from "ai"

type StoredSessionMessageIdentity = {
  id: number
  sessionId: number
  role: string
  sequence: number
}

export function storedSessionMessageUIId(message: StoredSessionMessageIdentity) {
  return `stored-${message.sessionId}-${message.sequence}-${message.role}-${message.id}`
}

export function ensureUniqueUIMessageIds(messages: UIMessage[]) {
  const seen = new Map<string, number>()
  let changed = false

  const next = messages.map((message) => {
    const id = String(message.id)
    const count = seen.get(id) ?? 0
    seen.set(id, count + 1)
    if (count === 0) return message

    changed = true
    return {
      ...message,
      id: `${id}#${count + 1}`,
    } satisfies UIMessage
  })

  return changed ? next : messages
}

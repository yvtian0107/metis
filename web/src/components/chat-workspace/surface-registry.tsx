import type { UIMessage } from "ai"
import type { ChatWorkspaceActions, ChatWorkspaceSurfaceContext, ChatWorkspaceSurfaceRenderer } from "./types"
import type { AgentSession } from "@/lib/api"

function readSurfaceType(part: UIMessage["parts"][number]) {
  const data = (part as { data?: unknown }).data
  if (!data || typeof data !== "object") return null
  const surfaceType = (data as { surfaceType?: unknown }).surfaceType
  return typeof surfaceType === "string" ? surfaceType : null
}

export function createSurfacePartRenderer({
  renderers,
  session,
  actions,
}: {
  renderers?: ChatWorkspaceSurfaceRenderer[]
  session?: AgentSession
  actions: ChatWorkspaceActions
}) {
  const rendererMap = new Map((renderers ?? []).map((renderer) => [renderer.surfaceType, renderer]))

  return {
    shouldSuppressText(part: UIMessage["parts"][number]) {
      const surfaceType = readSurfaceType(part)
      return Boolean(surfaceType && rendererMap.get(surfaceType)?.suppressText)
    },
    render(part: UIMessage["parts"][number], message: UIMessage) {
      const surfaceType = readSurfaceType(part)
      if (!surfaceType) return null
      const renderer = rendererMap.get(surfaceType)
      if (!renderer) return null
      const context: ChatWorkspaceSurfaceContext = { part, message, session, actions }
      return renderer.render(context)
    },
  }
}

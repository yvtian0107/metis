export function recoverInitialPromptDraft<TImage>(
  initialPrompt: string | undefined,
  initialImages: readonly TImage[] | undefined,
) {
  return {
    input: initialPrompt ?? "",
    images: [...(initialImages ?? [])],
  }
}

type StaffingHealthStatus = "pass" | "warn" | "fail"

interface ServiceDeskStaffingConfig {
  posts: {
    intake: {
      agentId: number
      agentName?: string
    }
  }
  health: {
    items: Array<{
      key: string
      status: StaffingHealthStatus
      message?: string
    }>
  }
}

export type ServiceDeskStaffingState = {
  ready: boolean
  agentId: number
  agentName: string
  reason: "loading" | "config_error" | "missing" | "unhealthy" | "ready"
  message?: string
}

function errorMessage(error: unknown) {
  return error instanceof Error ? error.message : "配置读取失败"
}

export function resolveServiceDeskStaffingState(
  config?: ServiceDeskStaffingConfig,
  query?: { loading?: boolean; error?: unknown },
): ServiceDeskStaffingState {
  if (query?.loading) {
    return { ready: false, agentId: 0, agentName: "IT 服务台", reason: "loading" }
  }
  if (query?.error) {
    return { ready: false, agentId: 0, agentName: "IT 服务台", reason: "config_error", message: errorMessage(query.error) }
  }

  const intake = config?.posts.intake
  const agentId = intake?.agentId ?? 0
  const agentName = intake?.agentName || "IT 服务台"
  if (!config || agentId <= 0) {
    return { ready: false, agentId: 0, agentName, reason: "missing" }
  }

  const health = config?.health.items.find((item) => item.key === "intake")
  if (health?.status !== "pass") {
    return {
      ready: false,
      agentId,
      agentName,
      reason: "unhealthy",
      message: health?.message || "服务受理岗未就绪",
    }
  }

  return { ready: true, agentId, agentName, reason: "ready" }
}

export function createServiceDeskWorkspaceActions({
  regenerate,
  clearError,
  continueGeneration,
  cancel,
}: {
  regenerate: () => void
  clearError: () => void
  continueGeneration: () => void
  cancel: () => void
}): {
  regenerate: () => void
  retry: () => void
  continueGeneration: () => void
  cancel: () => void
} {
  return {
    regenerate,
    retry: () => {
      clearError()
      regenerate()
    },
    continueGeneration,
    cancel,
  }
}

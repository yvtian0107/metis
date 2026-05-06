import type { ServiceDefListParams } from "./api"

export const itsmQueryKeys = {
  catalogs: {
    all: ["itsm", "catalogs"] as const,
    tree: () => [...itsmQueryKeys.catalogs.all, "tree"] as const,
    serviceCounts: () => [...itsmQueryKeys.catalogs.all, "service-counts"] as const,
  },
  services: {
    all: ["itsm", "services"] as const,
    lists: () => [...itsmQueryKeys.services.all, "list"] as const,
    list: (params: ServiceDefListParams) => [...itsmQueryKeys.services.lists(), params] as const,
    detail: (serviceId: number) => [...itsmQueryKeys.services.all, "detail", serviceId] as const,
    actions: (serviceId: number) => [...itsmQueryKeys.services.all, "actions", serviceId] as const,
  },
  priorities: {
    all: ["itsm", "priorities"] as const,
  },
  sla: {
    all: ["itsm", "sla"] as const,
    escalations: (slaId: number) => [...itsmQueryKeys.sla.all, slaId, "escalations"] as const,
    notificationChannels: ["itsm", "sla", "notification-channels"] as const,
  },
  workflows: {
    all: ["itsm", "workflows"] as const,
    capabilities: () => [...itsmQueryKeys.workflows.all, "capabilities"] as const,
  },
}

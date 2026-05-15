import type { TFunction } from "i18next"

export const SERVICE_ACTION_TYPE_HTTP = "http" as const

export const SERVICE_ACTION_TYPE_OPTIONS = [
  { value: SERVICE_ACTION_TYPE_HTTP, labelKey: "itsm:actions.webhook" },
] as const

export function formatServiceActionType(actionType: string, t: TFunction<"itsm">) {
  const option = SERVICE_ACTION_TYPE_OPTIONS.find((item) => item.value === actionType)
  if (!option) return actionType
  return t(option.labelKey)
}

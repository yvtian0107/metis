export const ACTIVE_MENU_PERMISSION_STATE_KEY = "activeMenuPermission"

export interface ActiveMenuLocationState {
  activeMenuPermission: string
}

export function withActiveMenuPermission(permission: string): ActiveMenuLocationState {
  return { [ACTIVE_MENU_PERMISSION_STATE_KEY]: permission }
}

export function getActiveMenuPermission(state: unknown): string | null {
  if (!state || typeof state !== "object") return null

  const value = (state as Record<string, unknown>)[ACTIVE_MENU_PERMISSION_STATE_KEY]
  return typeof value === "string" && value.length > 0 ? value : null
}

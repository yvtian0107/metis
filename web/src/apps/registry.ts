import type { RouteObject } from "react-router"

export interface MenuGroup {
  label: string
  items: string[]
}

export interface NavGroupItem {
  permission: string
}

export interface NavGroup {
  label: string
  items: NavGroupItem[]
}

export interface AppModule {
  name: string
  routes: RouteObject[]
  menuGroups?: MenuGroup[]
  navigation?: NavGroup[]
}

const modules: AppModule[] = []

export function registerApp(m: AppModule) {
  modules.push(m)
}

export function getAppRoutes(): RouteObject[] {
  return modules.flatMap((m) => m.routes)
}

export function getAppNavigation(appName: string): NavGroup[] | undefined {
  const module = modules.find((m) => m.name === appName)
  if (!module) return undefined
  if (module.navigation && module.navigation.length > 0) return module.navigation
  if (!module.menuGroups || module.menuGroups.length === 0) return undefined
  return module.menuGroups.map((group) => ({
    label: group.label,
    items: group.items.map((permission) => ({ permission })),
  }))
}

// App module imports are in _bootstrap.ts to avoid circular dependency.
// gen-registry.sh manages _bootstrap.ts for filtered builds.

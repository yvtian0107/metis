import { useMemo } from "react"
import { useLocation, useNavigate } from "react-router"
import { useQuery } from "@tanstack/react-query"
import { useTranslation } from "react-i18next"
import { Info } from "lucide-react"
import { useMenuStore, type MenuItem } from "@/stores/menu"
import { useUiStore } from "@/stores/ui"
import { getIcon } from "@/lib/icon-map"
import { cn } from "@/lib/utils"
import { getActiveMenuPermission } from "@/lib/navigation-state"
import { getAppNavigation } from "@/apps/registry"
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip"
import { api, type SiteInfo } from "@/lib/api"

interface NavApp {
  id: number
  label: string
  icon: string
  path: string
  permission: string
  children: MenuItem[]
  leafItems: MenuItem[]
}

interface NavSection {
  key: string | null
  label: string | null
  items: MenuItem[]
}

interface ResolvedNavSection {
  key: string
  label: string | null
  items: MenuItem[]
}

function getVisibleNavChildren(items: MenuItem[] | undefined): MenuItem[] {
  return (items || []).filter((item) => item.type !== "button" && !item.isHidden)
}

function collectLeafMenus(items: MenuItem[] | undefined): MenuItem[] {
  const leaves: MenuItem[] = []
  for (const item of getVisibleNavChildren(items)) {
    if (item.type === "menu") {
      leaves.push(item)
      continue
    }
    leaves.push(...collectLeafMenus(item.children))
  }
  return leaves
}

function getFirstLeafPath(items: MenuItem[] | undefined): string {
  return collectLeafMenus(items).find((item) => item.path)?.path || "/"
}

function findBestLeaf(items: MenuItem[], pathname: string): MenuItem | null {
  for (const item of items) {
    if (item.path && pathname === item.path) return item
  }

  let best: MenuItem | null = null
  let bestLen = 0
  for (const item of items) {
    if (!item.path || item.path === "/") continue
    if (pathname.startsWith(item.path) && item.path.length > bestLen) {
      best = item
      bestLen = item.path.length
    }
  }

  return best
}

function findLeafByPermission(items: MenuItem[], permission: string | null): MenuItem | null {
  if (!permission) return null
  return items.find((item) => item.permission === permission) ?? null
}

function findActiveLeaf(items: MenuItem[], pathname: string, activeMenuPermission: string | null): MenuItem | null {
  const exact = items.find((item) => item.path && pathname === item.path)
  if (exact) return exact

  return findLeafByPermission(items, activeMenuPermission) ?? findBestLeaf(items, pathname)
}

function buildNavApps(menuTree: MenuItem[]): NavApp[] {
  const apps: NavApp[] = []
  for (const item of menuTree) {
    if (item.isHidden) continue
    if (item.type === "directory") {
      const children = getVisibleNavChildren(item.children)
      const leafItems = collectLeafMenus(children)
      const firstPath = getFirstLeafPath(children)
      apps.push({
        id: item.id,
        label: item.name,
        icon: item.icon,
        path: firstPath,
        permission: item.permission,
        children,
        leafItems,
      })
    } else if (item.type === "menu") {
      apps.push({
        id: item.id,
        label: item.name,
        icon: item.icon,
        path: item.path || "/",
        permission: item.permission,
        children: [item],
        leafItems: [item],
      })
    }
  }
  return apps
}

function findActiveNavApp(apps: NavApp[], pathname: string): NavApp | null {
  for (const app of apps) {
    if (findBestLeaf(app.leafItems, pathname)) return app
  }

  let best: NavApp | null = null
  let bestLen = 0
  for (const app of apps) {
    const match = findBestLeaf(app.leafItems, pathname)
    if (match?.path && match.path.length > bestLen) {
      best = app
      bestLen = match.path.length
    }
  }

  return best ?? apps[0] ?? null
}

function buildSections(app: NavApp | null): NavSection[] {
  if (!app) return []

  const leafByPermission = new Map(app.leafItems.map((item) => [item.permission, item]))
  const navigation = getAppNavigation(app.permission)
  if (navigation && navigation.length > 0) {
    const grouped = new Set<string>()
    const sections: NavSection[] = navigation
      .map((group) => {
        const items = group.items
          .map((item) => leafByPermission.get(item.permission))
          .filter((item): item is MenuItem => Boolean(item))
        items.forEach((item) => grouped.add(item.permission))
        return {
          key: group.label,
          label: group.label,
          items,
        }
      })
      .filter((section) => section.items.length > 0)

    const ungrouped = app.leafItems.filter((item) => !grouped.has(item.permission))
    if (ungrouped.length > 0) sections.push({ key: null, label: null, items: ungrouped })
    return sections
  }

  const sections: NavSection[] = []
  const ungrouped: MenuItem[] = []
  for (const child of app.children) {
    if (child.type === "directory") {
      const items = collectLeafMenus(child.children)
      if (items.length > 0) {
        sections.push({
          key: child.permission || String(child.id),
          label: child.name,
          items,
        })
      }
      continue
    }
    ungrouped.push(child)
  }

  if (ungrouped.length > 0) sections.unshift({ key: null, label: null, items: ungrouped })
  if (sections.length > 0) return sections
  return [{ key: null, label: null, items: app.leafItems }]
}

function resolveSectionKey(section: NavSection, index: number): string {
  return section.key ?? section.label ?? `ungrouped-${index}`
}

export function Sidebar() {
  const { t } = useTranslation("layout")
  const location = useLocation()
  const { pathname } = location
  const navigate = useNavigate()
  const collapsed = useUiStore((s) => s.sidebarCollapsed)
  const menuTree = useMenuStore((s) => s.menuTree)
  const activeMenuPermission = getActiveMenuPermission(location.state)

  const { data: siteInfo } = useQuery({
    queryKey: ["site-info"],
    queryFn: () => api.get<SiteInfo>("/api/v1/site-info"),
    staleTime: 60_000,
  })

  const navApps = useMemo(() => buildNavApps(menuTree), [menuTree])
  const activeApp = useMemo(() => findActiveNavApp(navApps, pathname), [navApps, pathname])
  const activeLeaf = useMemo(
    () => (activeApp ? findActiveLeaf(activeApp.leafItems, pathname, activeMenuPermission) : null),
    [activeApp, pathname, activeMenuPermission],
  )
  const sections = useMemo<ResolvedNavSection[]>(
    () => buildSections(activeApp).map((section, index) => ({
      ...section,
      key: resolveSectionKey(section, index),
    })),
    [activeApp],
  )

  return (
    <aside
      className={cn(
        "fixed left-0 top-14 bottom-0 z-20 flex border-r border-sidebar-border/80",
        "bg-sidebar/68 backdrop-blur-2xl",
        "transition-all duration-200",
      )}
    >
      {/* Tier 1: Icon Rail */}
      <nav className="flex w-12 flex-col items-center gap-1 border-r border-sidebar-border/80 py-3">
        {navApps.map((app) => {
          const Icon = getIcon(app.icon)
          const isActive = activeApp?.id === app.id
          return (
            <Tooltip key={app.id} delayDuration={0}>
              <TooltipTrigger asChild>
                <button
                  onClick={() => navigate(app.path)}
                  className={cn(
                    "flex h-9 w-9 items-center justify-center rounded-xl transition-colors duration-200",
                    isActive
                      ? "bg-sidebar-accent/88 text-sidebar-accent-foreground shadow-[0_14px_28px_-18px_hsl(var(--primary)/0.55)]"
                      : "text-sidebar-foreground hover:bg-white/60",
                  )}
                >
                  <Icon className="h-4 w-4" />
                </button>
              </TooltipTrigger>
              <TooltipContent side="right" sideOffset={8}>
                {t(`menu.${app.permission}`, { defaultValue: app.label, nsSeparator: false })}
              </TooltipContent>
            </Tooltip>
          )
        })}

        {siteInfo?.version && (
          <Tooltip delayDuration={0}>
            <TooltipTrigger asChild>
              <div className="mt-auto flex h-9 w-9 items-center justify-center text-sidebar-foreground/30">
                <Info className="h-3.5 w-3.5" />
              </div>
            </TooltipTrigger>
            <TooltipContent side="right" sideOffset={8}>
              {siteInfo.version}
            </TooltipContent>
          </Tooltip>
        )}
      </nav>

      {/* Tier 2/3: Grouped Nav Panel */}
      <nav
        className={cn(
          "flex flex-col gap-1 overflow-hidden py-3 transition-all duration-200",
          collapsed ? "w-0 px-0" : "w-40 px-2",
        )}
      >
        {sections.map((section, si) => (
          <div key={section.key} className={section.label && si > 0 ? "pt-2" : undefined}>
            {section.label && (
              <div className="workspace-nav-section-title px-3 pb-1">
                {t(`menuGroup.${section.label}`, { ns: activeApp?.permission, defaultValue: section.label, nsSeparator: false })}
              </div>
            )}
            {section.items.map((item) => {
              const Icon = getIcon(item.icon)
              const isActive = activeLeaf?.id === item.id
              return (
                <button
                  key={item.id}
                  onClick={() => navigate(item.path || "/")}
                  className={cn(
                    "workspace-nav-item flex w-full items-center gap-2 rounded-xl px-3 py-2 transition-colors duration-200",
                    isActive
                      ? "bg-sidebar-accent/82 text-sidebar-accent-foreground font-medium shadow-[0_12px_24px_-18px_hsl(var(--primary)/0.45)]"
                      : "text-sidebar-foreground hover:bg-white/58",
                  )}
                >
                  <Icon className="h-4 w-4 shrink-0" />
                  <span className="truncate">{t(`menu.${item.permission ?? ""}`, { defaultValue: item.name, nsSeparator: false })}</span>
                </button>
              )
            })}
          </div>
        ))}
      </nav>
    </aside>
  )
}

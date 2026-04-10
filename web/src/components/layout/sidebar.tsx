import { useMemo } from "react"
import { useLocation, useNavigate } from "react-router"
import { useQuery } from "@tanstack/react-query"
import { Info } from "lucide-react"
import { useMenuStore, type MenuItem } from "@/stores/menu"
import { useUiStore } from "@/stores/ui"
import { getIcon } from "@/lib/icon-map"
import { cn } from "@/lib/utils"
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
  children: MenuItem[]
}

function buildNavApps(menuTree: MenuItem[]): NavApp[] {
  const apps: NavApp[] = []
  for (const item of menuTree) {
    if (item.isHidden) continue
    if (item.type === "directory") {
      // Directory becomes a tier-1 app, its menu children become tier-2 items
      const children = (item.children || []).filter(
        (c) => c.type === "menu" && !c.isHidden,
      )
      const firstPath = children[0]?.path || "/"
      apps.push({
        id: item.id,
        label: item.name,
        icon: item.icon,
        path: firstPath,
        children,
      })
    } else if (item.type === "menu") {
      // Top-level menu becomes both tier-1 and tier-2
      apps.push({
        id: item.id,
        label: item.name,
        icon: item.icon,
        path: item.path || "/",
        children: [item],
      })
    }
  }
  return apps
}

function findActiveNavApp(apps: NavApp[], pathname: string): NavApp | null {
  // Check children paths for exact or prefix match
  for (const app of apps) {
    for (const child of app.children) {
      if (child.path && pathname === child.path) return app
    }
  }
  // Fallback: match by longest prefix
  let best: NavApp | null = null
  let bestLen = 0
  for (const app of apps) {
    for (const child of app.children) {
      if (child.path && child.path !== "/" && pathname.startsWith(child.path) && child.path.length > bestLen) {
        best = app
        bestLen = child.path.length
      }
    }
  }
  return best ?? apps[0] ?? null
}

export function Sidebar() {
  const { pathname } = useLocation()
  const navigate = useNavigate()
  const collapsed = useUiStore((s) => s.sidebarCollapsed)
  const menuTree = useMenuStore((s) => s.menuTree)

  const { data: siteInfo } = useQuery({
    queryKey: ["site-info"],
    queryFn: () => api.get<SiteInfo>("/api/v1/site-info"),
    staleTime: 60_000,
  })

  const navApps = useMemo(() => buildNavApps(menuTree), [menuTree])
  const activeApp = useMemo(() => findActiveNavApp(navApps, pathname), [navApps, pathname])

  const visibleItems = activeApp?.children ?? []

  return (
    <aside
      className={cn(
        "fixed left-0 top-14 bottom-0 z-20 flex border-r border-sidebar-border",
        "bg-sidebar/80 backdrop-blur-2xl",
        "transition-all duration-200",
      )}
    >
      {/* Tier 1: Icon Rail */}
      <nav className="flex w-12 flex-col items-center gap-1 border-r border-sidebar-border py-3">
        {navApps.map((app) => {
          const Icon = getIcon(app.icon)
          const isActive = activeApp?.id === app.id
          return (
            <Tooltip key={app.id} delayDuration={0}>
              <TooltipTrigger asChild>
                <button
                  onClick={() => navigate(app.path)}
                  className={cn(
                    "flex h-9 w-9 items-center justify-center rounded-lg transition-colors duration-200",
                    isActive
                      ? "bg-sidebar-accent text-sidebar-accent-foreground"
                      : "text-sidebar-foreground hover:bg-black/[0.04]",
                  )}
                >
                  <Icon className="h-4 w-4" />
                </button>
              </TooltipTrigger>
              <TooltipContent side="right" sideOffset={8}>
                {app.label}
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

      {/* Tier 2: Nav Panel */}
      <nav
        className={cn(
          "flex flex-col gap-1 overflow-hidden py-3 transition-all duration-200",
          collapsed ? "w-0 px-0" : "w-40 px-2",
        )}
      >
        {visibleItems.map((item) => {
          const Icon = getIcon(item.icon)
          const isActive = pathname === item.path
          return (
            <button
              key={item.id}
              onClick={() => navigate(item.path || "/")}
              className={cn(
                "flex items-center gap-2 rounded-lg px-3 py-2 text-sm transition-colors duration-200",
                isActive
                  ? "bg-sidebar-accent text-sidebar-accent-foreground font-medium"
                  : "text-sidebar-foreground hover:bg-black/[0.04]",
              )}
            >
              <Icon className="h-4 w-4 shrink-0" />
              <span className="truncate">{item.name}</span>
            </button>
          )
        })}
      </nav>
    </aside>
  )
}

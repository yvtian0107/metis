import { useMemo } from "react"
import { useLocation } from "react-router"
import { useTranslation } from "react-i18next"
import { useMenuStore, type MenuItem } from "@/stores/menu"
import { Separator } from "@/components/ui/separator"

function buildPathLabels(menuTree: MenuItem[], t: (key: string, opts?: Record<string, unknown>) => string): Record<string, string> {
  const labels: Record<string, string> = {}
  function walk(items: MenuItem[]) {
    for (const m of items) {
      if (m.path) {
        labels[m.path] = t(`layout:menu.${m.permission ?? ""}`, { defaultValue: m.name })
      }
      if (m.children) walk(m.children)
    }
  }
  walk(menuTree)
  return labels
}

export function Header() {
  const { t } = useTranslation("layout")
  const { pathname } = useLocation()
  const menuTree = useMenuStore((s) => s.menuTree)

  const pathLabels = useMemo(() => buildPathLabels(menuTree, t), [menuTree, t])

  const segments = pathname.split("/").filter(Boolean)
  const crumbs = segments.map((seg, i) => {
    const fullPath = "/" + segments.slice(0, i + 1).join("/")
    return {
      label: pathLabels[fullPath] ?? seg,
      path: fullPath,
    }
  })

  if (crumbs.length === 0) return null

  return (
    <div className="flex h-10 items-center gap-2 px-6 text-sm text-muted-foreground">
      {crumbs.map((crumb, i) => (
        <span key={crumb.path} className="flex items-center gap-2">
          {i > 0 && <Separator orientation="vertical" className="h-4" />}
          <span className={i === crumbs.length - 1 ? "text-foreground font-medium" : ""}>
            {crumb.label}
          </span>
        </span>
      ))}
    </div>
  )
}

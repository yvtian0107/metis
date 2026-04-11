import { useEffect, useMemo, useState } from "react"
import { useTranslation } from "react-i18next"
import { useQuery, useMutation } from "@tanstack/react-query"
import { ChevronRight, ChevronDown } from "lucide-react"
import { api } from "@/lib/api"
import { getIcon } from "@/lib/icon-map"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
  SheetFooter,
} from "@/components/ui/sheet"
import type { MenuItem } from "@/stores/menu"
import type { Role } from "./types"

interface PermissionDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  role: Role | null
}

interface PermissionsData {
  menuPermissions: string[]
  apiPolicies: { path: string; method: string }[]
}

function buildPermMap(menus: MenuItem[]): Map<string, number> {
  const map = new Map<string, number>()
  function walk(items: MenuItem[]) {
    for (const m of items) {
      if (m.permission) map.set(m.permission, m.id)
      if (m.children) walk(m.children)
    }
  }
  walk(menus)
  return map
}

function getDescendantIds(menu: MenuItem): number[] {
  const ids: number[] = []
  function walk(items: MenuItem[]) {
    for (const m of items) {
      ids.push(m.id)
      if (m.children) walk(m.children)
    }
  }
  if (menu.children) walk(menu.children)
  return ids
}

function getAncestorIds(targetId: number, menus: MenuItem[]): number[] {
  const path: number[] = []
  function find(items: MenuItem[], ancestors: number[]): boolean {
    for (const m of items) {
      if (m.id === targetId) {
        path.push(...ancestors)
        return true
      }
      if (m.children && find(m.children, [...ancestors, m.id])) return true
    }
    return false
  }
  find(menus, [])
  return path
}

function collectAllIds(menus: MenuItem[]): number[] {
  const ids: number[] = []
  function walk(items: MenuItem[]) {
    for (const m of items) {
      ids.push(m.id)
      if (m.children) walk(m.children)
    }
  }
  walk(menus)
  return ids
}

/** Flatten tree into { menu, buttons[] } rows grouped by directory */
interface PermRow {
  menu: MenuItem
  buttons: MenuItem[]
}
interface PermSection {
  directory: MenuItem | null
  rows: PermRow[]
}

function buildSections(menus: MenuItem[]): PermSection[] {
  const sections: PermSection[] = []
  for (const item of menus) {
    if (item.type === "directory") {
      const rows: PermRow[] = []
      for (const child of item.children ?? []) {
        if (child.type === "menu" || child.type === "directory") {
          rows.push({
            menu: child,
            buttons: (child.children ?? []).filter((c) => c.type === "button"),
          })
        }
      }
      if (rows.length > 0) sections.push({ directory: item, rows })
    }
  }
  return sections
}

/** Top-level menu items (e.g. 首页) that are not inside a directory */
function getTopLevelMenus(menus: MenuItem[]): MenuItem[] {
  return menus.filter((m) => m.type === "menu")
}

export function PermissionDialog({ open, onOpenChange, role }: PermissionDialogProps) {
  const { t } = useTranslation(["roles", "common"])
  const [checkedIds, setCheckedIds] = useState<Set<number>>(new Set())
  const [expandedDirs, setExpandedDirs] = useState<Set<number>>(new Set())

  const { data: menuTree } = useQuery({
    queryKey: ["menus", "tree"],
    queryFn: () => api.get<MenuItem[]>("/api/v1/menus/tree"),
    enabled: open,
  })

  const { data: currentPerms } = useQuery({
    queryKey: ["roles", role?.id, "permissions"],
    queryFn: () => api.get<PermissionsData>(`/api/v1/roles/${role!.id}/permissions`),
    enabled: open && !!role,
  })

  const permMap = useMemo(
    () => (menuTree ? buildPermMap(menuTree) : new Map<string, number>()),
    [menuTree],
  )

  const sections = useMemo(
    () => (menuTree ? buildSections(menuTree) : []),
    [menuTree],
  )

  const topMenus = useMemo(
    () => (menuTree ? getTopLevelMenus(menuTree) : []),
    [menuTree],
  )

  // Expand all directories on open
  useEffect(() => {
    if (sections.length > 0) {
      setExpandedDirs(new Set(sections.map((s) => s.directory?.id).filter(Boolean) as number[]))
    }
  }, [sections])

  useEffect(() => {
    if (!currentPerms || !menuTree) return
    const ids = new Set<number>()
    for (const perm of currentPerms.menuPermissions || []) {
      const id = permMap.get(perm)
      if (id !== undefined) ids.add(id)
    }
    setCheckedIds(ids)
  }, [currentPerms, menuTree, permMap])

  function toggleDir(id: number) {
    setExpandedDirs((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  function toggleMenu(menu: MenuItem, checked: boolean) {
    setCheckedIds((prev) => {
      const next = new Set(prev)
      if (checked) {
        next.add(menu.id)
        for (const id of getDescendantIds(menu)) next.add(id)
        if (menuTree) {
          for (const id of getAncestorIds(menu.id, menuTree)) next.add(id)
        }
      } else {
        next.delete(menu.id)
        for (const id of getDescendantIds(menu)) next.delete(id)
      }
      return next
    })
  }

  function getCheckState(menu: MenuItem): boolean | "indeterminate" {
    if (!menu.children || menu.children.length === 0) return checkedIds.has(menu.id)
    const ids = getDescendantIds(menu)
    const n = ids.filter((id) => checkedIds.has(id)).length
    if (n === 0 && !checkedIds.has(menu.id)) return false
    if (n === ids.length) return true
    return "indeterminate"
  }

  const saveMutation = useMutation({
    mutationFn: () =>
      api.put(`/api/v1/roles/${role!.id}/permissions`, {
        menuIds: Array.from(checkedIds),
        apiPolicies: currentPerms?.apiPolicies || [],
      }),
    onSuccess: () => onOpenChange(false),
  })

  const totalCount = menuTree ? collectAllIds(menuTree).length : 0
  const checkedCount = checkedIds.size

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className="sm:max-w-[440px] flex flex-col gap-0 p-0">
        <SheetHeader className="px-6 pt-6 pb-3 border-b">
          <SheetTitle className="text-base">{t("roles:permission.title", { name: role?.name })}</SheetTitle>
          <SheetDescription className="text-xs mt-1">
            {t("roles:permission.selectedCount", { checked: checkedCount, total: totalCount })}
          </SheetDescription>
        </SheetHeader>

        <div className="flex-1 overflow-y-auto px-5 py-4 space-y-5">
          {!menuTree ? (
            <div className="flex items-center justify-center h-32 text-sm text-muted-foreground">
              {t("common:loading")}
            </div>
          ) : (
            <>
              {/* Top-level menu items (e.g. 首页) */}
              {topMenus.map((m) => {
                const TopIcon = getIcon(m.icon, m.type)
                return (
                  <div key={m.id} className="flex items-center gap-2 select-none">
                    <Checkbox
                      checked={checkedIds.has(m.id)}
                      onCheckedChange={(checked) => toggleMenu(m, !!checked)}
                    />
                    <TopIcon className="h-3.5 w-3.5 text-muted-foreground" />
                    <span className="text-[13px] font-semibold text-foreground">{m.name}</span>
                  </div>
                )
              })}
                {sections.map((section) => {
              const dir = section.directory
              const isExpanded = dir ? expandedDirs.has(dir.id) : true

              return (
                <div key={dir?.id ?? "top"}>
                  {/* Directory header */}
                  {dir && (
                    <div
                      className="flex items-center gap-2 mb-2 cursor-pointer select-none group"
                      onClick={() => toggleDir(dir.id)}
                    >
                      <span className="text-muted-foreground/50 group-hover:text-muted-foreground transition-colors">
                        {isExpanded
                          ? <ChevronDown className="h-4 w-4" />
                          : <ChevronRight className="h-4 w-4" />}
                      </span>
                      <span onClick={(e) => e.stopPropagation()}>
                        <Checkbox
                          checked={getCheckState(dir)}
                          onCheckedChange={(checked) => toggleMenu(dir, !!checked)}
                        />
                      </span>
                      <span className="text-[13px] font-semibold text-foreground flex items-center gap-1.5">
                        {(() => {
                          const DirIcon = getIcon(dir.icon, dir.type)
                          return <DirIcon className="h-3.5 w-3.5 text-muted-foreground" />
                        })()}
                        {dir.name}
                      </span>
                    </div>
                  )}

                  {/* Permission matrix table */}
                  {isExpanded && section.rows.length > 0 && (
                    <div className="rounded-lg border border-border/60 overflow-hidden ml-6">
                      {section.rows.map((row, idx) => (
                        <div
                          key={row.menu.id}
                          className={
                            "grid items-start gap-3 px-3 py-2.5"
                            + (idx > 0 ? " border-t border-border/40" : "")
                          }
                          style={{ gridTemplateColumns: "120px 1fr" }}
                        >
                          {/* Left: menu name */}
                          <label className="flex items-center gap-2 min-h-6 cursor-pointer">
                            <Checkbox
                              checked={getCheckState(row.menu)}
                              onCheckedChange={(checked) => toggleMenu(row.menu, !!checked)}
                            />
                            {(() => {
                              const MenuIcon = getIcon(row.menu.icon, row.menu.type)
                              return <MenuIcon className="h-3.5 w-3.5 text-muted-foreground/60 shrink-0" />
                            })()}
                            <span className="text-[13px] font-medium text-foreground leading-tight">
                              {row.menu.name}
                            </span>
                          </label>

                          {/* Right: button permissions */}
                          <div className="flex flex-wrap items-center gap-x-4 gap-y-1.5">
                            {row.buttons.map((btn) => (
                              <label
                                key={btn.id}
                                className="flex items-center gap-1.5 min-h-6 cursor-pointer"
                              >
                                <Checkbox
                                  checked={checkedIds.has(btn.id)}
                                  onCheckedChange={(checked) => toggleMenu(btn, !!checked)}
                                  className="h-3.5 w-3.5"
                                />
                                <span className="text-[12px] text-muted-foreground">
                                  {btn.name}
                                </span>
                              </label>
                            ))}
                          </div>
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              )
            })}
            </>
          )}
        </div>

        {saveMutation.error && (
          <p className="px-6 py-2 text-sm text-destructive">{saveMutation.error.message}</p>
        )}
        <SheetFooter className="px-6 py-4">
          <Button variant="outline" size="sm" onClick={() => onOpenChange(false)}>
            {t("common:cancel")}
          </Button>
          <Button
            size="sm"
            onClick={() => saveMutation.mutate()}
            disabled={saveMutation.isPending}
          >
            {saveMutation.isPending ? t("common:saving") : t("common:save")}
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  )
}

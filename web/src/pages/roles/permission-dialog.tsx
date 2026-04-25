import { useEffect, useMemo, useRef, useState } from "react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"
import { useQuery, useMutation } from "@tanstack/react-query"
import { ChevronRight, ChevronDown, Check } from "lucide-react"
import { api } from "@/lib/api"
import { getIcon } from "@/lib/icon-map"
import { cn } from "@/lib/utils"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
  SheetFooter,
} from "@/components/ui/sheet"
import type { MenuItem } from "@/stores/menu"
import type { Role, DataScope } from "./types"

interface PermissionDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  role: Role | null
}

interface PermissionsData {
  menuPermissions: string[]
  apiPolicies: { path: string; method: string }[]
}

interface DeptTreeNode {
  id: number
  name: string
  code: string
  children?: DeptTreeNode[]
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

// ---- Dept multi-select tree helpers ----


function DeptTreeItem({
  node,
  selected,
  onToggle,
  expanded,
  onToggleExpand,
  depth,
}: {
  node: DeptTreeNode
  selected: Set<number>
  onToggle: (id: number) => void
  expanded: Set<number>
  onToggleExpand: (id: number) => void
  depth: number
}) {
  const hasChildren = node.children && node.children.length > 0
  const isExpanded = expanded.has(node.id)
  const isChecked = selected.has(node.id)

  return (
    <div>
      <div
        className="flex items-center gap-2 rounded-md px-2 py-1.5 hover:bg-muted/50"
        style={{ paddingLeft: `${depth * 16 + 8}px` }}
      >
        {hasChildren ? (
          <button
            type="button"
            className="flex h-4 w-4 shrink-0 items-center justify-center text-muted-foreground"
            onClick={() => onToggleExpand(node.id)}
          >
            {isExpanded
              ? <ChevronDown className="h-3 w-3" />
              : <ChevronRight className="h-3 w-3" />}
          </button>
        ) : (
          <span className="w-4 shrink-0" />
        )}
        <Checkbox
          checked={isChecked}
          onCheckedChange={() => onToggle(node.id)}
          className="h-3.5 w-3.5"
        />
        <span className="text-[13px]">{node.name}</span>
      </div>
      {hasChildren && isExpanded && (
        <div>
          {node.children!.map((child) => (
            <DeptTreeItem
              key={child.id}
              node={child}
              selected={selected}
              onToggle={onToggle}
              expanded={expanded}
              onToggleExpand={onToggleExpand}
              depth={depth + 1}
            />
          ))}
        </div>
      )}
    </div>
  )
}

const DATA_SCOPE_OPTIONS: { value: DataScope; labelKey: string; descKey: string }[] = [
  { value: "all", labelKey: "roles:dataScope.all", descKey: "roles:dataScope.allDesc" },
  { value: "dept_and_sub", labelKey: "roles:dataScope.deptAndSub", descKey: "roles:dataScope.deptAndSubDesc" },
  { value: "dept", labelKey: "roles:dataScope.dept", descKey: "roles:dataScope.deptDesc" },
  { value: "self", labelKey: "roles:dataScope.self", descKey: "roles:dataScope.selfDesc" },
  { value: "custom", labelKey: "roles:dataScope.custom", descKey: "roles:dataScope.customDesc" },
]

export function PermissionDialog({ open, onOpenChange, role }: PermissionDialogProps) {
  const { t } = useTranslation(["roles", "common"])
  const [checkedIds, setCheckedIds] = useState<Set<number>>(new Set())
  const [expandedDirs, setExpandedDirs] = useState<Set<number>>(new Set())

  // Data scope state - sync from role prop using render-time update (React-recommended pattern)
  const [prevRoleId, setPrevRoleId] = useState<number | undefined>(role?.id)
  const [dataScope, setDataScope] = useState<DataScope>(role?.dataScope ?? "all")
  const [customDeptIds, setCustomDeptIds] = useState<Set<number>>(new Set(role?.deptIds ?? []))
  const [deptExpanded, setDeptExpanded] = useState<Set<number>>(new Set())

  // Sync dataScope/customDeptIds when role changes (render-time setState, avoids effect+setState)
  if (prevRoleId !== role?.id) {
    setPrevRoleId(role?.id)
    setDataScope(role?.dataScope ?? "all")
    setCustomDeptIds(new Set(role?.deptIds ?? []))
  }

  const deptExpandedInitRef = useRef(false)

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

  // Load dept tree for custom scope (may fail if org module not installed)
  // Initialize deptExpanded inside queryFn to avoid setState in useEffect
  const { data: deptTreeData } = useQuery({
    queryKey: ["departments", "tree"],
    queryFn: async () => {
      const r = await api.get<{ items: DeptTreeNode[] }>("/api/v1/org/departments/tree")
      if (!deptExpandedInitRef.current && r.items.length > 0) {
        deptExpandedInitRef.current = true
        setDeptExpanded(new Set(r.items.map((n) => n.id)))
      }
      return r.items
    },
    enabled: open,
    retry: false,
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
      const id = window.setTimeout(() => {
        setExpandedDirs(new Set(sections.map((s) => s.directory?.id).filter(Boolean) as number[]))
      }, 0)
      return () => window.clearTimeout(id)
    }
  }, [sections])

  useEffect(() => {
    if (!currentPerms || !menuTree) return
    const timer = window.setTimeout(() => {
      const ids = new Set<number>()
      for (const perm of currentPerms.menuPermissions || []) {
        const id = permMap.get(perm)
        if (id !== undefined) ids.add(id)
      }
      setCheckedIds(ids)
    }, 0)
    return () => window.clearTimeout(timer)
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

  function toggleDept(id: number) {
    setCustomDeptIds((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  function toggleDeptExpand(id: number) {
    setDeptExpanded((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  const saveMenuMutation = useMutation({
    mutationFn: () =>
      api.put(`/api/v1/roles/${role!.id}/permissions`, {
        menuIds: Array.from(checkedIds),
        apiPolicies: currentPerms?.apiPolicies || [],
      }),
    onSuccess: () => onOpenChange(false),
    onError: (err) => toast.error(err.message),
  })

  const saveDataScopeMutation = useMutation({
    mutationFn: () =>
      api.put(`/api/v1/roles/${role!.id}/data-scope`, {
        dataScope,
        deptIds: dataScope === "custom" ? Array.from(customDeptIds) : [],
      }),
    onSuccess: () => onOpenChange(false),
    onError: (err) => toast.error(err.message),
  })

  const totalCount = menuTree ? collectAllIds(menuTree).length : 0
  const checkedCount = checkedIds.size

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className="sm:max-w-[480px] flex flex-col gap-0 p-0">
        <SheetHeader className="px-6 pt-6 pb-3 border-b">
          <SheetTitle className="text-base">{t("roles:permission.title", { name: role?.name })}</SheetTitle>
          <SheetDescription className="sr-only">{role?.name}</SheetDescription>
        </SheetHeader>

        <Tabs defaultValue="menu" className="flex flex-1 min-h-0 flex-col">
          <TabsList className="mx-6 mt-3 mb-0 w-auto justify-start rounded-none border-b bg-transparent p-0 h-auto gap-0">
            <TabsTrigger
              value="menu"
              className="rounded-none border-b-2 border-transparent px-4 pb-2 pt-0 text-sm font-medium data-[state=active]:border-primary data-[state=active]:bg-transparent data-[state=active]:shadow-none"
            >
              {t("roles:permission.tabMenu")}
              <span className="ml-1.5 rounded-full bg-muted px-1.5 py-0.5 text-[10px] font-medium text-muted-foreground">
                {checkedCount}/{totalCount}
              </span>
            </TabsTrigger>
            <TabsTrigger
              value="dataScope"
              className="rounded-none border-b-2 border-transparent px-4 pb-2 pt-0 text-sm font-medium data-[state=active]:border-primary data-[state=active]:bg-transparent data-[state=active]:shadow-none"
            >
              {t("roles:permission.tabDataScope")}
            </TabsTrigger>
          </TabsList>

          {/* ── Menu Permissions Tab ── */}
          <TabsContent value="menu" className="flex flex-1 min-h-0 flex-col mt-0">
            <div className="flex-1 overflow-y-auto px-5 py-4 space-y-5">
              {!menuTree ? (
                <div className="flex items-center justify-center h-32 text-sm text-muted-foreground">
                  {t("common:loading")}
                </div>
              ) : (
                <>
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
                    const DirIcon = dir ? getIcon(dir.icon, dir.type) : null

                    return (
                      <div key={dir?.id ?? "top"}>
                        {dir && DirIcon && (
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
                              <DirIcon className="h-3.5 w-3.5 text-muted-foreground" />
                              {dir.name}
                            </span>
                          </div>
                        )}

                        {isExpanded && section.rows.length > 0 && (
                          <div className="rounded-lg border border-border/60 overflow-hidden ml-6">
                            {section.rows.map((row, idx) => {
                              const MenuIcon = getIcon(row.menu.icon, row.menu.type)
                              return (
                                <div
                                  key={row.menu.id}
                                  className={
                                    "grid items-start gap-3 px-3 py-2.5"
                                    + (idx > 0 ? " border-t border-border/40" : "")
                                  }
                                  style={{ gridTemplateColumns: "120px 1fr" }}
                                >
                                  <label className="flex items-center gap-2 min-h-6 cursor-pointer">
                                    <Checkbox
                                      checked={getCheckState(row.menu)}
                                      onCheckedChange={(checked) => toggleMenu(row.menu, !!checked)}
                                    />
                                    <MenuIcon className="h-3.5 w-3.5 text-muted-foreground/60 shrink-0" />
                                    <span className="text-[13px] font-medium text-foreground leading-tight">
                                      {row.menu.name}
                                    </span>
                                  </label>
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
                              )
                            })}
                          </div>
                        )}
                      </div>
                    )
                  })}
                </>
              )}
            </div>
            {saveMenuMutation.error && (
              <p className="px-6 py-2 text-sm text-destructive">{saveMenuMutation.error.message}</p>
            )}
            <SheetFooter className="px-6 py-4 border-t">
              <Button variant="outline" size="sm" onClick={() => onOpenChange(false)}>
                {t("common:cancel")}
              </Button>
              <Button
                size="sm"
                onClick={() => saveMenuMutation.mutate()}
                disabled={saveMenuMutation.isPending}
              >
                {saveMenuMutation.isPending ? t("common:saving") : t("common:save")}
              </Button>
            </SheetFooter>
          </TabsContent>

          {/* ── Data Scope Tab ── */}
          <TabsContent value="dataScope" className="flex flex-1 min-h-0 flex-col mt-0">
            <div className="flex-1 overflow-y-auto px-5 py-4 space-y-4">
              <p className="text-xs text-muted-foreground">{t("roles:dataScope.description")}</p>

              <div className="space-y-2">
                {DATA_SCOPE_OPTIONS.map((opt) => (
                  <button
                    key={opt.value}
                    type="button"
                    onClick={() => setDataScope(opt.value)}
                    className={cn(
                      "flex w-full items-start gap-3 rounded-lg border px-3 py-2.5 text-left transition-colors",
                      dataScope === opt.value
                        ? "border-primary bg-primary/5"
                        : "border-border hover:border-border/80 hover:bg-muted/40"
                    )}
                  >
                    <span className={cn(
                      "mt-0.5 flex h-4 w-4 shrink-0 items-center justify-center rounded-full border",
                      dataScope === opt.value ? "border-primary bg-primary" : "border-muted-foreground/40"
                    )}>
                      {dataScope === opt.value && <Check className="h-2.5 w-2.5 text-primary-foreground" />}
                    </span>
                    <div className="min-w-0">
                      <p className="text-[13px] font-medium text-foreground">{t(opt.labelKey)}</p>
                      <p className="text-[12px] text-muted-foreground">{t(opt.descKey)}</p>
                    </div>
                  </button>
                ))}
              </div>

              {dataScope === "custom" && (
                <div className="space-y-2">
                  <div className="flex items-center justify-between">
                    <p className="text-[13px] font-medium">{t("roles:dataScope.selectDepts")}</p>
                    {customDeptIds.size > 0 && (
                      <span className="text-xs text-muted-foreground">
                        {t("roles:dataScope.deptSelected", { count: customDeptIds.size })}
                      </span>
                    )}
                  </div>
                  {!deptTreeData ? (
                    <div className="rounded-lg border border-dashed p-4 text-center text-sm text-muted-foreground">
                      {t("roles:dataScope.noDeptTree")}
                    </div>
                  ) : (
                    <div className="rounded-lg border max-h-56 overflow-y-auto py-1">
                      {deptTreeData.map((node) => (
                        <DeptTreeItem
                          key={node.id}
                          node={node}
                          selected={customDeptIds}
                          onToggle={toggleDept}
                          expanded={deptExpanded}
                          onToggleExpand={toggleDeptExpand}
                          depth={0}
                        />
                      ))}
                    </div>
                  )}
                </div>
              )}
            </div>

            {saveDataScopeMutation.error && (
              <p className="px-6 py-2 text-sm text-destructive">{saveDataScopeMutation.error.message}</p>
            )}
            <SheetFooter className="px-6 py-4 border-t">
              <Button variant="outline" size="sm" onClick={() => onOpenChange(false)}>
                {t("common:cancel")}
              </Button>
              <Button
                size="sm"
                onClick={() => saveDataScopeMutation.mutate()}
                disabled={saveDataScopeMutation.isPending}
              >
                {saveDataScopeMutation.isPending ? t("common:saving") : t("common:save")}
              </Button>
            </SheetFooter>
          </TabsContent>
        </Tabs>
      </SheetContent>
    </Sheet>
  )
}

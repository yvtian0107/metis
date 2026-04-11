import { useState, useEffect, useMemo, useCallback } from "react"
import { useTranslation } from "react-i18next"
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import {
  DndContext,
  DragOverlay,
  closestCenter,
  PointerSensor,
  useSensor,
  useSensors,
  type DragStartEvent,
  type DragEndEvent,
  type CollisionDetection,
} from "@dnd-kit/core"
import {
  SortableContext,
  useSortable,
  verticalListSortingStrategy,
} from "@dnd-kit/sortable"
import { CSS } from "@dnd-kit/utilities"
import { Plus, ChevronRight, ChevronDown, GripVertical, Menu } from "lucide-react"
import { api } from "@/lib/api"
import { getIcon } from "@/lib/icon-map"
import { usePermission } from "@/hooks/use-permission"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import {
  DataTableActions,
  DataTableActionsCell,
  DataTableActionsHead,
  DataTableCard,
  DataTableEmptyRow,
  DataTableLoadingRow,
} from "@/components/ui/data-table"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "@/components/ui/alert-dialog"
import { addKernelNamespace } from "@/i18n"
import zhCNMenus from "@/i18n/locales/zh-CN/menus.json"
import enMenus from "@/i18n/locales/en/menus.json"
import { MenuSheet } from "./menu-sheet"
import type { MenuItem } from "@/stores/menu"

addKernelNamespace("menus", zhCNMenus, enMenus)

const typeIconClass: Record<string, string> = {
  directory: "text-muted-foreground",
  menu: "text-muted-foreground/70",
  button: "text-muted-foreground/50",
}

/** Collect all IDs in a tree */
function collectAllIds(items: MenuItem[]): number[] {
  const ids: number[] = []
  for (const m of items) {
    ids.push(m.id)
    if (m.children) ids.push(...collectAllIds(m.children))
  }
  return ids
}

/** Build a flat map of id → parentId for all visible (expanded) rows */
function buildParentMap(
  items: MenuItem[],
  parentId: number | null,
  expanded: Set<number>,
): Map<number, number | null> {
  const map = new Map<number, number | null>()
  for (const m of items) {
    map.set(m.id, parentId)
    if (m.children && expanded.has(m.id)) {
      for (const [k, v] of buildParentMap(m.children, m.id, expanded)) {
        map.set(k, v)
      }
    }
  }
  return map
}

/** Get visible row IDs in render order */
function getVisibleIds(items: MenuItem[], expanded: Set<number>): number[] {
  const ids: number[] = []
  for (const m of items) {
    ids.push(m.id)
    if (m.children && expanded.has(m.id)) {
      ids.push(...getVisibleIds(m.children, expanded))
    }
  }
  return ids
}

/** Find siblings array in tree by parentId */
function findSiblings(tree: MenuItem[], parentId: number | null): MenuItem[] | null {
  if (parentId === null) return tree
  for (const node of tree) {
    if (node.id === parentId) return node.children ?? null
    if (node.children) {
      const result = findSiblings(node.children, parentId)
      if (result) return result
    }
  }
  return null
}

// ── Sortable Row ──────────────────────────────────────────────────────

interface SortableRowProps {
  menu: MenuItem
  depth: number
  isExpanded: boolean
  hasChildren: boolean
  onToggleExpand: (id: number) => void
  onEdit: (menu: MenuItem) => void
  onCreate: (parentId: number) => void
  onDelete: (id: number) => void
  canCreate: boolean
  canUpdate: boolean
  canDelete: boolean
}

function SortableMenuRow({
  menu, depth, isExpanded, hasChildren,
  onToggleExpand, onEdit, onCreate, onDelete,
  canCreate, canUpdate, canDelete,
}: SortableRowProps) {
  const { t } = useTranslation(["menus", "common"])
  const {
    attributes,
    listeners,
    setNodeRef,
    transform,
    transition,
    isDragging,
  } = useSortable({ id: menu.id })

  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
    opacity: isDragging ? 0.4 : undefined,
  }

  const Icon = getIcon(menu.icon, menu.type)

  return (
    <TableRow ref={setNodeRef} style={style} {...attributes}>
      <TableCell className="w-[36px] px-1">
        <button
          type="button"
          className="cursor-grab active:cursor-grabbing p-1 rounded hover:bg-muted text-muted-foreground/40 hover:text-muted-foreground transition-colors"
          {...listeners}
        >
          <GripVertical className="h-3.5 w-3.5" />
        </button>
      </TableCell>
      <TableCell>
        <div className="flex items-center gap-1" style={{ paddingLeft: depth * 24 }}>
          {hasChildren ? (
            <button
              type="button"
              className="p-0.5 hover:bg-muted rounded"
              onClick={() => onToggleExpand(menu.id)}
            >
              {isExpanded ? (
                <ChevronDown className="h-4 w-4" />
              ) : (
                <ChevronRight className="h-4 w-4" />
              )}
            </button>
          ) : (
            <span className="w-5" />
          )}
          <Icon className={`h-4 w-4 ${typeIconClass[menu.type] ?? "text-muted-foreground"}`} />
          <span className="font-medium">{menu.name}</span>
        </div>
      </TableCell>
      <TableCell>
        <Badge variant="outline">{t(`menus:menuType.${menu.type}`, menu.type)}</Badge>
      </TableCell>
      <TableCell className="font-mono text-sm text-muted-foreground">
        {menu.path || "-"}
      </TableCell>
      <TableCell className="font-mono text-sm text-muted-foreground">
        {menu.permission || "-"}
      </TableCell>
      <TableCell>
        {menu.isHidden && <Badge variant="secondary">{t("menus:hidden")}</Badge>}
      </TableCell>
      <DataTableActionsCell>
        <DataTableActions>
          {canCreate && menu.type !== "button" && (
            <Button variant="ghost" size="sm" className="px-2.5" onClick={() => onCreate(menu.id)}>
              {t("menus:addChild")}
            </Button>
          )}
          {canUpdate && (
            <Button variant="ghost" size="sm" className="px-2.5" onClick={() => onEdit(menu)}>
              {t("common:edit")}
            </Button>
          )}
          {canDelete && (
            <AlertDialog>
              <AlertDialogTrigger asChild>
                <Button variant="ghost" size="sm" className="px-2.5 text-destructive">
                  {t("common:delete")}
                </Button>
              </AlertDialogTrigger>
              <AlertDialogContent>
                <AlertDialogHeader>
                  <AlertDialogTitle>{t("menus:deleteConfirm.title")}</AlertDialogTitle>
                  <AlertDialogDescription>
                    {t("menus:deleteConfirm.description", { name: menu.name })}
                    {hasChildren && t("menus:deleteConfirm.hasChildren")}
                  </AlertDialogDescription>
                </AlertDialogHeader>
                <AlertDialogFooter>
                  <AlertDialogCancel>{t("common:cancel")}</AlertDialogCancel>
                  <AlertDialogAction onClick={() => onDelete(menu.id)}>
                    {t("common:delete")}
                  </AlertDialogAction>
                </AlertDialogFooter>
              </AlertDialogContent>
            </AlertDialog>
          )}
        </DataTableActions>
      </DataTableActionsCell>
    </TableRow>
  )
}

/** Lightweight overlay shown while dragging */
function DragPreview({ menu }: { menu: MenuItem }) {
  const Icon = getIcon(menu.icon, menu.type)
  return (
    <div className="flex items-center gap-2 rounded-md border bg-background px-3 py-2 shadow-md text-sm">
      <GripVertical className="h-3.5 w-3.5 text-muted-foreground/40" />
      <Icon className={`h-4 w-4 ${typeIconClass[menu.type] ?? "text-muted-foreground"}`} />
      <span className="font-medium">{menu.name}</span>
    </div>
  )
}

// ── Page Component ────────────────────────────────────────────────────

export function Component() {
  const { t } = useTranslation(["menus", "common"])
  const queryClient = useQueryClient()
  const [sheetOpen, setSheetOpen] = useState(false)
  const [editing, setEditing] = useState<MenuItem | null>(null)
  const [parentId, setParentId] = useState<number | null>(null)
  const [expanded, setExpanded] = useState<Set<number>>(new Set())
  const [activeId, setActiveId] = useState<number | null>(null)
  const canCreate = usePermission("system:menu:create")
  const canUpdate = usePermission("system:menu:update")
  const canDelete = usePermission("system:menu:delete")

  const { data: menuTree, isLoading } = useQuery({
    queryKey: ["menus", "tree"],
    queryFn: () => api.get<MenuItem[]>("/api/v1/menus/tree"),
  })

  const deleteMutation = useMutation({
    mutationFn: (id: number) => api.delete(`/api/v1/menus/${id}`),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["menus"] }),
  })

  const sortMutation = useMutation({
    mutationFn: (items: { id: number; sort: number }[]) =>
      api.put("/api/v1/menus/sort", { items }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["menus"] }),
  })

  // Default expand all on first load
  useEffect(() => {
    if (menuTree && expanded.size === 0) {
      setExpanded(new Set(collectAllIds(menuTree)))
    }
  }, [menuTree]) // eslint-disable-line react-hooks/exhaustive-deps

  const parentMap = useMemo(
    () => (menuTree ? buildParentMap(menuTree, null, expanded) : new Map<number, number | null>()),
    [menuTree, expanded],
  )

  const visibleIds = useMemo(
    () => (menuTree ? getVisibleIds(menuTree, expanded) : []),
    [menuTree, expanded],
  )

  // Custom collision detection: only match siblings
  const siblingCollision: CollisionDetection = useCallback(
    (args) => {
      if (!args.active) return []
      const activeParent = parentMap.get(args.active.id as number)
      const filtered = args.droppableContainers.filter((c) =>
        parentMap.get(c.id as number) === activeParent,
      )
      return closestCenter({ ...args, droppableContainers: filtered })
    },
    [parentMap],
  )

  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 5 } }),
  )

  function handleDragStart(event: DragStartEvent) {
    setActiveId(event.active.id as number)
  }

  function handleDragEnd(event: DragEndEvent) {
    setActiveId(null)
    const { active, over } = event
    if (!over || active.id === over.id || !menuTree) return

    const activeParent = parentMap.get(active.id as number)
    const overParent = parentMap.get(over.id as number)
    if (activeParent !== overParent) return

    const siblings = findSiblings(menuTree, activeParent ?? null)
    if (!siblings) return

    const oldIndex = siblings.findIndex((m) => m.id === active.id)
    const newIndex = siblings.findIndex((m) => m.id === over.id)
    if (oldIndex === -1 || newIndex === -1 || oldIndex === newIndex) return

    // Build new sort order
    const reordered = [...siblings]
    const [moved] = reordered.splice(oldIndex, 1)
    reordered.splice(newIndex, 0, moved)

    const items = reordered.map((m, i) => ({ id: m.id, sort: i }))
    sortMutation.mutate(items)
  }

  function handleCreate(pid: number | null = null) {
    setEditing(null)
    setParentId(pid)
    setSheetOpen(true)
  }

  function handleEdit(menu: MenuItem) {
    setEditing(menu)
    setParentId(null)
    setSheetOpen(true)
  }

  function toggleExpand(id: number) {
    setExpanded((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  function renderRows(menus: MenuItem[], depth = 0): React.ReactNode[] {
    const rows: React.ReactNode[] = []
    for (const menu of menus) {
      const hasChildren = menu.children && menu.children.length > 0
      const isExpanded = expanded.has(menu.id)

      rows.push(
        <SortableMenuRow
          key={menu.id}
          menu={menu}
          depth={depth}
          isExpanded={isExpanded}
          hasChildren={hasChildren}
          onToggleExpand={toggleExpand}
          onEdit={handleEdit}
          onCreate={handleCreate}
          onDelete={(id) => deleteMutation.mutate(id)}
          canCreate={canCreate}
          canUpdate={canUpdate}
          canDelete={canDelete}
        />,
      )

      if (hasChildren && isExpanded) {
        rows.push(...renderRows(menu.children, depth + 1))
      }
    }
    return rows
  }

  // Find the active menu item for DragOverlay
  const activeMenu = useMemo(() => {
    if (activeId === null || !menuTree) return null
    function find(items: MenuItem[]): MenuItem | null {
      for (const m of items) {
        if (m.id === activeId) return m
        if (m.children) {
          const r = find(m.children)
          if (r) return r
        }
      }
      return null
    }
    return find(menuTree)
  }, [activeId, menuTree])

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h2 className="text-lg font-semibold">{t("menus:title")}</h2>
        <div className="flex items-center gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={() => {
              if (menuTree) {
                const allIds = new Set(collectAllIds(menuTree))
                setExpanded((prev) => (prev.size === allIds.size ? new Set() : allIds))
              }
            }}
          >
            {expanded.size > 0 ? t("menus:collapseAll") : t("menus:expandAll")}
          </Button>
          <Button size="sm" onClick={() => handleCreate()} disabled={!canCreate}>
            <Plus className="mr-1.5 h-4 w-4" />
            {t("menus:createMenu")}
          </Button>
        </div>
      </div>

      <DndContext
        sensors={sensors}
        collisionDetection={siblingCollision}
        onDragStart={handleDragStart}
        onDragEnd={handleDragEnd}
      >
        <DataTableCard>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className="w-9" />
                <TableHead className="min-w-[220px]">{t("common:name")}</TableHead>
                <TableHead className="w-[100px]">{t("common:type")}</TableHead>
                <TableHead className="min-w-[180px]">{t("menus:path")}</TableHead>
                <TableHead className="min-w-[180px]">{t("menus:permission")}</TableHead>
                <TableHead className="w-[100px]">{t("common:status")}</TableHead>
                <DataTableActionsHead className="min-w-[210px]">{t("common:actions")}</DataTableActionsHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              <SortableContext items={visibleIds} strategy={verticalListSortingStrategy}>
                {isLoading ? (
                  <DataTableLoadingRow colSpan={7} />
                ) : !menuTree || menuTree.length === 0 ? (
                  <DataTableEmptyRow
                    colSpan={7}
                    icon={Menu}
                    title={t("menus:empty.title")}
                    description={t("menus:empty.description")}
                  />
                ) : (
                  renderRows(menuTree)
                )}
              </SortableContext>
            </TableBody>
          </Table>
        </DataTableCard>

        <DragOverlay dropAnimation={null}>
          {activeMenu ? <DragPreview menu={activeMenu} /> : null}
        </DragOverlay>
      </DndContext>

      <MenuSheet
        open={sheetOpen}
        onOpenChange={setSheetOpen}
        menu={editing}
        parentId={parentId}
      />
    </div>
  )
}

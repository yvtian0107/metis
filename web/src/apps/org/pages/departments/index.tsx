import { useState, useMemo } from "react"
import { useTranslation } from "react-i18next"
import { useQuery } from "@tanstack/react-query"
import { useNavigate } from "react-router"
import { Plus, Network, ChevronRight } from "lucide-react"
import { usePermission } from "@/hooks/use-permission"
import { api } from "@/lib/api"
import { cn } from "@/lib/utils"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import {
  DataTableCard,
  DataTableEmptyRow,
  DataTableLoadingRow,
  DataTableToolbar,
} from "@/components/ui/data-table"
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from "@/components/ui/table"
import type { TreeNode } from "../../types"
import { collectAllIds } from "../../types"
import { DepartmentSheet, type DepartmentItem } from "../../components/department-sheet"
import {
  WorkspaceBooleanStatus,
  WorkspaceSearchField,
} from "@/components/workspace/primitives"

interface FlatNode extends TreeNode {
  depth: number
  hasChildren: boolean
}

function flattenTree(nodes: TreeNode[] | undefined, depth: number): FlatNode[] {
  const result: FlatNode[] = []
  if (!nodes) return result
  for (const node of nodes) {
    const hasChildren = !!node.children && node.children.length > 0
    result.push({ ...node, depth, hasChildren })
    if (hasChildren) {
      result.push(...flattenTree(node.children, depth + 1))
    }
  }
  return result
}

export function Component() {
  const { t } = useTranslation(["org", "common"])
  const navigate = useNavigate()
  const [formOpen, setFormOpen] = useState(false)
  const [editing, setEditing] = useState<DepartmentItem | null>(null)
  const [keyword, setKeyword] = useState("")
  const [expanded, setExpanded] = useState<Set<number> | null>(null)

  const canCreate = usePermission("org:department:create")

  const { data, isLoading } = useQuery({
    queryKey: ["departments", "tree"],
    queryFn: async () => {
      const res = await api.get<{ items: TreeNode[] }>("/api/v1/org/departments/tree")
      return res.items ?? []
    },
  })
  const treeData = useMemo(() => data ?? [], [data])
  const allExpandedIds = useMemo(() => new Set(collectAllIds(treeData)), [treeData])
  const visibleExpanded = expanded ?? allExpandedIds
  const isAllExpanded = treeData.length > 0 && visibleExpanded.size === allExpandedIds.size

  const flatItems = useMemo(() => flattenTree(treeData, 0), [treeData])

  const filtered = useMemo(() => {
    if (!keyword) return null
    return flatItems.filter(
      (d) =>
        d.name.toLowerCase().includes(keyword.toLowerCase()) ||
        d.code.toLowerCase().includes(keyword.toLowerCase())
    )
  }, [flatItems, keyword])

  function handleCreate() {
    setEditing(null)
    setFormOpen(true)
  }

  function toggleExpand(id: number) {
    setExpanded((prev) => {
      const next = new Set(prev ?? visibleExpanded)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  function renderRows(nodes: TreeNode[], depth = 0): React.ReactNode[] {
    const rows: React.ReactNode[] = []
    for (const node of nodes) {
      const hasChildren = node.children && node.children.length > 0
      const isExpanded = visibleExpanded.has(node.id)
      rows.push(
        <TableRow
          key={node.id}
          className="group cursor-pointer border-border/45 hover:bg-surface-soft/45"
          onClick={() => navigate(`/org/departments/${node.id}`)}
        >
          <TableCell className="min-w-[260px] py-3.5">
            <div className="flex items-center gap-2" style={{ paddingLeft: `${depth * 22}px` }}>
              {hasChildren ? (
                <button
                  type="button"
                  className={cn(
                    "flex h-6 w-6 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-surface-soft hover:text-foreground",
                    isExpanded && "bg-surface-soft text-foreground"
                  )}
                  onClick={(e) => {
                    e.stopPropagation()
                    toggleExpand(node.id)
                  }}
                >
                  <ChevronRight
                    className={cn(
                      "h-4 w-4 transition-transform duration-200",
                      isExpanded && "rotate-90"
                    )}
                  />
                </button>
              ) : (
                <span className="h-6 w-6" />
              )}
              <div className="min-w-0">
                <div className="flex items-center gap-2">
                  <span className="truncate text-sm font-medium text-foreground">{node.name}</span>
                  <Badge variant="outline" className="h-5 px-1.5 text-[10px] font-normal text-muted-foreground">
                    {node.code}
                  </Badge>
                </div>
                {depth > 0 && (
                  <div className="mt-1 h-px w-8 bg-border/50" />
                )}
              </div>
            </div>
          </TableCell>
          <TableCell className="w-[160px] text-sm text-muted-foreground">{node.managerName || "—"}</TableCell>
          <TableCell className="w-[110px]">
            <span className="text-sm tabular-nums text-muted-foreground">
              {t("org:departments.memberCount", { count: node.memberCount })}
            </span>
          </TableCell>
          <TableCell className="w-[110px]">
            <WorkspaceBooleanStatus
              active={node.isActive}
              activeLabel={t("org:departments.active")}
              inactiveLabel={t("org:departments.inactive")}
            />
          </TableCell>
          <TableCell className="w-[40px]">
            <ChevronRight className="h-4 w-4 text-muted-foreground/50 transition-colors group-hover:text-foreground" />
          </TableCell>
        </TableRow>
      )
      if (hasChildren && isExpanded) {
        rows.push(...renderRows(node.children!, depth + 1))
      }
    }
    return rows
  }

  return (
    <div className="workspace-page">
      <div className="workspace-page-header">
        <div>
          <div className="flex items-center gap-2">
            <h2 className="workspace-page-title">{t("org:departments.title")}</h2>
            <Badge variant="outline" className="bg-transparent font-normal text-muted-foreground">
              {t("org:departments.orgThread")}
            </Badge>
          </div>
        </div>
        <div className="flex items-center gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={() => {
              if (treeData.length > 0) {
                setExpanded(isAllExpanded ? new Set() : new Set(allExpandedIds))
              }
            }}
          >
            {isAllExpanded ? t("common:collapseAll") : t("common:expandAll")}
          </Button>
          {canCreate && (
            <Button size="sm" onClick={handleCreate}>
              <Plus className="mr-1.5 h-4 w-4" />
              {t("org:departments.create")}
            </Button>
          )}
        </div>
      </div>

      <DataTableToolbar>
        <WorkspaceSearchField
          value={keyword}
          onChange={setKeyword}
          placeholder={t("org:departments.searchPlaceholder")}
          className="sm:w-80"
        />
      </DataTableToolbar>

      <DataTableCard>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="min-w-[260px]">{t("org:departments.name")}</TableHead>
              <TableHead className="w-[160px]">{t("org:departments.manager")}</TableHead>
              <TableHead className="w-[110px]">{t("org:departments.members")}</TableHead>
              <TableHead className="w-[110px]">{t("common:status")}</TableHead>
              <TableHead className="w-[40px]" />
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading ? (
              <DataTableLoadingRow colSpan={5} />
            ) : treeData.length === 0 ? (
              <DataTableEmptyRow
                colSpan={5}
                icon={Network}
                title={t("org:departments.empty")}
                description={canCreate ? t("org:departments.emptyHint") : undefined}
              />
            ) : filtered ? (
              filtered.length === 0 ? (
                <DataTableEmptyRow
                  colSpan={5}
                  icon={Network}
                  title={t("org:departments.empty")}
                />
              ) : (
                filtered.map((item) => (
                  <TableRow
                    key={item.id}
                    className="group cursor-pointer border-border/45 hover:bg-surface-soft/45"
                    onClick={() => navigate(`/org/departments/${item.id}`)}
                  >
                    <TableCell className="min-w-[260px] py-3.5">
                      <div className="flex items-center gap-2" style={{ paddingLeft: `${item.depth * 22}px` }}>
                        <span className="h-6 w-6" />
                        <div className="min-w-0">
                          <div className="flex items-center gap-2">
                            <span className="truncate text-sm font-medium text-foreground">{item.name}</span>
                            <Badge variant="outline" className="h-5 px-1.5 text-[10px] font-normal text-muted-foreground">
                              {item.code}
                            </Badge>
                          </div>
                        </div>
                      </div>
                    </TableCell>
                    <TableCell className="w-[160px] text-sm text-muted-foreground">{item.managerName || "—"}</TableCell>
                    <TableCell className="w-[110px]">
                      <span className="text-sm tabular-nums text-muted-foreground">
                        {t("org:departments.memberCount", { count: item.memberCount })}
                      </span>
                    </TableCell>
                    <TableCell className="w-[110px]">
                      <WorkspaceBooleanStatus
                        active={item.isActive}
                        activeLabel={t("org:departments.active")}
                        inactiveLabel={t("org:departments.inactive")}
                      />
                    </TableCell>
                    <TableCell className="w-[40px]">
                      <ChevronRight className="h-4 w-4 text-muted-foreground/50 transition-colors group-hover:text-foreground" />
                    </TableCell>
                  </TableRow>
                ))
              )
            ) : (
              renderRows(treeData)
            )}
          </TableBody>
        </Table>
      </DataTableCard>

      <DepartmentSheet open={formOpen} onOpenChange={setFormOpen} department={editing} />
    </div>
  )
}

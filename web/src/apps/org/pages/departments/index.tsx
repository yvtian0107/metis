import { useState, useMemo, useEffect } from "react"
import { useTranslation } from "react-i18next"
import { useQuery } from "@tanstack/react-query"
import { useNavigate } from "react-router"
import { Plus, Search, Network, ChevronRight } from "lucide-react"
import { usePermission } from "@/hooks/use-permission"
import { api } from "@/lib/api"
import { cn } from "@/lib/utils"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Badge } from "@/components/ui/badge"
import {
  DataTableCard,
  DataTableEmptyRow,
  DataTableLoadingRow,
} from "@/components/ui/data-table"
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from "@/components/ui/table"
import type { TreeNode } from "../../types"
import { collectAllIds } from "../../types"
import { DepartmentSheet, type DepartmentItem } from "../../components/department-sheet"

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
  const [expanded, setExpanded] = useState<Set<number>>(new Set())

  const canCreate = usePermission("org:department:create")

  const { data, isLoading } = useQuery({
    queryKey: ["departments", "tree"],
    queryFn: async () => {
      const res = await api.get<{ items: TreeNode[] }>("/api/v1/org/departments/tree")
      return res.items ?? []
    },
  })
  const treeData = data ?? []

  // Default expand all on first load
  useEffect(() => {
    if (treeData.length > 0 && expanded.size === 0) {
      setExpanded(new Set(collectAllIds(treeData)))
    }
  }, [treeData]) // eslint-disable-line react-hooks/exhaustive-deps

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
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  function renderRows(nodes: TreeNode[], depth = 0): React.ReactNode[] {
    const rows: React.ReactNode[] = []
    for (const node of nodes) {
      const hasChildren = node.children && node.children.length > 0
      const isExpanded = expanded.has(node.id)
      rows.push(
        <TableRow
          key={node.id}
          className="group cursor-pointer"
          onClick={() => navigate(`/org/departments/${node.id}`)}
        >
          <TableCell className="min-w-[200px]">
            <div className="flex items-center gap-1" style={{ paddingLeft: `${depth * 24}px` }}>
              {hasChildren ? (
                <button
                  type="button"
                  className={cn(
                    "w-5 h-5 flex items-center justify-center rounded-sm text-muted-foreground hover:text-foreground transition-colors duration-200",
                    isExpanded && "bg-accent/40"
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
                <span className="w-5" />
              )}
              <span className="font-medium">{node.name}</span>
            </div>
          </TableCell>
          <TableCell className="w-[100px] text-sm text-muted-foreground">{node.code}</TableCell>
          <TableCell className="w-[120px] text-sm text-muted-foreground">{node.managerName || "—"}</TableCell>
          <TableCell className="w-[80px]">
            {node.memberCount > 0 && (
              <span className="rounded-full bg-muted px-2 py-0.5 text-xs font-medium tabular-nums text-muted-foreground">
                {node.memberCount}
              </span>
            )}
          </TableCell>
          <TableCell className="w-[80px]">
            <Badge variant={node.isActive ? "default" : "secondary"}>
              {node.isActive ? t("org:departments.active") : t("org:departments.inactive")}
            </Badge>
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
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h2 className="text-lg font-semibold">{t("org:departments.title")}</h2>
        <div className="flex items-center gap-2">
          <Button
            variant="outline"
            onClick={() => {
              if (treeData.length > 0) {
                const allIds = new Set(collectAllIds(treeData))
                setExpanded((prev) => (prev.size === allIds.size ? new Set() : allIds))
              }
            }}
          >
            {expanded.size > 0 ? t("common:collapseAll") : t("common:expandAll")}
          </Button>
          {canCreate && (
            <Button onClick={handleCreate}>
              <Plus className="mr-1.5 h-4 w-4" />
              {t("org:departments.create")}
            </Button>
          )}
        </div>
      </div>

      <div className="flex w-full flex-col gap-2 sm:flex-row sm:items-center">
        <div className="relative w-full sm:max-w-sm">
          <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
          <Input
            placeholder={t("org:departments.searchPlaceholder")}
            value={keyword}
            onChange={(e) => setKeyword(e.target.value)}
            className="pl-8"
          />
        </div>
      </div>

      <DataTableCard>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="min-w-[200px]">{t("org:departments.name")}</TableHead>
              <TableHead className="w-[100px]">{t("org:departments.code")}</TableHead>
              <TableHead className="w-[120px]">{t("org:departments.manager")}</TableHead>
              <TableHead className="w-[80px]">{t("org:departments.members")}</TableHead>
              <TableHead className="w-[80px]">{t("common:status")}</TableHead>
              <TableHead className="w-[40px]" />
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading ? (
              <DataTableLoadingRow colSpan={6} />
            ) : treeData.length === 0 ? (
              <DataTableEmptyRow
                colSpan={6}
                icon={Network}
                title={t("org:departments.empty")}
                description={canCreate ? t("org:departments.emptyHint") : undefined}
              />
            ) : filtered ? (
              filtered.length === 0 ? (
                <DataTableEmptyRow
                  colSpan={6}
                  icon={Network}
                  title={t("org:departments.empty")}
                />
              ) : (
                filtered.map((item) => (
                  <TableRow
                    key={item.id}
                    className="group cursor-pointer"
                    onClick={() => navigate(`/org/departments/${item.id}`)}
                  >
                    <TableCell className="min-w-[200px]">
                      <div className="flex items-center gap-1" style={{ paddingLeft: `${item.depth * 24}px` }}>
                        <span className="w-5" />
                        <span className="font-medium">{item.name}</span>
                      </div>
                    </TableCell>
                    <TableCell className="w-[100px] text-sm text-muted-foreground">{item.code}</TableCell>
                    <TableCell className="w-[120px] text-sm text-muted-foreground">{item.managerName || "—"}</TableCell>
                    <TableCell className="w-[80px]">
                      {item.memberCount > 0 && (
                        <span className="rounded-full bg-muted px-2 py-0.5 text-xs font-medium tabular-nums text-muted-foreground">
                          {item.memberCount}
                        </span>
                      )}
                    </TableCell>
                    <TableCell className="w-[80px]">
                      <Badge variant={item.isActive ? "default" : "secondary"}>
                        {item.isActive ? t("org:departments.active") : t("org:departments.inactive")}
                      </Badge>
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

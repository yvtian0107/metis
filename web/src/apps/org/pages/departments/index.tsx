"use client"

import { useState, useMemo, useEffect } from "react"
import { useTranslation } from "react-i18next"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { Plus, Search, Pencil, Trash2, Network, ChevronRight } from "lucide-react"
import { usePermission } from "@/hooks/use-permission"
import { api } from "@/lib/api"
import { toast } from "sonner"
import { cn } from "@/lib/utils"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
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
import { DepartmentSheet, type DepartmentItem } from "../../components/department-sheet"

interface TreeNode extends DepartmentItem {
  children?: TreeNode[]
}

interface FlatNode extends DepartmentItem {
  depth: number
  hasChildren: boolean
}

function collectAllIds(nodes: TreeNode[]): number[] {
  const ids: number[] = []
  for (const n of nodes) {
    ids.push(n.id)
    if (n.children) ids.push(...collectAllIds(n.children))
  }
  return ids
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
  const queryClient = useQueryClient()
  const [formOpen, setFormOpen] = useState(false)
  const [editing, setEditing] = useState<DepartmentItem | null>(null)
  const [keyword, setKeyword] = useState("")
  const [expanded, setExpanded] = useState<Set<number>>(new Set())

  const canCreate = usePermission("org:department:create")
  const canUpdate = usePermission("org:department:update")
  const canDelete = usePermission("org:department:delete")

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

  const deleteMutation = useMutation({
    mutationFn: (id: number) => api.delete(`/api/v1/org/departments/${id}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["departments", "tree"] })
      toast.success(t("org:departments.deleteSuccess"))
    },
    onError: (err) => toast.error(err.message),
  })

  function handleCreate() {
    setEditing(null)
    setFormOpen(true)
  }

  function handleEdit(item: DepartmentItem) {
    setEditing(item)
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
        <TableRow key={node.id} className="group">
          <TableCell className="min-w-[180px]">
            <div className="flex items-center gap-1" style={{ paddingLeft: `${depth * 24}px` }}>
              {hasChildren ? (
                <button
                  type="button"
                  className={cn(
                    "w-5 h-5 flex items-center justify-center rounded-sm text-muted-foreground hover:text-foreground transition-colors duration-200",
                    isExpanded && "bg-accent/40"
                  )}
                  onClick={() => toggleExpand(node.id)}
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
          <TableCell className="w-[120px] text-sm text-muted-foreground">{node.code}</TableCell>
          <TableCell className="w-[120px] text-sm text-muted-foreground">{node.managerName || "-"}</TableCell>
          <TableCell className="w-[100px]">
            <Badge variant={node.isActive ? "default" : "secondary"}>
              {node.isActive ? t("org:departments.active") : t("org:departments.inactive")}
            </Badge>
          </TableCell>
          <DataTableActionsCell>
            <DataTableActions>
              {canUpdate && (
                <Button variant="ghost" size="sm" className="px-2.5" onClick={() => handleEdit(node)}>
                  <Pencil className="mr-1 h-3.5 w-3.5" />
                  {t("common:edit")}
                </Button>
              )}
              {canDelete && (
                <AlertDialog>
                  <AlertDialogTrigger asChild>
                    <Button variant="ghost" size="sm" className="px-2.5 text-destructive hover:text-destructive">
                      <Trash2 className="mr-1 h-3.5 w-3.5" />
                      {t("common:delete")}
                    </Button>
                  </AlertDialogTrigger>
                  <AlertDialogContent>
                    <AlertDialogHeader>
                      <AlertDialogTitle>{t("org:departments.deleteTitle")}</AlertDialogTitle>
                      <AlertDialogDescription>
                        {t("org:departments.deleteDesc", { name: node.name })}
                      </AlertDialogDescription>
                    </AlertDialogHeader>
                    <AlertDialogFooter>
                      <AlertDialogCancel size="sm">{t("common:cancel")}</AlertDialogCancel>
                      <AlertDialogAction
                        size="sm"
                        onClick={() => deleteMutation.mutate(node.id)}
                        disabled={deleteMutation.isPending}
                      >
                        {t("org:departments.confirmDelete")}
                      </AlertDialogAction>
                    </AlertDialogFooter>
                  </AlertDialogContent>
                </AlertDialog>
              )}
            </DataTableActions>
          </DataTableActionsCell>
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
              <TableHead className="min-w-[180px]">{t("org:departments.name")}</TableHead>
              <TableHead className="w-[120px]">{t("org:departments.code")}</TableHead>
              <TableHead className="w-[120px]">{t("org:departments.manager")}</TableHead>
              <TableHead className="w-[100px]">{t("common:status")}</TableHead>
              <DataTableActionsHead className="min-w-[140px]">{t("common:actions")}</DataTableActionsHead>
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
                  <TableRow key={item.id}>
                    <TableCell className="min-w-[180px]">
                      <div className="flex items-center gap-1" style={{ paddingLeft: `${item.depth * 24}px` }}>
                        {item.hasChildren ? (
                          <span className="w-5" />
                        ) : (
                          <span className="w-5" />
                        )}
                        <span className="font-medium">{item.name}</span>
                      </div>
                    </TableCell>
                    <TableCell className="w-[120px] text-sm text-muted-foreground">{item.code}</TableCell>
                    <TableCell className="w-[120px] text-sm text-muted-foreground">{item.managerName || "-"}</TableCell>
                    <TableCell className="w-[100px]">
                      <Badge variant={item.isActive ? "default" : "secondary"}>
                        {item.isActive ? t("org:departments.active") : t("org:departments.inactive")}
                      </Badge>
                    </TableCell>
                    <DataTableActionsCell>
                      <DataTableActions>
                        {canUpdate && (
                          <Button variant="ghost" size="sm" className="px-2.5" onClick={() => handleEdit(item)}>
                            <Pencil className="mr-1 h-3.5 w-3.5" />
                            {t("common:edit")}
                          </Button>
                        )}
                        {canDelete && (
                          <AlertDialog>
                            <AlertDialogTrigger asChild>
                              <Button variant="ghost" size="sm" className="px-2.5 text-destructive hover:text-destructive">
                                <Trash2 className="mr-1 h-3.5 w-3.5" />
                                {t("common:delete")}
                              </Button>
                            </AlertDialogTrigger>
                            <AlertDialogContent>
                              <AlertDialogHeader>
                                <AlertDialogTitle>{t("org:departments.deleteTitle")}</AlertDialogTitle>
                                <AlertDialogDescription>
                                  {t("org:departments.deleteDesc", { name: item.name })}
                                </AlertDialogDescription>
                              </AlertDialogHeader>
                              <AlertDialogFooter>
                                <AlertDialogCancel size="sm">{t("common:cancel")}</AlertDialogCancel>
                                <AlertDialogAction
                                  size="sm"
                                  onClick={() => deleteMutation.mutate(item.id)}
                                  disabled={deleteMutation.isPending}
                                >
                                  {t("org:departments.confirmDelete")}
                                </AlertDialogAction>
                              </AlertDialogFooter>
                            </AlertDialogContent>
                          </AlertDialog>
                        )}
                      </DataTableActions>
                    </DataTableActionsCell>
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

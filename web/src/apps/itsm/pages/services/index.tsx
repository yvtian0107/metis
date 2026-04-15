"use client"

import { useMemo, useState } from "react"
import { useTranslation } from "react-i18next"
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { useNavigate } from "react-router"
import { Plus, Search, Pencil, Trash2, Cog, Eye } from "lucide-react"
import { usePermission } from "@/hooks/use-permission"
import { useListPage } from "@/hooks/use-list-page"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Badge } from "@/components/ui/badge"
import {
  Select, SelectContent, SelectItem, SelectTrigger, SelectValue,
} from "@/components/ui/select"
import {
  DataTableActions, DataTableActionsCell, DataTableActionsHead,
  DataTableCard, DataTableEmptyRow, DataTableLoadingRow,
  DataTablePagination, DataTableToolbar, DataTableToolbarGroup,
} from "@/components/ui/data-table"
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from "@/components/ui/table"
import {
  AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent,
  AlertDialogDescription, AlertDialogFooter, AlertDialogHeader,
  AlertDialogTitle, AlertDialogTrigger,
} from "@/components/ui/alert-dialog"
import {
  type ServiceDefItem, type CatalogItem,
  fetchCatalogTree, deleteServiceDef,
} from "../../api"

function flattenCatalogs(nodes: CatalogItem[], depth = 0): Array<CatalogItem & { depth: number }> {
  const result: Array<CatalogItem & { depth: number }> = []
  for (const n of nodes) {
    result.push({ ...n, depth })
    if (n.children?.length) result.push(...flattenCatalogs(n.children, depth + 1))
  }
  return result
}

export function Component() {
  const { t } = useTranslation(["itsm", "common"])
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [catalogFilter, setCatalogFilter] = useState("")

  const canCreate = usePermission("itsm:service:create")
  const canUpdate = usePermission("itsm:service:update")
  const canDelete = usePermission("itsm:service:delete")

  const extraParams = useMemo(() => {
    const params: Record<string, string> = {}
    if (catalogFilter) params.catalogId = catalogFilter
    return params
  }, [catalogFilter])

  const {
    keyword, setKeyword, page, setPage,
    items, total, totalPages, isLoading, handleSearch,
  } = useListPage<ServiceDefItem>({
    queryKey: "itsm-services",
    endpoint: "/api/v1/itsm/services",
    extraParams,
  })

  const { data: catalogs = [] } = useQuery({
    queryKey: ["itsm-catalogs"],
    queryFn: () => fetchCatalogTree(),
  })

  const flatCatalogs = useMemo(() => flattenCatalogs(catalogs), [catalogs])

  const catalogMap = useMemo(() => {
    const map = new Map<number, string>()
    for (const c of flatCatalogs) map.set(c.id, c.name)
    return map
  }, [flatCatalogs])

  const deleteMut = useMutation({
    mutationFn: (id: number) => deleteServiceDef(id),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ["itsm-services"] }); toast.success(t("itsm:services.deleteSuccess")) },
    onError: (err) => toast.error(err.message),
  })

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h2 className="text-lg font-semibold">{t("itsm:services.title")}</h2>
        {canCreate && (
          <Button onClick={() => navigate("/itsm/services/create")}>
            <Plus className="mr-1.5 h-4 w-4" />{t("itsm:services.create")}
          </Button>
        )}
      </div>

      <DataTableToolbar>
        <DataTableToolbarGroup>
          <form onSubmit={handleSearch} className="flex w-full flex-col gap-2 sm:flex-row sm:items-center">
            <div className="relative w-full sm:max-w-sm">
              <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
              <Input placeholder={t("itsm:services.searchPlaceholder")} value={keyword} onChange={(e) => setKeyword(e.target.value)} className="pl-8" />
            </div>
            <Select value={catalogFilter} onValueChange={(v) => { setCatalogFilter(v === "all" ? "" : v); setPage(1) }}>
              <SelectTrigger className="w-[180px]"><SelectValue placeholder={t("itsm:services.allCatalogs")} /></SelectTrigger>
              <SelectContent>
                <SelectItem value="all">{t("itsm:services.allCatalogs")}</SelectItem>
                {flatCatalogs.map((c) => (
                  <SelectItem key={c.id} value={String(c.id)}>{"─".repeat(c.depth)} {c.name}</SelectItem>
                ))}
              </SelectContent>
            </Select>
            <Button type="submit" variant="outline" size="sm">{t("common:search")}</Button>
          </form>
        </DataTableToolbarGroup>
      </DataTableToolbar>

      <DataTableCard>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="min-w-[180px]">{t("itsm:services.name")}</TableHead>
              <TableHead className="w-[350px]">{t("itsm:services.code")}</TableHead>
              <TableHead className="w-[140px]">{t("itsm:services.catalog")}</TableHead>
              <TableHead className="w-[100px]">{t("itsm:services.engineType")}</TableHead>
              <TableHead className="w-[80px]">{t("common:status")}</TableHead>
              <DataTableActionsHead className="min-w-[140px]">{t("common:actions")}</DataTableActionsHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading ? (
              <DataTableLoadingRow colSpan={6} />
            ) : items.length === 0 ? (
              <DataTableEmptyRow colSpan={6} icon={Cog} title={t("itsm:services.empty")} description={canCreate ? t("itsm:services.emptyHint") : undefined} />
            ) : (
              items.map((item) => (
                <TableRow key={item.id}>
                  <TableCell className="font-medium">{item.name}</TableCell>
                  <TableCell className="text-sm text-muted-foreground">{item.code}</TableCell>
                  <TableCell className="text-sm text-muted-foreground">{catalogMap.get(item.catalogId) ?? "—"}</TableCell>
                  <TableCell>
                    <Badge variant={item.engineType === "smart" ? "default" : "outline"}>
                      {item.engineType === "smart" ? t("itsm:services.engineSmart") : t("itsm:services.engineClassic")}
                    </Badge>
                  </TableCell>
                  <TableCell>
                    <Badge variant={item.isActive ? "default" : "secondary"}>
                      {item.isActive ? t("itsm:services.active") : t("itsm:services.inactive")}
                    </Badge>
                  </TableCell>
                  <DataTableActionsCell>
                    <DataTableActions>
                      <Button variant="ghost" size="sm" className="px-2.5" onClick={() => navigate(`/itsm/services/${item.id}`)}>
                        <Eye className="mr-1 h-3.5 w-3.5" />{t("itsm:services.view")}
                      </Button>
                      {canUpdate && (
                        <Button variant="ghost" size="sm" className="px-2.5" onClick={() => navigate(`/itsm/services/${item.id}`)}>
                          <Pencil className="mr-1 h-3.5 w-3.5" />{t("common:edit")}
                        </Button>
                      )}
                      {canDelete && (
                        <AlertDialog>
                          <AlertDialogTrigger asChild>
                            <Button variant="ghost" size="sm" className="px-2.5 text-destructive hover:text-destructive">
                              <Trash2 className="mr-1 h-3.5 w-3.5" />{t("common:delete")}
                            </Button>
                          </AlertDialogTrigger>
                          <AlertDialogContent>
                            <AlertDialogHeader>
                              <AlertDialogTitle>{t("itsm:services.deleteTitle")}</AlertDialogTitle>
                              <AlertDialogDescription>{t("itsm:services.deleteDesc", { name: item.name })}</AlertDialogDescription>
                            </AlertDialogHeader>
                            <AlertDialogFooter>
                              <AlertDialogCancel size="sm">{t("common:cancel")}</AlertDialogCancel>
                              <AlertDialogAction size="sm" onClick={() => deleteMut.mutate(item.id)} disabled={deleteMut.isPending}>{t("itsm:services.confirmDelete")}</AlertDialogAction>
                            </AlertDialogFooter>
                          </AlertDialogContent>
                        </AlertDialog>
                      )}
                    </DataTableActions>
                  </DataTableActionsCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </DataTableCard>

      <DataTablePagination total={total} page={page} totalPages={totalPages} onPageChange={setPage} />
    </div>
  )
}

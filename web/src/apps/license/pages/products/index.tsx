import { useState } from "react"
import { useNavigate } from "react-router"
import { useTranslation } from "react-i18next"
import { Plus, Search, Package, Eye, Pencil } from "lucide-react"
import { usePermission } from "@/hooks/use-permission"
import { useListPage } from "@/hooks/use-list-page"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Badge } from "@/components/ui/badge"
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip"
import {
  DataTableActions,
  DataTableActionsCell,
  DataTableActionsHead,
  DataTableCard,
  DataTableEmptyRow,
  DataTableLoadingRow,
  DataTablePagination,
  DataTableToolbar,
  DataTableToolbarGroup,
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
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { formatDateTime } from "@/lib/utils"
import { ProductSheet, type ProductItem } from "../../components/product-sheet"
import { STATUS_STYLES } from "../../constants"

export function Component() {
  const { t } = useTranslation(["license", "common"])
  const navigate = useNavigate()
  const [formOpen, setFormOpen] = useState(false)
  const [editing, setEditing] = useState<ProductItem | null>(null)
  const [statusFilter, setStatusFilter] = useState("")

  const canCreate = usePermission("license:product:create")
  const canUpdate = usePermission("license:product:update")

  const {
    keyword, setKeyword, page, setPage,
    items: products, total, totalPages, isLoading, handleSearch,
  } = useListPage<ProductItem>({
    queryKey: "license-products",
    endpoint: "/api/v1/license/products",
    extraParams: statusFilter ? { status: statusFilter } : undefined,
  })

  function handleCreate() {
    setEditing(null)
    setFormOpen(true)
  }

  function handleEdit(item: ProductItem) {
    setEditing(item)
    setFormOpen(true)
  }

  function handleStatusFilter(value: string) {
    setStatusFilter(value === "all" ? "" : value)
    setPage(1)
  }

  return (
    <TooltipProvider delayDuration={200}>
      <div className="space-y-4">
        <div className="flex items-center justify-between gap-3">
          <div>
            <h2 className="text-lg font-semibold">{t("license:products.title")}</h2>
            <p className="text-sm text-muted-foreground">{t("license:products.subtitle", "管理商品、套餐与授权密钥")}</p>
          </div>
          {canCreate && (
            <Button size="sm" onClick={handleCreate} className="shrink-0">
              <Plus className="mr-1.5 h-4 w-4" />
              {t("license:products.create")}
            </Button>
          )}
        </div>

        <DataTableToolbar>
          <DataTableToolbarGroup>
            <form onSubmit={handleSearch} className="flex w-full flex-col gap-2 sm:flex-row sm:items-center">
              <div className="relative w-full sm:max-w-sm">
                <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
                <Input
                  placeholder={t("license:products.searchPlaceholder")}
                  value={keyword}
                  onChange={(e) => setKeyword(e.target.value)}
                  className="pl-8"
                />
              </div>
              <Select value={statusFilter || "all"} onValueChange={handleStatusFilter}>
                <SelectTrigger className="w-full sm:w-[140px]">
                  <SelectValue placeholder={t("license:products.allStatus")} />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">{t("license:products.allStatus")}</SelectItem>
                  <SelectItem value="unpublished">{t("license:status.unpublished")}</SelectItem>
                  <SelectItem value="published">{t("license:status.published")}</SelectItem>
                  <SelectItem value="archived">{t("license:status.archived")}</SelectItem>
                </SelectContent>
              </Select>
              <Button type="submit" variant="outline" className="shrink-0">
                {t("common:search")}
              </Button>
            </form>
          </DataTableToolbarGroup>
        </DataTableToolbar>

        <DataTableCard>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className="min-w-[200px]">{t("common:name")}</TableHead>
                <TableHead className="w-[160px]">{t("license:products.code")}</TableHead>
                <TableHead className="w-[110px]">{t("common:status")}</TableHead>
                <TableHead className="w-[100px] text-right">{t("license:products.planCount")}</TableHead>
                <TableHead className="w-[160px]">{t("common:createdAt")}</TableHead>
                <DataTableActionsHead className="w-[96px]">{t("common:actions")}</DataTableActionsHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {isLoading ? (
                <DataTableLoadingRow colSpan={6} />
              ) : products.length === 0 ? (
                <DataTableEmptyRow
                  colSpan={6}
                  icon={Package}
                  title={t("license:products.empty")}
                  description={canCreate ? t("license:products.emptyHint") : undefined}
                />
              ) : (
                products.map((item) => {
                  const statusStyle = STATUS_STYLES[item.status] ?? STATUS_STYLES.unpublished
                  const statusKey = item.status as keyof typeof STATUS_STYLES
                  return (
                    <TableRow
                      key={item.id}
                      className="cursor-pointer"
                      onClick={() => navigate(`/license/products/${item.id}`)}
                    >
                      <TableCell>
                        <div className="flex items-center gap-2.5">
                          <div className="flex h-7 w-7 shrink-0 items-center justify-center rounded-md bg-muted text-muted-foreground">
                            <Package className="h-3.5 w-3.5" />
                          </div>
                          <div className="min-w-0 space-y-0.5">
                            <div className="font-medium">{item.name}</div>
                            {item.description && (
                              <p className="line-clamp-1 text-xs text-muted-foreground">{item.description}</p>
                            )}
                          </div>
                        </div>
                      </TableCell>
                      <TableCell>
                        <code className="rounded bg-muted px-1.5 py-0.5 text-xs font-mono text-muted-foreground">
                          {item.code}
                        </code>
                      </TableCell>
                      <TableCell>
                        <Badge
                          variant={statusStyle.variant}
                          className={statusStyle.className}
                        >
                          {t(`license:status.${statusKey}`, item.status)}
                        </Badge>
                      </TableCell>
                      <TableCell className="text-right text-sm tabular-nums">{item.planCount}</TableCell>
                      <TableCell className="whitespace-nowrap text-sm text-muted-foreground">
                        {formatDateTime(item.createdAt)}
                      </TableCell>
                      <DataTableActionsCell>
                        <DataTableActions>
                          <Tooltip>
                            <TooltipTrigger asChild>
                              <Button
                                variant="ghost"
                                size="icon"
                                className="h-8 w-8"
                                onClick={(e) => { e.stopPropagation(); navigate(`/license/products/${item.id}`) }}
                              >
                                <Eye className="h-4 w-4" />
                              </Button>
                            </TooltipTrigger>
                            <TooltipContent side="top">{t("license:products.detail")}</TooltipContent>
                          </Tooltip>
                          {canUpdate && (
                            <Tooltip>
                              <TooltipTrigger asChild>
                                <Button
                                  variant="ghost"
                                  size="icon"
                                  className="h-8 w-8"
                                  onClick={(e) => { e.stopPropagation(); handleEdit(item) }}
                                >
                                  <Pencil className="h-4 w-4" />
                                </Button>
                              </TooltipTrigger>
                              <TooltipContent side="top">{t("common:edit")}</TooltipContent>
                            </Tooltip>
                          )}
                        </DataTableActions>
                      </DataTableActionsCell>
                    </TableRow>
                  )
                })
              )}
            </TableBody>
          </Table>
        </DataTableCard>

        <DataTablePagination
          total={total}
          page={page}
          totalPages={totalPages}
          onPageChange={setPage}
        />

        <ProductSheet open={formOpen} onOpenChange={setFormOpen} product={editing} />
      </div>
    </TooltipProvider>
  )
}

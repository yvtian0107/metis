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

const STATUS_VARIANTS: Record<string, "default" | "secondary" | "outline"> = {
  unpublished: "secondary",
  published: "default",
  archived: "outline",
}

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
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h2 className="text-lg font-semibold">{t("license:products.title")}</h2>
        {canCreate && (
          <Button size="sm" onClick={handleCreate}>
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
              <SelectTrigger className="w-full sm:w-[130px]">
                <SelectValue placeholder={t("license:products.allStatus")} />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">{t("license:products.allStatus")}</SelectItem>
                <SelectItem value="unpublished">{t("license:status.unpublished")}</SelectItem>
                <SelectItem value="published">{t("license:status.published")}</SelectItem>
                <SelectItem value="archived">{t("license:status.archived")}</SelectItem>
              </SelectContent>
            </Select>
            <Button type="submit" variant="outline">
              {t("common:search")}
            </Button>
          </form>
        </DataTableToolbarGroup>
      </DataTableToolbar>

      <DataTableCard>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="min-w-[180px]">{t("common:name")}</TableHead>
              <TableHead className="w-[150px]">{t("license:products.code")}</TableHead>
              <TableHead className="w-[100px]">{t("common:status")}</TableHead>
              <TableHead className="w-[80px]">{t("license:products.planCount")}</TableHead>
              <TableHead className="w-[150px]">{t("common:createdAt")}</TableHead>
              <DataTableActionsHead className="min-w-[140px]">{t("common:actions")}</DataTableActionsHead>
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
                const variant = STATUS_VARIANTS[item.status] ?? ("secondary" as const)
                const statusKey = item.status as keyof typeof STATUS_VARIANTS
                return (
                  <TableRow key={item.id} className="cursor-pointer" onClick={() => navigate(`/license/products/${item.id}`)}>
                    <TableCell className="font-medium">{item.name}</TableCell>
                    <TableCell className="font-mono text-sm text-muted-foreground">{item.code}</TableCell>
                    <TableCell>
                      <Badge variant={variant}>{t(`license:status.${statusKey}`, item.status)}</Badge>
                    </TableCell>
                    <TableCell className="text-sm">{item.planCount}</TableCell>
                    <TableCell className="text-sm text-muted-foreground whitespace-nowrap">
                      {formatDateTime(item.createdAt)}
                    </TableCell>
                    <DataTableActionsCell>
                      <DataTableActions>
                        <Button
                          variant="ghost"
                          size="sm"
                          className="px-2.5"
                          onClick={(e) => { e.stopPropagation(); navigate(`/license/products/${item.id}`) }}
                        >
                          <Eye className="mr-1 h-3.5 w-3.5" />
                          {t("license:products.detail")}
                        </Button>
                        {canUpdate && (
                          <Button
                            variant="ghost"
                            size="sm"
                            className="px-2.5"
                            onClick={(e) => { e.stopPropagation(); handleEdit(item) }}
                          >
                            <Pencil className="mr-1 h-3.5 w-3.5" />
                            {t("common:edit")}
                          </Button>
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
  )
}

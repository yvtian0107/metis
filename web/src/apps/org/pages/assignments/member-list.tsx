import { useTranslation } from "react-i18next"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Badge } from "@/components/ui/badge"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog"
import {
  DataTableActionsCell,
  DataTableActionsHead,
  DataTableEmptyRow,
  DataTableLoadingRow,
  DataTablePagination,
} from "@/components/ui/data-table"
import {
  Search,
  Plus,
  Star,
  Users,
  MoreHorizontal,
  ArrowRightLeft,
  Building2,
  Trash2,
  FolderOpen,
} from "lucide-react"
import type { MemberItem, TreeNode } from "./types"

interface MemberListProps {
  selectedDept: TreeNode | null
  items: MemberItem[]
  total: number
  page: number
  totalPages: number
  isLoading: boolean
  keyword: string
  setKeyword: (v: string) => void
  handleSearch: (e: React.FormEvent) => void
  setPage: (p: number) => void
  positionMap: Map<number, string>
  canCreate: boolean
  canUpdate: boolean
  canDelete: boolean
  onAddMember: () => void
  onSetPrimary: (item: MemberItem) => void
  onChangePosition: (item: MemberItem) => void
  onViewOrgInfo: (item: MemberItem) => void
  onRemoveMember: (item: MemberItem) => void
  removeTarget: MemberItem | null
  onRemoveTargetChange: (item: MemberItem | null) => void
  onConfirmRemove: (item: MemberItem) => void
}

export function MemberList({
  selectedDept,
  items,
  total,
  page,
  totalPages,
  isLoading,
  keyword,
  setKeyword,
  handleSearch,
  setPage,
  positionMap,
  canCreate,
  canUpdate,
  canDelete,
  onAddMember,
  onSetPrimary,
  onChangePosition,
  onViewOrgInfo,
  onRemoveMember,
  removeTarget,
  onRemoveTargetChange,
  onConfirmRemove,
}: MemberListProps) {
  const { t } = useTranslation(["org", "common"])

  return (
    <>
      <section className="flex min-h-0 flex-col overflow-hidden rounded-xl border bg-card">
        {!selectedDept ? (
          <>
            <div className="border-b px-6 py-4">
              <div className="flex flex-col gap-4 xl:flex-row xl:items-center xl:justify-between">
                <div>
                  <h3 className="truncate text-base font-semibold text-foreground">
                    {t("org:assignments.title")}
                  </h3>
                </div>
                {canCreate && (
                  <Button disabled>
                    <Plus className="mr-1.5 h-4 w-4" />
                    {t("org:assignments.addMember")}
                  </Button>
                )}
              </div>
            </div>
            <div className="flex flex-1 items-center justify-center px-6 py-10">
              <div className="max-w-sm text-center">
                <div className="mx-auto flex h-14 w-14 items-center justify-center rounded-xl bg-muted text-muted-foreground">
                  <FolderOpen className="h-6 w-6" />
                </div>
                <p className="mt-5 text-base font-semibold text-foreground">
                  {t("org:assignments.selectDept")}
                </p>
                <p className="mt-2 text-sm leading-6 text-muted-foreground">
                  {t("org:assignments.selectDeptHint")}
                </p>
              </div>
            </div>
          </>
        ) : (
          <>
            <div className="border-b px-6 py-4">
              <div className="flex flex-col gap-4 xl:flex-row xl:items-center xl:justify-between">
                <div className="min-w-0">
                  <div className="flex items-center gap-2.5">
                    <h3 className="truncate text-base font-semibold text-foreground">
                      {selectedDept.name}
                    </h3>
                    {selectedDept.code ? (
                      <Badge variant="outline" className="text-[10px] font-medium uppercase tracking-[0.14em] text-muted-foreground">
                        {selectedDept.code}
                      </Badge>
                    ) : null}
                  </div>
                  <p className="mt-1 text-sm text-muted-foreground">
                    {total} {t("org:assignments.memberCount")}
                  </p>
                </div>

                <div className="flex w-full flex-col gap-2 lg:w-auto lg:flex-row lg:items-center lg:justify-end">
                  <form onSubmit={handleSearch} className="flex w-full gap-2 lg:w-auto">
                    <div className="relative w-full lg:w-[280px]">
                      <Search className="absolute left-2.5 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                      <Input
                        placeholder={t("org:assignments.searchPlaceholder")}
                        value={keyword}
                        onChange={(e) => setKeyword(e.target.value)}
                        className="h-9 pl-8"
                      />
                    </div>
                    <Button type="submit" variant="outline" className="h-9 shrink-0">
                      {t("common:search")}
                    </Button>
                  </form>
                  {canCreate && (
                    <Button onClick={onAddMember} className="h-9 shrink-0">
                      <Plus className="mr-1.5 h-4 w-4" />
                      {t("org:assignments.addMember")}
                    </Button>
                  )}
                </div>
              </div>
            </div>

            <div className="min-h-0 flex-1 overflow-auto">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead className="sticky top-0 z-10 min-w-[220px] bg-card">
                      {t("org:assignments.user")}
                    </TableHead>
                    <TableHead className="sticky top-0 z-10 min-w-[140px] bg-card">
                      {t("org:assignments.position")}
                    </TableHead>
                    <TableHead className="sticky top-0 z-10 w-[96px] bg-card">
                      {t("org:assignments.type")}
                    </TableHead>
                    <TableHead className="sticky top-0 z-10 w-[140px] bg-card">
                      {t("org:assignments.assignedAt")}
                    </TableHead>
                    <DataTableActionsHead className="sticky top-0 z-10 bg-card" />
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {isLoading ? (
                    <DataTableLoadingRow colSpan={5} />
                  ) : items.length === 0 ? (
                    <DataTableEmptyRow
                      colSpan={5}
                      icon={Users}
                      title={t("org:assignments.empty")}
                      description={canCreate ? t("org:assignments.emptyHint") : undefined}
                    />
                  ) : (
                    items.map((item) => (
                      <TableRow key={item.assignmentId}>
                        <TableCell className="py-3.5">
                          <div className="flex items-center gap-3">
                            {item.avatar ? (
                              <img
                                src={item.avatar}
                                alt={item.username}
                                className="h-9 w-9 shrink-0 rounded-full object-cover"
                              />
                            ) : (
                              <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-muted text-xs font-semibold text-foreground/80">
                                {item.username.charAt(0).toUpperCase()}
                              </div>
                            )}
                            <div className="min-w-0">
                              <p className="truncate text-sm font-medium text-foreground">
                                {item.username}
                              </p>
                              {item.email && (
                                <p className="truncate text-xs text-muted-foreground">
                                  {item.email}
                                </p>
                              )}
                            </div>
                          </div>
                        </TableCell>
                        <TableCell className="text-sm text-foreground/90">
                          {positionMap.get(item.positionId) ?? "-"}
                        </TableCell>
                        <TableCell>
                          {item.isPrimary ? (
                            <Badge variant="default" className="px-2 text-[10px] font-medium">
                              {t("org:assignments.primary")}
                            </Badge>
                          ) : (
                            <Badge variant="outline" className="px-2 text-[10px] font-medium">
                              {t("org:assignments.secondary")}
                            </Badge>
                          )}
                        </TableCell>
                        <TableCell className="text-xs text-muted-foreground tabular-nums">
                          {item.createdAt ? new Date(item.createdAt).toLocaleDateString() : "-"}
                        </TableCell>
                        <DataTableActionsCell>
                          <DropdownMenu>
                            <DropdownMenuTrigger asChild>
                              <Button variant="ghost" size="icon-sm" className="rounded-lg">
                                <MoreHorizontal className="h-4 w-4" />
                              </Button>
                            </DropdownMenuTrigger>
                            <DropdownMenuContent align="end">
                              {canUpdate && !item.isPrimary && (
                                <DropdownMenuItem onClick={() => onSetPrimary(item)}>
                                  <Star className="mr-2 h-4 w-4" />
                                  {t("org:assignments.setPrimary")}
                                </DropdownMenuItem>
                              )}
                              {canUpdate && (
                                <DropdownMenuItem onClick={() => onChangePosition(item)}>
                                  <ArrowRightLeft className="mr-2 h-4 w-4" />
                                  {t("org:assignments.changePosition")}
                                </DropdownMenuItem>
                              )}
                              <DropdownMenuItem onClick={() => onViewOrgInfo(item)}>
                                <Building2 className="mr-2 h-4 w-4" />
                                {t("org:assignments.viewOrgInfo")}
                              </DropdownMenuItem>
                              {canDelete && (
                                <>
                                  <DropdownMenuSeparator />
                                  <DropdownMenuItem
                                    className="text-destructive focus:text-destructive"
                                    onClick={() => onRemoveMember(item)}
                                  >
                                    <Trash2 className="mr-2 h-4 w-4" />
                                    {t("org:assignments.removeMember")}
                                  </DropdownMenuItem>
                                </>
                              )}
                            </DropdownMenuContent>
                          </DropdownMenu>
                        </DataTableActionsCell>
                      </TableRow>
                    ))
                  )}
                </TableBody>
              </Table>
            </div>

            <div className="border-t border-border/60 px-6 py-4">
              <DataTablePagination
                total={total}
                page={page}
                totalPages={totalPages}
                onPageChange={setPage}
                className="pt-0"
              />
            </div>
          </>
        )}
      </section>

      <AlertDialog open={!!removeTarget} onOpenChange={(open) => { if (!open) onRemoveTargetChange(null) }}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t("org:assignments.removeTitle")}</AlertDialogTitle>
            <AlertDialogDescription>
              {t("org:assignments.removeDesc", { name: removeTarget?.username })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t("common:cancel")}</AlertDialogCancel>
            <AlertDialogAction
              onClick={() => removeTarget && onConfirmRemove(removeTarget)}
              className="bg-destructive text-white hover:bg-destructive/90"
            >
              {t("org:assignments.confirmRemove")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}

import { useState, useMemo, useCallback } from "react"
import { useTranslation } from "react-i18next"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { api } from "@/lib/api"
import { toast } from "sonner"
import { usePermission } from "@/hooks/use-permission"
import { useListPage } from "@/hooks/use-list-page"
import type { TreeNode, MemberItem, PositionItem } from "./types"
import { collectExpandedIds, findNodeById } from "./types"
import { DepartmentTree } from "./department-tree"
import { MemberList } from "./member-list"
import { AddMemberSheet } from "./add-member-sheet"
import { ChangePositionSheet } from "../../components/change-position-sheet"
import { UserOrgSheet } from "../../components/user-org-sheet"

export function Component() {
  const { t } = useTranslation(["org", "common"])
  const queryClient = useQueryClient()

  // -- Shared state --
  const [selectedDeptId, setSelectedDeptId] = useState<number | null>(null)
  const [userExpanded, setUserExpanded] = useState<Set<number> | null>(null)
  const [sheetOpen, setSheetOpen] = useState(false)
  const [removeTarget, setRemoveTarget] = useState<MemberItem | null>(null)
  const [changePositionTarget, setChangePositionTarget] = useState<MemberItem | null>(null)
  const [orgSheetTarget, setOrgSheetTarget] = useState<MemberItem | null>(null)

  // -- Permissions --
  const canCreate = usePermission("org:assignment:create")
  const canUpdate = usePermission("org:assignment:update")
  const canDelete = usePermission("org:assignment:delete")

  // -- Department tree data --
  const { data: treeData, isLoading: treeLoading } = useQuery({
    queryKey: ["departments", "tree"],
    queryFn: async () => {
      const res = await api.get<{ items: TreeNode[] }>("/api/v1/org/departments/tree")
      return res.items
    },
  })

  // Auto-expand first 2 levels from data, user interactions override via userExpanded
  const autoExpanded = useMemo(() => {
    if (!treeData || treeData.length === 0) return new Set<number>()
    return new Set(collectExpandedIds(treeData, 2))
  }, [treeData])

  const expanded = userExpanded ?? autoExpanded

  // -- Member list --
  const extraParams = useMemo(() => {
    return selectedDeptId ? { departmentId: String(selectedDeptId) } : undefined
  }, [selectedDeptId])

  const {
    keyword,
    setKeyword,
    page,
    setPage,
    items,
    total,
    totalPages,
    isLoading,
    handleSearch,
  } = useListPage<MemberItem>({
    queryKey: "org-assignments",
    endpoint: "/api/v1/org/users",
    extraParams,
    enabled: !!selectedDeptId,
  })

  // -- Positions for display --
  const { data: positionsData } = useQuery({
    queryKey: ["positions", "all"],
    queryFn: async () => {
      const res = await api.get<{ items: PositionItem[] }>("/api/v1/org/positions?pageSize=0")
      return res.items
    },
  })

  const positionMap = useMemo(() => {
    const map = new Map<number, string>()
    positionsData?.forEach((p) => map.set(p.id, p.name))
    return map
  }, [positionsData])

  const existingUserIds = useMemo(() => {
    return new Set(items.map((m) => m.userId))
  }, [items])

  const selectedDept = useMemo(() => {
    if (!treeData || !selectedDeptId) return null
    return findNodeById(treeData, selectedDeptId)
  }, [treeData, selectedDeptId])

  // -- Mutations --
  const invalidateAll = useCallback(() => {
    queryClient.invalidateQueries({ queryKey: ["org-assignments"] })
    queryClient.invalidateQueries({ queryKey: ["departments", "tree"] })
  }, [queryClient])

  const removeMutation = useMutation({
    mutationFn: async (member: MemberItem) => {
      await api.delete(`/api/v1/org/users/${member.userId}/positions/${member.assignmentId}`)
    },
    onSuccess: () => {
      toast.success(t("org:assignments.removeSuccess"))
      invalidateAll()
      setRemoveTarget(null)
    },
    onError: (err: Error) => toast.error(err.message),
  })

  const setPrimaryMutation = useMutation({
    mutationFn: async (member: MemberItem) => {
      await api.put(`/api/v1/org/users/${member.userId}/positions/${member.assignmentId}/primary`, {})
    },
    onSuccess: () => {
      toast.success(t("org:assignments.primarySuccess"))
      queryClient.invalidateQueries({ queryKey: ["org-assignments"] })
    },
    onError: (err: Error) => toast.error(err.message),
  })

  return (
    <div className="flex min-h-[620px] flex-col gap-4 lg:h-[calc(100vh-104px)]">
      <div className="flex items-center justify-between">
        <h2 className="text-lg font-semibold text-foreground">
          {t("org:assignments.title")}
        </h2>
      </div>

      <div className="grid min-h-0 flex-1 grid-cols-1 gap-4 lg:grid-cols-[304px_minmax(0,1fr)]">
        <DepartmentTree
          treeData={treeData}
          treeLoading={treeLoading}
          selectedDeptId={selectedDeptId}
          onSelectDept={(id) => {
            setSelectedDeptId(id)
            setPage(1)
          }}
          expanded={expanded}
          onSetExpanded={setUserExpanded}
        />

        <MemberList
          selectedDept={selectedDept}
          items={items}
          total={total}
          page={page}
          totalPages={totalPages}
          isLoading={isLoading}
          keyword={keyword}
          setKeyword={setKeyword}
          handleSearch={handleSearch}
          setPage={setPage}
          positionMap={positionMap}
          canCreate={canCreate}
          canUpdate={canUpdate}
          canDelete={canDelete}
          onAddMember={() => setSheetOpen(true)}
          onSetPrimary={(item) => setPrimaryMutation.mutate(item)}
          onChangePosition={setChangePositionTarget}
          onViewOrgInfo={setOrgSheetTarget}
          onRemoveMember={setRemoveTarget}
          removeTarget={removeTarget}
          onRemoveTargetChange={setRemoveTarget}
          onConfirmRemove={(item) => removeMutation.mutate(item)}
        />
      </div>

      <AddMemberSheet
        open={sheetOpen}
        onOpenChange={setSheetOpen}
        selectedDept={selectedDept}
        deptId={selectedDeptId}
        existingUserIds={existingUserIds}
        onSuccess={() => {}}
      />

      {changePositionTarget && (
        <ChangePositionSheet
          open={!!changePositionTarget}
          onOpenChange={(open) => { if (!open) setChangePositionTarget(null) }}
          userId={changePositionTarget.userId}
          assignmentId={changePositionTarget.assignmentId}
          currentPositionId={changePositionTarget.positionId}
          onSuccess={invalidateAll}
        />
      )}

      <UserOrgSheet
        open={!!orgSheetTarget}
        onOpenChange={(open) => { if (!open) setOrgSheetTarget(null) }}
        userId={orgSheetTarget?.userId ?? null}
        username={orgSheetTarget?.username ?? ""}
        email={orgSheetTarget?.email ?? ""}
      />
    </div>
  )
}

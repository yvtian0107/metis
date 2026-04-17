import { useState } from "react"
import { useTranslation } from "react-i18next"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { api } from "@/lib/api"
import { toast } from "sonner"
import { useDebouncedValue } from "@/hooks/use-debounce"
import { cn } from "@/lib/utils"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetDescription,
  SheetFooter,
} from "@/components/ui/sheet"
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from "@/components/ui/command"
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover"
import { ChevronsUpDown, Check, Search, X, Star } from "lucide-react"
import type { TreeNode, PositionItem, UserItem } from "../types"

interface AddMemberSheetProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  selectedDept: TreeNode | null
  deptId: number | null
  existingUserIds: Set<number>
  onSuccess: () => void
}

export function AddMemberSheet({
  open,
  onOpenChange,
  selectedDept,
  deptId,
  existingUserIds,
  onSuccess,
}: AddMemberSheetProps) {
  const { t } = useTranslation(["org", "common"])
  const queryClient = useQueryClient()

  const [selectedUserObj, setSelectedUserObj] = useState<UserItem | null>(null)
  const [userComboOpen, setUserComboOpen] = useState(false)
  const [userKeyword, setUserKeyword] = useState("")
  const debouncedUserKeyword = useDebouncedValue(userKeyword, 300)

  const [selectedPositionIds, setSelectedPositionIds] = useState<number[]>([])
  const [primaryPositionId, setPrimaryPositionId] = useState<number | null>(null)
  const [posPopoverOpen, setPosPopoverOpen] = useState(false)

  function resetSheet() {
    setSelectedUserObj(null)
    setUserKeyword("")
    setUserComboOpen(false)
    setSelectedPositionIds([])
    setPrimaryPositionId(null)
    setPosPopoverOpen(false)
  }

  function handleOpenChange(nextOpen: boolean) {
    if (nextOpen) resetSheet()
    onOpenChange(nextOpen)
  }

  function togglePosition(posId: number) {
    setSelectedPositionIds((prev) => {
      if (prev.includes(posId)) {
        const next = prev.filter((id) => id !== posId)
        if (primaryPositionId === posId) {
          setPrimaryPositionId(next.length > 0 ? next[0] : null)
        }
        return next
      }
      const next = [...prev, posId]
      if (next.length === 1) {
        setPrimaryPositionId(posId)
      }
      return next
    })
  }

  const { data: userSearchData } = useQuery({
    queryKey: ["users", "search", debouncedUserKeyword],
    queryFn: async () => {
      const params = new URLSearchParams({ page: "1", pageSize: "50" })
      if (debouncedUserKeyword) params.set("keyword", debouncedUserKeyword)
      const res = await api.get<{ items: UserItem[] }>(`/api/v1/users?${params}`)
      return res.items
    },
    enabled: open,
  })

  const { data: positionsData } = useQuery({
    queryKey: ["departments", deptId, "positions"],
    queryFn: async () => {
      const res = await api.get<{ items: PositionItem[] }>(`/api/v1/org/departments/${deptId}/positions`)
      return res.items
    },
    enabled: open && !!deptId,
  })

  const addMutation = useMutation({
    mutationFn: async () => {
      await api.put(`/api/v1/org/users/${selectedUserObj!.id}/departments/${deptId}/positions`, {
        positionIds: selectedPositionIds,
        primaryPositionId: primaryPositionId,
      })
    },
    onSuccess: () => {
      toast.success(t("org:assignments.addSuccess"))
      queryClient.invalidateQueries({ queryKey: ["org-assignments"] })
      queryClient.invalidateQueries({ queryKey: ["departments", "tree"] })
      onSuccess()
      onOpenChange(false)
    },
    onError: (err: Error) => toast.error(err.message),
  })

  function onSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (!selectedUserObj || selectedPositionIds.length === 0) return
    addMutation.mutate()
  }

  const canSubmit = !!selectedUserObj && selectedPositionIds.length > 0 && !addMutation.isPending
  const hasPositionsConfigured = positionsData && positionsData.length > 0

  return (
    <Sheet open={open} onOpenChange={handleOpenChange}>
      <SheetContent className="gap-0 p-0 sm:max-w-md">
        <SheetHeader className="border-b px-6 py-5">
          <SheetTitle>{t("org:assignments.addMemberTo", { dept: selectedDept?.name ?? "" })}</SheetTitle>
          <SheetDescription className="sr-only">
            {t("org:assignments.addMember")}
          </SheetDescription>
        </SheetHeader>
        <form onSubmit={onSubmit} className="flex min-h-0 flex-1 flex-col overflow-hidden">
          <div className="flex-1 space-y-5 overflow-auto px-6 py-6">
            {/* User picker with Command */}
            <div className="space-y-2">
              <label className="text-sm font-medium">{t("org:assignments.selectUser")}</label>
              <Popover open={userComboOpen} onOpenChange={setUserComboOpen}>
                <PopoverTrigger asChild>
                  <Button
                    variant="outline"
                    role="combobox"
                    aria-expanded={userComboOpen}
                    className="w-full justify-between font-normal"
                    type="button"
                  >
                    {selectedUserObj ? (
                      <span className="flex items-center gap-2">
                        {selectedUserObj.avatar ? (
                          <img src={selectedUserObj.avatar} alt={selectedUserObj.username} className="h-5 w-5 rounded-full" />
                        ) : (
                          <div className="flex h-5 w-5 items-center justify-center rounded-full bg-muted text-[10px]">
                            {selectedUserObj.username.charAt(0).toUpperCase()}
                          </div>
                        )}
                        <span>{selectedUserObj.username}</span>
                        {selectedUserObj.email && <span className="text-xs text-muted-foreground">{selectedUserObj.email}</span>}
                      </span>
                    ) : (
                      <span className="text-muted-foreground">{t("org:assignments.selectUser")}</span>
                    )}
                    <ChevronsUpDown className="ml-2 h-4 w-4 shrink-0 opacity-50" />
                  </Button>
                </PopoverTrigger>
                <PopoverContent className="w-[var(--radix-popover-trigger-width)] p-0" align="start">
                  <Command shouldFilter={false}>
                    <div className="flex items-center gap-2 border-b px-3" data-slot="command-input-wrapper">
                      <Search className="size-4 shrink-0 opacity-50" />
                      <input
                        placeholder={t("org:assignments.searchUserPlaceholder")}
                        value={userKeyword}
                        onChange={(e) => setUserKeyword(e.target.value)}
                        className="flex h-10 w-full rounded-md bg-transparent py-3 text-sm outline-hidden placeholder:text-muted-foreground"
                      />
                    </div>
                    <CommandList>
                      <CommandEmpty>{t("common:noData")}</CommandEmpty>
                      <CommandGroup>
                        {userSearchData?.map((user) => {
                          const alreadyAssigned = existingUserIds.has(user.id)
                          const isSelected = selectedUserObj?.id === user.id
                          return (
                            <CommandItem
                              key={user.id}
                              value={String(user.id)}
                              disabled={alreadyAssigned}
                              onSelect={() => {
                                setSelectedUserObj(user)
                                setUserComboOpen(false)
                              }}
                              className="flex items-center gap-2"
                            >
                              {user.avatar ? (
                                <img src={user.avatar} alt={user.username} className="h-5 w-5 rounded-full" />
                              ) : (
                                <div className="flex h-5 w-5 items-center justify-center rounded-full bg-muted text-[10px]">
                                  {user.username.charAt(0).toUpperCase()}
                                </div>
                              )}
                              <span>{user.username}</span>
                              {user.email && <span className="text-xs text-muted-foreground">{user.email}</span>}
                              {alreadyAssigned && <span className="text-xs text-muted-foreground">({t("org:assignments.alreadyAssigned")})</span>}
                              {isSelected && <Check className="ml-auto h-4 w-4" />}
                            </CommandItem>
                          )
                        })}
                      </CommandGroup>
                    </CommandList>
                  </Command>
                </PopoverContent>
              </Popover>
            </div>

            {/* Position multi-select */}
            <div className="space-y-2">
              <label className="text-sm font-medium">
                {t("org:assignments.selectPositions")}
                {selectedPositionIds.length > 1 && (
                  <span className="ml-1 text-xs font-normal text-muted-foreground">
                    ({t("org:assignments.primaryHint")})
                  </span>
                )}
              </label>
              {!hasPositionsConfigured ? (
                <div className="rounded-md border border-dashed p-4 text-center text-sm text-muted-foreground">
                  <p>{t("org:assignments.noPositionsConfigured")}</p>
                  <p className="mt-1 text-xs">{t("org:assignments.configurePositionsHint")}</p>
                </div>
              ) : (
                <>
                  <Popover open={posPopoverOpen} onOpenChange={setPosPopoverOpen}>
                    <PopoverTrigger asChild>
                      <Button
                        variant="outline"
                        role="combobox"
                        aria-expanded={posPopoverOpen}
                        className="w-full justify-between font-normal"
                        type="button"
                      >
                        <span className="truncate text-muted-foreground">
                          {selectedPositionIds.length > 0
                            ? t("org:assignments.positionsSelected", { count: selectedPositionIds.length })
                            : t("org:assignments.selectPositions")}
                        </span>
                        <ChevronsUpDown className="ml-2 h-4 w-4 shrink-0 opacity-50" />
                      </Button>
                    </PopoverTrigger>
                    <PopoverContent className="w-[var(--radix-popover-trigger-width)] p-0" align="start">
                      <Command>
                        <CommandInput placeholder={t("org:positions.searchPlaceholder")} />
                        <CommandList>
                          <CommandEmpty>{t("common:noData")}</CommandEmpty>
                          <CommandGroup>
                            {positionsData?.map((pos) => {
                              const isSelected = selectedPositionIds.includes(pos.id)
                              const isPrimary = primaryPositionId === pos.id
                              return (
                                <CommandItem
                                  key={pos.id}
                                  value={pos.name}
                                  onSelect={() => togglePosition(pos.id)}
                                  className="flex items-center justify-between"
                                >
                                  <span className="flex items-center gap-2">
                                    <Check
                                      className={cn("h-4 w-4", isSelected ? "opacity-100" : "opacity-0")}
                                    />
                                    {pos.name}
                                  </span>
                                  {isSelected && (
                                    isPrimary ? (
                                      <Badge variant="default" className="h-5 gap-0.5 px-1.5 text-[10px]">
                                        <Star className="h-3 w-3 fill-current" />
                                        {t("org:assignments.primaryBadge")}
                                      </Badge>
                                    ) : (
                                      <button
                                        type="button"
                                        className="rounded px-1.5 py-0.5 text-xs text-muted-foreground transition-colors hover:bg-accent hover:text-accent-foreground"
                                        onClick={(e) => {
                                          e.stopPropagation()
                                          setPrimaryPositionId(pos.id)
                                        }}
                                      >
                                        {t("org:assignments.setAsPrimary")}
                                      </button>
                                    )
                                  )}
                                </CommandItem>
                              )
                            })}
                          </CommandGroup>
                        </CommandList>
                      </Command>
                    </PopoverContent>
                  </Popover>
                  {selectedPositionIds.length > 0 && (
                    <div className="flex flex-wrap gap-1.5 pt-1.5">
                      {selectedPositionIds.map((posId) => {
                        const pos = positionsData?.find((p) => p.id === posId)
                        if (!pos) return null
                        const isPrimary = primaryPositionId === posId
                        return (
                          <Badge
                            key={posId}
                            variant={isPrimary ? "default" : "secondary"}
                            className="gap-1 py-0.5 pr-1"
                          >
                            {isPrimary && <Star className="h-3 w-3 fill-current" />}
                            {pos.name}
                            <button
                              type="button"
                              className="ml-0.5 rounded-full p-0.5 transition-colors hover:bg-background/20"
                              onClick={() => togglePosition(posId)}
                            >
                              <X className="h-3 w-3" />
                            </button>
                          </Badge>
                        )
                      })}
                    </div>
                  )}
                </>
              )}
            </div>
          </div>

          <SheetFooter className="px-6 py-4">
            <Button variant="outline" type="button" onClick={() => onOpenChange(false)}>
              {t("common:cancel")}
            </Button>
            <Button type="submit" disabled={!canSubmit}>
              {addMutation.isPending ? t("common:saving") : t("common:confirm")}
            </Button>
          </SheetFooter>
        </form>
      </SheetContent>
    </Sheet>
  )
}

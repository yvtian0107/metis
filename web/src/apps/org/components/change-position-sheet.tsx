import { useState } from "react"
import { useTranslation } from "react-i18next"
import { useMutation, useQuery } from "@tanstack/react-query"
import { api } from "@/lib/api"
import { toast } from "sonner"
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
import { ChevronsUpDown, Check, X, Star } from "lucide-react"
import type { MemberPositionItem } from "../types"

interface PositionOption {
  id: number
  name: string
  isActive: boolean
}

interface EditPositionsSheetProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  userId: number
  departmentId: number
  currentPositions: MemberPositionItem[]
  onSuccess: () => void
}

export function EditPositionsSheet({
  open,
  onOpenChange,
  userId,
  departmentId,
  currentPositions,
  onSuccess,
}: EditPositionsSheetProps) {
  const { t } = useTranslation(["org", "common"])

  const [selectedPositionIds, setSelectedPositionIds] = useState<number[]>(
    () => currentPositions.map((p) => p.positionId)
  )
  const [primaryPositionId, setPrimaryPositionId] = useState<number | null>(
    () => currentPositions.find((p) => p.isPrimary)?.positionId ?? null
  )
  const [posPopoverOpen, setPosPopoverOpen] = useState(false)

  // Reset state when sheet opens with new data
  function handleOpenChange(nextOpen: boolean) {
    if (nextOpen) {
      setSelectedPositionIds(currentPositions.map((p) => p.positionId))
      setPrimaryPositionId(currentPositions.find((p) => p.isPrimary)?.positionId ?? null)
      setPosPopoverOpen(false)
    }
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

  const { data: positions } = useQuery({
    queryKey: ["departments", departmentId, "positions"],
    queryFn: async () => {
      const res = await api.get<{ items: PositionOption[] }>(`/api/v1/org/departments/${departmentId}/positions`)
      return res.items
    },
    enabled: open && !!departmentId,
  })

  const mutation = useMutation({
    mutationFn: async () => {
      await api.put(`/api/v1/org/users/${userId}/departments/${departmentId}/positions`, {
        positionIds: selectedPositionIds,
        primaryPositionId: primaryPositionId,
      })
    },
    onSuccess: () => {
      toast.success(t("org:assignments.editPositionsSuccess"))
      onSuccess()
      onOpenChange(false)
    },
    onError: (err: Error) => toast.error(err.message),
  })

  const hasChanges = (() => {
    const oldIds = currentPositions.map((p) => p.positionId).sort()
    const newIds = [...selectedPositionIds].sort()
    if (oldIds.length !== newIds.length) return true
    for (let i = 0; i < oldIds.length; i++) {
      if (oldIds[i] !== newIds[i]) return true
    }
    const oldPrimary = currentPositions.find((p) => p.isPrimary)?.positionId ?? null
    return oldPrimary !== primaryPositionId
  })()

  return (
    <Sheet open={open} onOpenChange={handleOpenChange}>
      <SheetContent className="gap-0 p-0 sm:max-w-md">
        <SheetHeader className="border-b px-6 py-5">
          <SheetTitle>{t("org:assignments.editPositions")}</SheetTitle>
          <SheetDescription className="sr-only">
            {t("org:assignments.editPositions")}
          </SheetDescription>
        </SheetHeader>
        <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
          <div className="flex-1 overflow-auto px-6 py-6">
            <div className="space-y-2">
              <label className="text-sm font-medium">
                {t("org:assignments.selectPositions")}
                {selectedPositionIds.length > 1 && (
                  <span className="ml-1 text-xs font-normal text-muted-foreground">
                    ({t("org:assignments.primaryHint")})
                  </span>
                )}
              </label>
              <Popover open={posPopoverOpen} onOpenChange={setPosPopoverOpen}>
                <PopoverTrigger asChild>
                  <Button
                    variant="outline"
                    role="combobox"
                    aria-expanded={posPopoverOpen}
                    className="w-full justify-between font-normal"
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
                        {positions?.map((pos) => {
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
                    const pos = positions?.find((p) => p.id === posId)
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
            </div>
          </div>
          <SheetFooter className="px-6 py-4">
            <Button variant="outline" onClick={() => onOpenChange(false)}>
              {t("common:cancel")}
            </Button>
            <Button
              onClick={() => mutation.mutate()}
              disabled={selectedPositionIds.length === 0 || !hasChanges || mutation.isPending}
            >
              {mutation.isPending ? t("common:saving") : t("common:confirm")}
            </Button>
          </SheetFooter>
        </div>
      </SheetContent>
    </Sheet>
  )
}

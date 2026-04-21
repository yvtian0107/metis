import { useMemo, useState } from "react"
import { useTranslation } from "react-i18next"
import { BookOpen, ChevronRight, Code, Globe, Loader2, Search, Wrench } from "lucide-react"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import { Input } from "@/components/ui/input"
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet"
import { cn } from "@/lib/utils"

export interface BindingItem {
  id: number
  name: string
  displayName?: string
  description?: string
  isExecutable?: boolean
  availabilityStatus?: string
  availabilityReason?: string
}

export interface BindingGroup {
  key: string
  id?: number
  title: string
  description: string
  items: BindingItem[]
}

interface BindingSelectorSectionProps {
  title: string
  description: string
  groups?: BindingGroup[]
  items?: BindingItem[]
  isLoading: boolean
  value: number[]
  onChange: (ids: number[]) => void
  groupValues?: Record<string, number[]>
  onGroupItemsChange?: (group: BindingGroup, ids: number[]) => void
  sheetTitle?: string
  sheetDescription?: string
  emphasize?: boolean
}

interface ResolvedBindingItem {
  id: number
  name: string
  description?: string
  isDisabled?: boolean
  disabledReason?: string
  availabilityStatus?: string
}

interface ResolvedBindingGroup {
  key: string
  id?: number
  title: string
  description: string
  items: ResolvedBindingItem[]
}

const TOOLKIT_ICONS: Record<string, React.ElementType> = {
  knowledge: BookOpen,
  network: Globe,
  code: Code,
}

export function BindingSelectorSection({
  title,
  description,
  groups,
  items,
  isLoading,
  value,
  onChange,
  groupValues,
  onGroupItemsChange,
  sheetTitle,
  sheetDescription,
  emphasize = false,
}: BindingSelectorSectionProps) {
  const { t } = useTranslation(["ai", "common"])
  const [open, setOpen] = useState(false)
  const [activeGroupKey, setActiveGroupKey] = useState<string | null>(null)
  const [query, setQuery] = useState("")

  const resolvedItems = useMemo<ResolvedBindingItem[]>(() => {
    return (items ?? []).map((item) => ({
      id: item.id,
      name: t(`ai:tools.toolDefs.${item.name}.name`, { defaultValue: item.displayName || item.name }),
      description: item.description
        ? t(`ai:tools.toolDefs.${item.name}.description`, { defaultValue: item.description })
        : undefined,
      isDisabled: item.isExecutable === false,
      disabledReason: item.availabilityReason,
      availabilityStatus: item.availabilityStatus,
    }))
  }, [items, t])

  const resolvedGroups = useMemo<ResolvedBindingGroup[]>(() => {
    return (groups ?? []).map((group) => ({
      key: group.key,
      id: group.id,
      title: group.title,
      description: group.description,
      items: group.items.map((item) => ({
        id: item.id,
        name: t(`ai:tools.toolDefs.${item.name}.name`, { defaultValue: item.displayName || item.name }),
        description: item.description
          ? t(`ai:tools.toolDefs.${item.name}.description`, { defaultValue: item.description })
          : undefined,
        isDisabled: item.isExecutable === false,
        disabledReason: item.availabilityReason,
        availabilityStatus: item.availabilityStatus,
      })),
    }))
  }, [groups, t])

  const selectedItems = useMemo(() => {
    const source = resolvedGroups.length > 0 ? resolvedGroups.flatMap((group) => group.items) : resolvedItems
    return source.filter((item) => value.includes(item.id))
  }, [resolvedGroups, resolvedItems, value])

  const filteredItems = useMemo(() => {
    const trimmed = query.trim().toLowerCase()
    if (!trimmed) return resolvedItems
    return resolvedItems.filter((item) => `${item.name} ${item.description || ""}`.toLowerCase().includes(trimmed))
  }, [query, resolvedItems])

  const filteredGroups = useMemo(() => {
    const sourceGroups = activeGroupKey
      ? resolvedGroups.filter((group) => group.key === activeGroupKey)
      : resolvedGroups
    const trimmed = query.trim().toLowerCase()
    if (!trimmed) return sourceGroups
    return sourceGroups
      .map((group) => ({
        ...group,
        items: group.items.filter((item) => `${item.name} ${item.description || ""}`.toLowerCase().includes(trimmed)),
      }))
      .filter((group) => group.items.length > 0)
  }, [activeGroupKey, query, resolvedGroups])

  const activeResolvedGroup = useMemo(() => {
    if (!activeGroupKey) return undefined
    return resolvedGroups.find((group) => group.key === activeGroupKey)
  }, [activeGroupKey, resolvedGroups])

  const activeOriginalGroup = useMemo(() => {
    if (!activeGroupKey) return undefined
    return (groups ?? []).find((group) => group.key === activeGroupKey)
  }, [activeGroupKey, groups])

  function selectedIDsForGroup(group: ResolvedBindingGroup) {
    return groupValues?.[group.key] ?? group.items.filter((item) => value.includes(item.id)).map((item) => item.id)
  }

  function toggle(id: number) {
    if (activeResolvedGroup) {
      const target = activeResolvedGroup.items.find((item) => item.id === id)
      if (target?.isDisabled) return
      const current = selectedIDsForGroup(activeResolvedGroup)
      const next = current.includes(id) ? current.filter((itemId) => itemId !== id) : [...current, id]
      const groupItemIDs = activeResolvedGroup.items.map((item) => item.id)
      const nextFlat = [...value.filter((itemId) => !groupItemIDs.includes(itemId)), ...next]
      onChange(Array.from(new Set(nextFlat)))
      if (activeOriginalGroup) {
        onGroupItemsChange?.(activeOriginalGroup, next)
      }
      return
    }
    const target = resolvedItems.find((item) => item.id === id)
    if (target?.isDisabled) return
    if (value.includes(id)) {
      onChange(value.filter((itemId) => itemId !== id))
      return
    }
    onChange([...value, id])
  }

  function clearSelection() {
    if (activeResolvedGroup) {
      const groupItemIDs = activeResolvedGroup.items.map((item) => item.id)
      onChange(value.filter((itemId) => !groupItemIDs.includes(itemId)))
      if (activeOriginalGroup) {
        onGroupItemsChange?.(activeOriginalGroup, [])
      }
      return
    }
    onChange([])
  }

  return (
    <>
      <section className="space-y-4">
        <div className="flex items-start justify-between gap-3">
          <div className="space-y-1.5">
            <div className="flex items-center gap-2">
              <h3 className="text-base font-semibold text-foreground">{title}</h3>
              <Badge variant={value.length > 0 ? "default" : "outline"}>
                {t("ai:agents.selectedCount", { count: value.length })}
              </Badge>
            </div>
            <p className="text-sm leading-6 text-muted-foreground">{description}</p>
          </div>
          {resolvedGroups.length === 0 && (
            <Button type="button" variant="ghost" size="sm" onClick={() => setOpen(true)} className="shrink-0">
              {t("ai:agents.manageSelection")}
              <ChevronRight className="size-4" />
            </Button>
          )}
        </div>

        {isLoading ? (
          <div className="flex min-h-24 items-center justify-center rounded-[1rem] border border-dashed border-border/55 bg-background/20">
            <Loader2 className="size-4 animate-spin text-muted-foreground" />
          </div>
        ) : resolvedGroups.length > 0 ? (
          <div className="grid grid-cols-1 gap-4 xl:grid-cols-2">
            {resolvedGroups.map((group) => {
              const groupSelectedIDs = selectedIDsForGroup(group)
              const groupSelected = group.items.filter((item) => groupSelectedIDs.includes(item.id))
              const Icon = TOOLKIT_ICONS[group.key] ?? Wrench
              return (
                <button
                  key={group.key}
                  type="button"
                  onClick={() => {
                    setActiveGroupKey(group.key)
                    setOpen(true)
                  }}
                  className={cn(
                    "group rounded-[1.1rem] border border-border/60 bg-background/30 px-4 py-4 text-left transition-colors hover:border-border/90 hover:bg-accent/20",
                    emphasize && "first:border-primary/25 first:bg-primary/[0.035]"
                  )}
                >
                  <div className="flex items-start justify-between gap-3">
                    <div className="flex items-start gap-3">
                      <div className="mt-0.5 flex size-10 shrink-0 items-center justify-center rounded-xl border border-border/55 bg-background/70 text-primary">
                        <Icon className="size-5" />
                      </div>
                      <div className="space-y-1">
                        <div className="flex items-center gap-2">
                          <p className="text-sm font-semibold text-foreground">{group.title}</p>
                          <Badge variant={groupSelected.length > 0 ? "default" : "outline"}>
                            {groupSelected.length}/{group.items.length}
                          </Badge>
                        </div>
                        <p className="text-sm leading-6 text-muted-foreground">{group.description}</p>
                      </div>
                    </div>
                    <span className="mt-1 flex size-8 shrink-0 items-center justify-center rounded-full border border-border/60 bg-background/70 text-muted-foreground transition-colors group-hover:text-foreground">
                      <ChevronRight className="size-4" />
                    </span>
                  </div>

                  <div className="mt-4 border-t border-border/45 pt-4">
                    {groupSelected.length === 0 ? (
                      <p className="text-sm text-muted-foreground">{t("ai:agents.clickToSelect")}</p>
                    ) : (
                      <div className="space-y-3">
                        <div className="flex flex-wrap gap-2">
                          {groupSelected.slice(0, 4).map((item) => (
                            <Badge key={item.id} variant="outline">
                              {item.name}
                            </Badge>
                          ))}
                          {groupSelected.length > 4 && <Badge variant="secondary">+{groupSelected.length - 4}</Badge>}
                        </div>
                        <p className="text-xs leading-5 text-muted-foreground">
                          {groupSelected[0]?.description || t("ai:agents.manageInSheet")}
                        </p>
                      </div>
                    )}
                  </div>
                </button>
              )
            })}
          </div>
        ) : (
          <button
            type="button"
            onClick={() => {
              setActiveGroupKey(null)
              setOpen(true)
            }}
            className={cn(
              "group w-full rounded-[1.1rem] border border-border/60 bg-background/30 px-4 py-4 text-left transition-colors hover:border-border/90 hover:bg-accent/20",
              emphasize && "border-primary/25 bg-primary/[0.04]"
            )}
          >
            {selectedItems.length === 0 ? (
              <div className="flex min-h-24 items-center justify-center rounded-[0.9rem] border border-dashed border-border/55 bg-background/25 text-center">
                <p className="text-sm text-muted-foreground">{t("ai:agents.clickToSelect")}</p>
              </div>
            ) : (
              <div className="space-y-3">
                <div className="flex flex-wrap gap-2">
                  {selectedItems.slice(0, 5).map((item) => (
                    <Badge key={item.id} variant="outline">
                      {item.name}
                    </Badge>
                  ))}
                  {selectedItems.length > 5 && <Badge variant="secondary">+{selectedItems.length - 5}</Badge>}
                </div>
                <p className="text-xs leading-5 text-muted-foreground">
                  {selectedItems[0]?.description || t("ai:agents.manageInSheet")}
                </p>
              </div>
            )}
          </button>
        )}
      </section>

      <Sheet
        open={open}
        onOpenChange={(nextOpen) => {
          setOpen(nextOpen)
          if (!nextOpen) {
            setActiveGroupKey(null)
            setQuery("")
          }
        }}
      >
        <SheetContent side="right" className="w-full sm:max-w-xl">
          <SheetHeader className="border-b border-border/50 pb-4">
            <SheetTitle>{activeResolvedGroup?.title || sheetTitle || title}</SheetTitle>
            <SheetDescription>{activeResolvedGroup?.description || sheetDescription || description}</SheetDescription>
          </SheetHeader>

          <div className="flex min-h-0 flex-1 flex-col gap-4 px-4 pb-4">
            <div className="mt-1 flex items-center gap-2 rounded-xl border border-border/60 bg-background/55 px-3 py-2">
              <Search className="size-4 text-muted-foreground" />
              <Input
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                placeholder={t("common:search")}
                className="h-auto border-0 bg-transparent px-0 py-0 shadow-none focus-visible:ring-0"
              />
            </div>

            <div className="flex items-center justify-between gap-3 rounded-xl border border-border/55 bg-background/30 px-4 py-3">
              <div>
                <p className="text-sm font-medium">{t("ai:agents.currentSelection")}</p>
                <p className="text-xs text-muted-foreground">
                  {t("ai:agents.selectedCount", {
                    count: activeResolvedGroup ? selectedIDsForGroup(activeResolvedGroup).length : value.length,
                  })}
                </p>
              </div>
              <Button
                type="button"
                variant="ghost"
                size="sm"
                onClick={clearSelection}
                disabled={activeResolvedGroup ? selectedIDsForGroup(activeResolvedGroup).length === 0 : value.length === 0}
              >
                {t("ai:agents.clearSelection")}
              </Button>
            </div>

            <div className="min-h-0 flex-1 overflow-y-auto pr-1">
              {isLoading ? (
                <div className="flex h-full min-h-48 items-center justify-center rounded-[1rem] border border-dashed border-border/55 bg-background/20">
                  <Loader2 className="size-4 animate-spin text-muted-foreground" />
                </div>
              ) : resolvedGroups.length > 0 ? (
                <div className="space-y-6">
                  {filteredGroups.map((group) => (
                    <div key={group.key} className="space-y-3">
                      <div className="space-y-1">
                        <h4 className="text-sm font-semibold text-foreground">{group.title}</h4>
                        <p className="text-xs leading-5 text-muted-foreground">{group.description}</p>
                      </div>
                      <div className="space-y-3">
                        {group.items.map((item) => {
                          const checked = selectedIDsForGroup(group).includes(item.id)
                          const disabled = item.isDisabled
                          return (
                            <label
                              key={item.id}
                              className={cn(
                                "flex items-start gap-3 rounded-[1rem] border px-4 py-3 transition-colors",
                                disabled ? "cursor-not-allowed border-border/40 bg-muted/20 opacity-70" : "cursor-pointer hover:border-border/90 hover:bg-accent/24",
                                checked ? "border-primary/30 bg-primary/[0.06]" : "border-border/55 bg-background/42"
                              )}
                            >
                              <Checkbox checked={checked} disabled={disabled} onCheckedChange={() => toggle(item.id)} className="mt-0.5" />
                              <div className="min-w-0 flex-1 space-y-1">
                                <div className="flex items-center justify-between gap-3">
                                  <span className="truncate text-sm font-medium text-foreground">{item.name}</span>
                                  {checked ? (
                                    <Badge variant="default">{t("ai:agents.selected")}</Badge>
                                  ) : disabled && item.availabilityStatus ? (
                                    <Badge variant="outline">{t(`ai:tools.builtin.availability.${item.availabilityStatus}`)}</Badge>
                                  ) : null}
                                </div>
                                <p className="line-clamp-2 text-xs leading-5 text-muted-foreground">
                                  {item.description || t("ai:agents.noItemDescription")}
                                </p>
                                {disabled && item.disabledReason && (
                                  <p className="text-xs leading-5 text-muted-foreground">{item.disabledReason}</p>
                                )}
                              </div>
                            </label>
                          )
                        })}
                      </div>
                    </div>
                  ))}
                </div>
              ) : filteredItems.length === 0 ? (
                <div className="flex h-full min-h-48 flex-col items-center justify-center gap-2 rounded-[1rem] border border-dashed border-border/55 bg-background/20 px-6 text-center">
                  <p className="text-sm font-medium text-foreground">{t("ai:agents.noMatchingItems")}</p>
                  <p className="text-xs leading-5 text-muted-foreground">{t("ai:agents.noItemsHint")}</p>
                </div>
              ) : (
                <div className="space-y-3">
                  {filteredItems.map((item) => {
                    const checked = value.includes(item.id)
                    const disabled = item.isDisabled
                    return (
                      <label
                        key={item.id}
                        className={cn(
                          "flex items-start gap-3 rounded-[1rem] border px-4 py-3 transition-colors",
                          disabled ? "cursor-not-allowed border-border/40 bg-muted/20 opacity-70" : "cursor-pointer hover:border-border/90 hover:bg-accent/24",
                          checked ? "border-primary/30 bg-primary/[0.06]" : "border-border/55 bg-background/42"
                        )}
                      >
                        <Checkbox checked={checked} disabled={disabled} onCheckedChange={() => toggle(item.id)} className="mt-0.5" />
                        <div className="min-w-0 flex-1 space-y-1">
                          <div className="flex items-center justify-between gap-3">
                            <span className="truncate text-sm font-medium text-foreground">{item.name}</span>
                            {checked ? (
                              <Badge variant="default">{t("ai:agents.selected")}</Badge>
                            ) : disabled && item.availabilityStatus ? (
                              <Badge variant="outline">{t(`ai:tools.builtin.availability.${item.availabilityStatus}`)}</Badge>
                            ) : null}
                          </div>
                          <p className="line-clamp-2 text-xs leading-5 text-muted-foreground">
                            {item.description || t("ai:agents.noItemDescription")}
                          </p>
                          {disabled && item.disabledReason && (
                            <p className="text-xs leading-5 text-muted-foreground">{item.disabledReason}</p>
                          )}
                        </div>
                      </label>
                    )
                  })}
                </div>
              )}
            </div>
          </div>

          <SheetFooter className="px-4">
            <Button type="button" variant="outline" onClick={() => setOpen(false)}>
              {t("common:close")}
            </Button>
            <Button type="button" onClick={() => setOpen(false)}>
              {t("common:confirm")}
            </Button>
          </SheetFooter>
        </SheetContent>
      </Sheet>
    </>
  )
}

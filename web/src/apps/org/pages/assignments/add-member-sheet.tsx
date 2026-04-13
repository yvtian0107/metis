import { useState } from "react"
import { useTranslation } from "react-i18next"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { useForm, Controller } from "react-hook-form"
import { z } from "zod"
import { zodResolver } from "@hookform/resolvers/zod"
import { api } from "@/lib/api"
import { toast } from "sonner"
import { useDebouncedValue } from "@/hooks/use-debounce"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetDescription,
  SheetFooter,
} from "@/components/ui/sheet"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandItem,
  CommandList,
} from "@/components/ui/command"
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover"
import { ChevronsUpDown, Check, Search } from "lucide-react"
import type { TreeNode, PositionItem, UserItem } from "./types"

const addMemberSchema = z.object({
  userId: z.number().min(1),
  positionId: z.number().min(1),
  isPrimary: z.boolean(),
})

type AddMemberForm = z.infer<typeof addMemberSchema>

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

  const form = useForm<AddMemberForm>({
    resolver: zodResolver(addMemberSchema),
    defaultValues: { userId: 0, positionId: 0, isPrimary: false },
  })

  // Track selected user separately to avoid form.watch() (incompatible with React Compiler)
  const [selectedUserObj, setSelectedUserObj] = useState<UserItem | null>(null)
  const [userComboOpen, setUserComboOpen] = useState(false)
  const [userKeyword, setUserKeyword] = useState("")
  const debouncedUserKeyword = useDebouncedValue(userKeyword, 300)

  function resetSheet() {
    form.reset({ userId: 0, positionId: 0, isPrimary: false })
    setSelectedUserObj(null)
    setUserKeyword("")
    setUserComboOpen(false)
  }

  function handleOpenChange(nextOpen: boolean) {
    if (nextOpen) resetSheet()
    onOpenChange(nextOpen)
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
    queryKey: ["positions", "all"],
    queryFn: async () => {
      const res = await api.get<{ items: PositionItem[] }>("/api/v1/org/positions?pageSize=0")
      return res.items
    },
  })

  const addMutation = useMutation({
    mutationFn: async (data: AddMemberForm) => {
      await api.post(`/api/v1/org/users/${data.userId}/positions`, {
        departmentId: deptId,
        positionId: data.positionId,
        isPrimary: data.isPrimary,
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

  function onSubmit(data: AddMemberForm) {
    addMutation.mutate(data)
  }

  const canSubmit = !!selectedUserObj && addMutation.isPending === false

  return (
    <Sheet open={open} onOpenChange={handleOpenChange}>
      <SheetContent className="gap-0 p-0 sm:max-w-md">
        <SheetHeader className="border-b px-6 py-5">
          <SheetTitle>{t("org:assignments.addMemberTo", { dept: selectedDept?.name ?? "" })}</SheetTitle>
          <SheetDescription className="sr-only">
            {t("org:assignments.addMember")}
          </SheetDescription>
        </SheetHeader>
        <form onSubmit={form.handleSubmit(onSubmit)} className="flex min-h-0 flex-1 flex-col overflow-hidden">
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
                                form.setValue("userId", user.id, { shouldValidate: true })
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

            {/* Position select */}
            <div className="space-y-2">
              <label className="text-sm font-medium">{t("org:assignments.selectPosition")}</label>
              <Controller
                control={form.control}
                name="positionId"
                render={({ field }) => (
                  <Select
                    value={field.value ? String(field.value) : ""}
                    onValueChange={(v) => field.onChange(Number(v))}
                  >
                    <SelectTrigger className="w-full">
                      <SelectValue placeholder={t("org:assignments.selectPosition")} />
                    </SelectTrigger>
                    <SelectContent>
                      {positionsData?.filter((p) => p.isActive).map((pos) => (
                        <SelectItem key={pos.id} value={String(pos.id)}>
                          {pos.name}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                )}
              />
            </div>

            {/* Primary checkbox */}
            <Controller
              control={form.control}
              name="isPrimary"
              render={({ field }) => (
                <div className="flex items-center gap-2">
                  <Checkbox
                    id="isPrimary"
                    checked={field.value}
                    onCheckedChange={(v) => field.onChange(v === true)}
                  />
                  <label htmlFor="isPrimary" className="cursor-pointer text-sm font-medium">
                    {t("org:assignments.setPrimary")}
                  </label>
                </div>
              )}
            />
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

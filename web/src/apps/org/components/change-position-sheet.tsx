import { useState } from "react"
import { useTranslation } from "react-i18next"
import { useMutation, useQuery } from "@tanstack/react-query"
import { api } from "@/lib/api"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
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

interface ChangePositionSheetProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  userId: number
  assignmentId: number
  currentPositionId: number
  onSuccess: () => void
}

interface PositionItem {
  id: number
  name: string
  isActive: boolean
}

export function ChangePositionSheet({
  open,
  onOpenChange,
  userId,
  assignmentId,
  currentPositionId,
  onSuccess,
}: ChangePositionSheetProps) {
  const { t } = useTranslation(["org", "common"])
  const [positionId, setPositionId] = useState<string>(String(currentPositionId))

  const { data: positions } = useQuery({
    queryKey: ["positions", "all"],
    queryFn: async () => {
      const res = await api.get<{ items: PositionItem[] }>("/api/v1/org/positions?pageSize=0")
      return res.items.filter((p) => p.isActive)
    },
  })

  const mutation = useMutation({
    mutationFn: async () => {
      await api.put(`/api/v1/org/users/${userId}/positions/${assignmentId}`, {
        positionId: Number(positionId),
      })
    },
    onSuccess: () => {
      toast.success(t("org:assignments.changePositionSuccess"))
      onSuccess()
      onOpenChange(false)
    },
    onError: (err: Error) => toast.error(err.message),
  })

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className="gap-0 p-0 sm:max-w-md">
        <SheetHeader className="border-b px-6 py-5">
          <SheetTitle>{t("org:assignments.changePosition")}</SheetTitle>
          <SheetDescription className="sr-only">
            {t("org:assignments.changePosition")}
          </SheetDescription>
        </SheetHeader>
        <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
          <div className="flex-1 overflow-auto px-6 py-6">
            <div className="space-y-2">
              <label className="text-sm font-medium">{t("org:assignments.selectPosition")}</label>
              <Select value={positionId} onValueChange={setPositionId}>
                <SelectTrigger className="w-full">
                  <SelectValue placeholder={t("org:assignments.selectPosition")} />
                </SelectTrigger>
                <SelectContent>
                  {positions?.map((pos) => (
                    <SelectItem key={pos.id} value={String(pos.id)}>
                      {pos.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          </div>
          <SheetFooter className="px-6 py-4">
            <Button variant="outline" onClick={() => onOpenChange(false)}>
              {t("common:cancel")}
            </Button>
            <Button
              onClick={() => mutation.mutate()}
              disabled={!positionId || Number(positionId) === currentPositionId || mutation.isPending}
            >
              {mutation.isPending ? t("common:saving") : t("common:confirm")}
            </Button>
          </SheetFooter>
        </div>
      </SheetContent>
    </Sheet>
  )
}

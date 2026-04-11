import { useMutation, useQueryClient } from "@tanstack/react-query"
import { useTranslation } from "react-i18next"
import { api } from "@/lib/api"
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

interface KickDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  sessionId: number | null
  username: string
}

export function KickDialog({ open, onOpenChange, sessionId, username }: KickDialogProps) {
  const { t } = useTranslation(["sessions", "common"])
  const queryClient = useQueryClient()

  const mutation = useMutation({
    mutationFn: (id: number) => api.delete(`/api/v1/sessions/${id}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["sessions"] })
      onOpenChange(false)
    },
  })

  return (
    <AlertDialog open={open} onOpenChange={onOpenChange}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{t("sessions:kickTitle")}</AlertDialogTitle>
          <AlertDialogDescription>
            {t("sessions:kickConfirm", { username })}
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel disabled={mutation.isPending}>{t("common:cancel")}</AlertDialogCancel>
          <AlertDialogAction
            onClick={() => sessionId && mutation.mutate(sessionId)}
            disabled={mutation.isPending}
          >
            {mutation.isPending ? t("common:processing") : t("sessions:confirmKick")}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}

import { useTranslation } from "react-i18next"
import { useForm } from "react-hook-form"
import { z } from "zod"
import { zodResolver } from "@hookform/resolvers/zod"
import { useMutation } from "@tanstack/react-query"
import { Loader2 } from "lucide-react"
import { api } from "@/lib/api"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Textarea } from "@/components/ui/textarea"
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "@/components/ui/dialog"
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from "@/components/ui/form"

type FormValues = {
  to: string
  subject: string
  body: string
}

interface SendTestDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  channelId: number | null
}

export function SendTestDialog({ open, onOpenChange, channelId }: SendTestDialogProps) {
  const { t } = useTranslation(["channels", "common"])

  const form = useForm<FormValues>({
    resolver: zodResolver(
      z.object({
        to: z.string().email(t("validation_email")),
        subject: z.string().min(1, t("validation_subjectRequired")),
        body: z.string().min(1, t("validation_bodyRequired")),
      }),
    ),
    defaultValues: {
      to: "",
      subject: t("sendTestDialog.defaultSubject"),
      body: t("sendTestDialog.defaultBody"),
    },
  })

  const sendMutation = useMutation({
    mutationFn: async (values: FormValues) => {
      const res = await api.post<{ success: boolean; error?: string }>(
        `/api/v1/channels/${channelId}/send-test`,
        values,
      )
      if (!res.success) throw new Error(res.error || t("toast.sendError"))
      return res
    },
    onSuccess: () => {
      toast.success(t("toast.sendSuccess"))
      onOpenChange(false)
    },
    onError: (err) => toast.error(t("toast.sendFailed", { message: err.message })),
  })

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{t("sendTestDialog.title")}</DialogTitle>
          <DialogDescription>
            {t("sendTestDialog.description")}
          </DialogDescription>
        </DialogHeader>
        <Form {...form}>
          <form onSubmit={form.handleSubmit((v) => sendMutation.mutate(v))} className="space-y-4">
            <FormField
              control={form.control}
              name="to"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t("sendTestDialog.recipient")}</FormLabel>
                  <FormControl>
                    <Input placeholder="test@example.com" {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name="subject"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t("sendTestDialog.subject")}</FormLabel>
                  <FormControl>
                    <Input {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name="body"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t("sendTestDialog.body")}</FormLabel>
                  <FormControl>
                    <Textarea rows={3} {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <DialogFooter>
              <Button type="submit" size="sm" disabled={sendMutation.isPending}>
                {sendMutation.isPending && <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />}
                {t("sendTestDialog.send")}
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  )
}

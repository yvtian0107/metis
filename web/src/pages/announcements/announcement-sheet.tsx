import { useEffect } from "react"
import { useTranslation } from "react-i18next"
import { useForm } from "react-hook-form"
import { z } from "zod"
import { zodResolver } from "@hookform/resolvers/zod"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { api } from "@/lib/api"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Textarea } from "@/components/ui/textarea"
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetDescription,
  SheetFooter,
} from "@/components/ui/sheet"
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from "@/components/ui/form"

interface Announcement {
  id: number
  title: string
  content: string
  createdAt: string
  updatedAt: string
  creatorUsername: string
}

type FormValues = {
  title: string
  content?: string
}

interface AnnouncementSheetProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  announcement: Announcement | null
}

export function AnnouncementSheet({ open, onOpenChange, announcement }: AnnouncementSheetProps) {
  const { t } = useTranslation(["announcements", "common"])
  const queryClient = useQueryClient()
  const isEditing = announcement !== null

  const schema = z.object({
    title: z.string().min(1, t("validation.titleRequired")).max(255),
    content: z.string().optional(),
  })

  const form = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: { title: "", content: "" },
  })

  useEffect(() => {
    if (open) {
      if (announcement) {
        form.reset({ title: announcement.title, content: announcement.content || "" })
      } else {
        form.reset({ title: "", content: "" })
      }
    }
  }, [open, announcement, form])

  const createMutation = useMutation({
    mutationFn: (values: FormValues) => api.post("/api/v1/announcements", values),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["announcements"] })
      queryClient.invalidateQueries({ queryKey: ["notifications-unread-count"] })
      onOpenChange(false)
    },
  })

  const updateMutation = useMutation({
    mutationFn: (values: FormValues) =>
      api.put(`/api/v1/announcements/${announcement!.id}`, values),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["announcements"] })
      onOpenChange(false)
    },
  })

  function onSubmit(values: FormValues) {
    if (isEditing) {
      updateMutation.mutate(values)
    } else {
      createMutation.mutate(values)
    }
  }

  const isPending = createMutation.isPending || updateMutation.isPending
  const error = createMutation.error || updateMutation.error

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className="sm:max-w-md">
        <SheetHeader>
          <SheetTitle>{isEditing ? t("sheet.editTitle") : t("sheet.createTitle")}</SheetTitle>
          <SheetDescription className="sr-only">
            {isEditing ? t("sheet.editDescription") : t("sheet.createDescription")}
          </SheetDescription>
        </SheetHeader>
        <Form {...form}>
          <form onSubmit={form.handleSubmit(onSubmit)} className="flex flex-1 flex-col gap-4 px-4">
            <FormField
              control={form.control}
              name="title"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t("form.titleLabel")}</FormLabel>
                  <FormControl>
                    <Input placeholder={t("form.titlePlaceholder")} {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name="content"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t("form.contentLabel")}</FormLabel>
                  <FormControl>
                    <Textarea
                      placeholder={t("form.contentPlaceholder")}
                      rows={6}
                      {...field}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            {error && (
              <p className="text-sm text-destructive">{error.message}</p>
            )}

            <SheetFooter>
              <Button type="submit" size="sm" disabled={isPending}>
                {isPending ? t("submit.saving") : isEditing ? t("submit.save") : t("submit.publish")}
              </Button>
            </SheetFooter>
          </form>
        </Form>
      </SheetContent>
    </Sheet>
  )
}

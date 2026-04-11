import { useTranslation } from "react-i18next"
import { useForm } from "react-hook-form"
import { z } from "zod"
import { zodResolver } from "@hookform/resolvers/zod"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { api } from "@/lib/api"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from "@/components/ui/form"

export function SiteNameCard({ appName }: { appName: string }) {
  const { t } = useTranslation(["settings", "common"])
  const queryClient = useQueryClient()

  const schema = z.object({
    appName: z.string().min(1, t("settings:site.appNameRequired")).max(50, t("settings:site.appNameMaxLength")),
  })

  type FormValues = z.infer<typeof schema>

  const form = useForm<FormValues>({
    resolver: zodResolver(schema),
    values: { appName },
  })

  const mutation = useMutation({
    mutationFn: (data: FormValues) => api.put("/api/v1/site-info", data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["site-info"] })
      form.reset(form.getValues())
    },
  })

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("settings:site.title")}</CardTitle>
        <CardDescription>{t("settings:site.description")}</CardDescription>
      </CardHeader>
      <CardContent>
        <Form {...form}>
          <form onSubmit={form.handleSubmit((v) => mutation.mutate(v))} className="flex items-end gap-3">
            <FormField
              control={form.control}
              name="appName"
              render={({ field }) => (
                <FormItem className="flex-1">
                  <FormLabel>{t("settings:site.appName")}</FormLabel>
                  <FormControl>
                    <Input placeholder={t("settings:site.appNamePlaceholder")} {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <Button
              type="submit"
              disabled={!form.formState.isDirty || mutation.isPending}
            >
              {mutation.isPending ? t("common:saving") : t("common:save")}
            </Button>
          </form>
        </Form>
      </CardContent>
    </Card>
  )
}

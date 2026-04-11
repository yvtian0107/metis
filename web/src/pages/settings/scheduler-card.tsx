import { useTranslation } from "react-i18next"
import { useForm } from "react-hook-form"
import { z } from "zod"
import { zodResolver } from "@hookform/resolvers/zod"
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
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
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from "@/components/ui/form"

type FormValues = {
  historyRetentionDays: number
  auditRetentionDaysAuth: number
  auditRetentionDaysOperation: number
}

type SchedulerSettings = FormValues

export function SchedulerCard() {
  const { t } = useTranslation(["settings", "common"])
  const queryClient = useQueryClient()

  const schema = z.object({
    historyRetentionDays: z.number().int().min(0, t("settings:scheduler.validation.numberMin")),
    auditRetentionDaysAuth: z.number().int().min(0, t("settings:scheduler.validation.numberMin")),
    auditRetentionDaysOperation: z.number().int().min(0, t("settings:scheduler.validation.numberMin")),
  })

  const { data, isLoading } = useQuery({
    queryKey: ["settings", "scheduler"],
    queryFn: () => api.get<SchedulerSettings>("/api/v1/settings/scheduler"),
  })

  const form = useForm<FormValues>({
    resolver: zodResolver(schema),
    values: data ?? {
      historyRetentionDays: 30,
      auditRetentionDaysAuth: 90,
      auditRetentionDaysOperation: 365,
    },
  })

  const mutation = useMutation({
    mutationFn: (values: FormValues) => api.put("/api/v1/settings/scheduler", values),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["settings", "scheduler"] })
      form.reset(form.getValues())
    },
  })

  if (isLoading) {
    return (
      <Card>
        <CardContent className="flex h-32 items-center justify-center text-muted-foreground">
          {t("common:loading")}
        </CardContent>
      </Card>
    )
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("settings:scheduler.title")}</CardTitle>
        <CardDescription>
          {t("settings:scheduler.description")}
        </CardDescription>
      </CardHeader>
      <CardContent>
        <Form {...form}>
          <form onSubmit={form.handleSubmit((v) => mutation.mutate(v))} className="space-y-4">
            <div className="grid gap-4 sm:grid-cols-3">
              <FormField
                control={form.control}
                name="historyRetentionDays"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t("settings:scheduler.historyRetention")}</FormLabel>
                    <FormControl>
                      <Input
                        type="number"
                        min={0}
                        {...field}
                        onChange={(e) => field.onChange(e.target.valueAsNumber)}
                      />
                    </FormControl>
                    <FormDescription>{t("settings:scheduler.historyRetentionDescription")}</FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name="auditRetentionDaysAuth"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t("settings:scheduler.auditRetentionAuth")}</FormLabel>
                    <FormControl>
                      <Input
                        type="number"
                        min={0}
                        {...field}
                        onChange={(e) => field.onChange(e.target.valueAsNumber)}
                      />
                    </FormControl>
                    <FormDescription>{t("settings:scheduler.auditRetentionAuthDescription")}</FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name="auditRetentionDaysOperation"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t("settings:scheduler.auditRetentionOperation")}</FormLabel>
                    <FormControl>
                      <Input
                        type="number"
                        min={0}
                        {...field}
                        onChange={(e) => field.onChange(e.target.valueAsNumber)}
                      />
                    </FormControl>
                    <FormDescription>{t("settings:scheduler.auditRetentionOperationDescription")}</FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>
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

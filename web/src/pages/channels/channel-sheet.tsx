import { useEffect, useMemo } from "react"
import { useTranslation } from "react-i18next"
import { useForm, useWatch } from "react-hook-form"
import { z } from "zod"
import { zodResolver } from "@hookform/resolvers/zod"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { Loader2, Plug } from "lucide-react"
import { api } from "@/lib/api"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Switch } from "@/components/ui/switch"
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
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from "@/components/ui/form"
import { CHANNEL_TYPES, type ConfigField } from "./channel-types"

export interface ChannelItem {
  id: number
  name: string
  type: string
  config: string
  enabled: boolean
  createdAt: string
  updatedAt: string
}

type FormValues = z.infer<ReturnType<typeof createSchema>>

function createSchema(nameRequired: string, typeRequired: string) {
  return z.object({
    name: z.string().min(1, nameRequired),
    type: z.string().min(1, typeRequired),
    config: z.record(z.string(), z.unknown()),
  })
}

interface ChannelSheetProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  channel: ChannelItem | null
}

export function ChannelSheet({ open, onOpenChange, channel }: ChannelSheetProps) {
  const { t } = useTranslation(["channels", "common"])
  const queryClient = useQueryClient()
  const isEditing = channel !== null

  const schema = useMemo(
    () => createSchema(t("validation.nameRequired"), t("validation.typeRequired")),
    [t],
  )

  const form = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: { name: "", type: "email", config: {} },
  })

  const selectedType = useWatch({ control: form.control, name: "type" })
  const configSchema = useMemo(
    () => CHANNEL_TYPES[selectedType]?.configSchema ?? [],
    [selectedType],
  )

  useEffect(() => {
    if (open) {
      if (channel) {
        let cfg: Record<string, unknown> = {}
        try { cfg = JSON.parse(channel.config) } catch { /* ignore */ }
        form.reset({ name: channel.name, type: channel.type, config: cfg })
      } else {
        // Build defaults from schema
        const defaults: Record<string, unknown> = {}
        const defaultType = "email"
        const fields = CHANNEL_TYPES[defaultType]?.configSchema ?? []
        for (const f of fields) {
          if (f.default !== undefined) defaults[f.key] = f.default
          else if (f.type === "boolean") defaults[f.key] = false
          else if (f.type === "number") defaults[f.key] = 0
          else defaults[f.key] = ""
        }
        form.reset({ name: "", type: defaultType, config: defaults })
      }
    }
  }, [open, channel, form])

  // Reset config defaults when type changes (only in create mode)
  useEffect(() => {
    if (!isEditing && open) {
      const defaults: Record<string, unknown> = {}
      for (const f of configSchema) {
        if (f.default !== undefined) defaults[f.key] = f.default
        else if (f.type === "boolean") defaults[f.key] = false
        else if (f.type === "number") defaults[f.key] = 0
        else defaults[f.key] = ""
      }
      form.setValue("config", defaults)
    }
  }, [selectedType, isEditing, open, configSchema, form])

  const createMutation = useMutation({
    mutationFn: (values: FormValues) =>
      api.post("/api/v1/channels", {
        name: values.name,
        type: values.type,
        config: JSON.stringify(values.config),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["channels"] })
      onOpenChange(false)
      toast.success(t("toast.createSuccess"))
    },
    onError: (err) => toast.error(err.message),
  })

  const updateMutation = useMutation({
    mutationFn: (values: FormValues) =>
      api.put(`/api/v1/channels/${channel!.id}`, {
        name: values.name,
        config: JSON.stringify(values.config),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["channels"] })
      onOpenChange(false)
      toast.success(t("toast.updateSuccess"))
    },
    onError: (err) => toast.error(err.message),
  })

  const testMutation = useMutation({
    mutationFn: async () => {
      if (!channel) throw new Error(t("validation.saveFirst"))
      const res = await api.post<{ success: boolean; error?: string }>(
        `/api/v1/channels/${channel.id}/test`,
      )
      if (!res.success) throw new Error(res.error || t("toast.testFailed"))
      return res
    },
    onSuccess: () => toast.success(t("toast.testConnSuccess")),
    onError: (err) => toast.error(t("toast.testConnFailed", { message: err.message })),
  })

  function onSubmit(values: FormValues) {
    if (isEditing) {
      updateMutation.mutate(values)
    } else {
      createMutation.mutate(values)
    }
  }

  const isPending = createMutation.isPending || updateMutation.isPending

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
              name="name"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t("form.nameLabel")}</FormLabel>
                  <FormControl>
                    <Input placeholder={t("form.namePlaceholder")} {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name="type"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t("form.typeLabel")}</FormLabel>
                  {isEditing ? (
                    <div>
                      <Input
                        value={t(CHANNEL_TYPES[field.value]?.labelKey ?? field.value)}
                        disabled
                      />
                      <p className="text-xs text-muted-foreground mt-1">{t("form.typeImmutable")}</p>
                    </div>
                  ) : (
                    <Select value={field.value} onValueChange={field.onChange}>
                      <FormControl>
                        <SelectTrigger>
                          <SelectValue placeholder={t("form.typePlaceholder")} />
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent>
                        {Object.entries(CHANNEL_TYPES).map(([key, def]) => (
                          <SelectItem key={key} value={key}>
                            {t(def.labelKey)}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  )}
                  <FormMessage />
                </FormItem>
              )}
            />

            {configSchema.length > 0 && (
              <div className="space-y-3 rounded-lg border p-4">
                <p className="text-sm font-medium">{t("form.connectionConfig")}</p>
                {configSchema.map((field) => (
                  <ConfigFieldInput
                    key={field.key}
                    field={field}
                    form={form}
                    isEditing={isEditing}
                  />
                ))}
              </div>
            )}

            <SheetFooter>
              {isEditing && (
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  onClick={() => testMutation.mutate()}
                  disabled={testMutation.isPending}
                >
                  {testMutation.isPending ? (
                    <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />
                  ) : (
                    <Plug className="mr-1.5 h-3.5 w-3.5" />
                  )}
                  {t("testConnection")}
                </Button>
              )}
              <Button type="submit" size="sm" disabled={isPending}>
                {isPending ? t("common:saving") : t("common:save")}
              </Button>
            </SheetFooter>
          </form>
        </Form>
      </SheetContent>
    </Sheet>
  )
}

function ConfigFieldInput({
  field,
  form,
  isEditing,
}: {
  field: ConfigField
  form: ReturnType<typeof useForm<FormValues>>
  isEditing: boolean
}) {
  const { t } = useTranslation("channels")
  const name = `config.${field.key}` as const

  if (field.type === "boolean") {
    return (
      <FormField
        control={form.control}
        name={name}
        render={({ field: formField }) => (
          <FormItem className="flex items-center justify-between">
            <FormLabel className="mt-0">{t(field.labelKey)}</FormLabel>
            <FormControl>
              <Switch
                checked={!!formField.value}
                onCheckedChange={formField.onChange}
              />
            </FormControl>
          </FormItem>
        )}
      />
    )
  }

  const placeholderText = field.sensitive && isEditing
    ? t("form.sensitiveHint")
    : field.placeholderKey
      ? (field.placeholderKey.includes(".") ? t(field.placeholderKey) : field.placeholderKey)
      : ""

  return (
    <FormField
      control={form.control}
      name={name}
      render={({ field: formField }) => (
        <FormItem>
          <FormLabel>{t(field.labelKey)}</FormLabel>
          <FormControl>
            <Input
              type={field.sensitive ? "password" : field.type === "number" ? "number" : "text"}
              placeholder={placeholderText}
              value={
                field.sensitive && isEditing && formField.value === "******"
                  ? ""
                  : (formField.value as string | number) ?? ""
              }
              onChange={(e) => {
                const val = field.type === "number"
                  ? Number(e.target.value)
                  : e.target.value
                formField.onChange(val)
              }}
            />
          </FormControl>
          <FormMessage />
        </FormItem>
      )}
    />
  )
}

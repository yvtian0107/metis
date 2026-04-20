import { useEffect } from "react"
import { useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { z } from "zod"
import { zodResolver } from "@hookform/resolvers/zod"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { api } from "@/lib/api"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"

export interface ProviderItem {
  id: number
  name: string
  type: string
  protocol: string
  baseUrl: string
  apiKeyMasked: string
  status: string
  healthCheckedAt: string | null
  modelCount: number
  modelTypeCounts: Record<string, number>
  createdAt: string
  updatedAt: string
}

const PROVIDER_TYPES = ["openai", "anthropic", "ollama"] as const

function useProviderSchema(isEditing: boolean) {
  const { t } = useTranslation("ai")
  return z.object({
    name: z.string().min(1, t("validation.nameRequired")).max(128),
    type: z.string().min(1, t("validation.typeRequired")),
    baseUrl: z.string().min(1, t("validation.baseUrlRequired")).max(512),
    apiKey: isEditing
      ? z.string().max(512).optional()
      : z.string().min(1, t("validation.apiKeyRequired")).max(512),
  })
}

type FormValues = z.infer<ReturnType<typeof useProviderSchema>>

interface ProviderSheetProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  provider: ProviderItem | null
}

const BASE_URL_DEFAULTS: Record<string, string> = {
  openai: "https://api.openai.com/v1",
  anthropic: "https://api.anthropic.com",
  ollama: "http://localhost:11434/v1",
}

export function ProviderSheet({ open, onOpenChange, provider }: ProviderSheetProps) {
  const { t } = useTranslation(["ai", "common"])
  const queryClient = useQueryClient()
  const isEditing = provider !== null
  const schema = useProviderSchema(isEditing)

  const form = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: {
      name: "",
      type: "openai",
      baseUrl: BASE_URL_DEFAULTS.openai,
      apiKey: "",
    },
  })

  const watchedType = form.watch("type")

  useEffect(() => {
    if (open) {
      if (provider) {
        form.reset({
          name: provider.name,
          type: provider.type,
          baseUrl: provider.baseUrl,
          apiKey: "",
        })
      } else {
        form.reset({
          name: "",
          type: "openai",
          baseUrl: BASE_URL_DEFAULTS.openai,
          apiKey: "",
        })
      }
    }
  }, [open, provider, form])

  // Auto-fill base URL when type changes (only for new providers)
  useEffect(() => {
    if (!isEditing && watchedType && BASE_URL_DEFAULTS[watchedType]) {
      form.setValue("baseUrl", BASE_URL_DEFAULTS[watchedType])
    }
  }, [watchedType, isEditing, form])

  const createMutation = useMutation({
    mutationFn: (values: FormValues) =>
      api.post("/api/v1/ai/providers", values),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["ai-providers"] })
      onOpenChange(false)
      toast.success(t("ai:providers.createSuccess"))
    },
    onError: (err) => toast.error(err.message),
  })

  const updateMutation = useMutation({
    mutationFn: (values: FormValues) =>
      api.put(`/api/v1/ai/providers/${provider!.id}`, values),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["ai-providers"] })
      onOpenChange(false)
      toast.success(t("ai:providers.updateSuccess"))
    },
    onError: (err) => toast.error(err.message),
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
      <SheetContent className="sm:max-w-lg overflow-y-auto">
        <SheetHeader>
          <SheetTitle>
            {isEditing ? t("ai:providers.edit") : t("ai:providers.create")}
          </SheetTitle>
          <SheetDescription className="sr-only">
            {isEditing ? t("ai:providers.edit") : t("ai:providers.create")}
          </SheetDescription>
        </SheetHeader>
        <Form {...form}>
          <form onSubmit={form.handleSubmit(onSubmit)} className="flex flex-1 flex-col gap-5 px-4">
            <FormField
              control={form.control}
              name="name"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t("ai:providers.name")}</FormLabel>
                  <FormControl>
                    <Input placeholder={t("ai:providers.namePlaceholder")} {...field} />
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
                  <FormLabel>{t("ai:providers.type")}</FormLabel>
                  <Select value={field.value} onValueChange={field.onChange}>
                    <FormControl>
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                    </FormControl>
                    <SelectContent>
                      {PROVIDER_TYPES.map((pt) => (
                        <SelectItem key={pt} value={pt}>
                          {t(`ai:types.${pt}`)}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name="baseUrl"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t("ai:providers.baseUrl")}</FormLabel>
                  <FormControl>
                    <Input placeholder={t("ai:providers.baseUrlPlaceholder")} {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name="apiKey"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t("ai:providers.apiKey")}</FormLabel>
                  <FormControl>
                    <Input
                      type="password"
                      placeholder={isEditing ? t("ai:providers.apiKeyHint") : t("ai:providers.apiKeyPlaceholder")}
                      {...field}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <SheetFooter>
              <Button type="submit" size="sm" disabled={isPending}>
                {isPending ? t("common:saving") : isEditing ? t("common:save") : t("common:create")}
              </Button>
            </SheetFooter>
          </form>
        </Form>
      </SheetContent>
    </Sheet>
  )
}

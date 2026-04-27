import { useEffect } from "react"
import { useForm, useWatch } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { z } from "zod"
import { zodResolver } from "@/lib/form"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { api, type PaginatedResponse } from "@/lib/api"
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
import { Checkbox } from "@/components/ui/checkbox"
import type { ProviderItem } from "./provider-sheet"

export interface ModelItem {
  id: number
  modelId: string
  displayName: string
  providerId: number
  providerName: string
  type: string
  capabilities: string[]
  contextWindow: number
  maxOutputTokens: number
  inputPrice: number
  outputPrice: number
  isDefault: boolean
  status: string
  createdAt: string
  updatedAt: string
}

const MODEL_TYPES = ["llm", "embed", "rerank", "tts", "stt", "image"] as const
const CAPABILITIES = ["vision", "tool_use", "reasoning", "coding", "long_context"] as const

function useModelSchema() {
  const { t } = useTranslation("ai")
  return z.object({
    modelId: z.string().min(1, t("validation.modelIdRequired")).max(128),
    displayName: z.string().min(1, t("validation.displayNameRequired")).max(128),
    providerId: z.coerce.number().min(1, t("validation.providerRequired")),
    type: z.string().min(1, t("validation.typeRequired")),
    capabilities: z.array(z.string()),
    contextWindow: z.coerce.number().int().min(0),
    maxOutputTokens: z.coerce.number().int().min(0),
    inputPrice: z.coerce.number().min(0),
    outputPrice: z.coerce.number().min(0),
    status: z.string(),
  })
}

type FormValues = z.infer<ReturnType<typeof useModelSchema>>

interface ModelSheetProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  model: ModelItem | null
  defaultProviderId?: number
  defaultType?: string
}

export function ModelSheet({ open, onOpenChange, model, defaultProviderId, defaultType }: ModelSheetProps) {
  const { t } = useTranslation(["ai", "common"])
  const queryClient = useQueryClient()
  const isEditing = model !== null
  const schema = useModelSchema()

  const { data: providersData } = useQuery({
    queryKey: ["ai-providers-all"],
    queryFn: () => api.get<PaginatedResponse<ProviderItem>>("/api/v1/ai/providers?pageSize=100"),
    enabled: open && !defaultProviderId,
  })
  const providers = providersData?.items ?? []

  const form = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: {
      modelId: "",
      displayName: "",
      providerId: 0,
      type: "llm",
      capabilities: [],
      contextWindow: 0,
      maxOutputTokens: 0,
      inputPrice: 0,
      outputPrice: 0,
      status: "active",
    },
  })

  const watchedType = useWatch({ control: form.control, name: "type" })

  useEffect(() => {
    if (open) {
      if (model) {
        form.reset({
          modelId: model.modelId,
          displayName: model.displayName,
          providerId: model.providerId,
          type: model.type,
          capabilities: Array.isArray(model.capabilities) ? model.capabilities : [],
          contextWindow: model.contextWindow,
          maxOutputTokens: model.maxOutputTokens,
          inputPrice: model.inputPrice,
          outputPrice: model.outputPrice,
          status: model.status,
        })
      } else {
        form.reset({
          modelId: "",
          displayName: "",
          providerId: defaultProviderId ?? 0,
          type: defaultType ?? "llm",
          capabilities: [],
          contextWindow: 0,
          maxOutputTokens: 0,
          inputPrice: 0,
          outputPrice: 0,
          status: "active",
        })
      }
    }
  }, [open, model, form, defaultProviderId, defaultType])

  const createMutation = useMutation({
    mutationFn: (values: FormValues) =>
      api.post("/api/v1/ai/models", values),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["ai-models"] })
      onOpenChange(false)
      toast.success(t("ai:models.createSuccess"))
    },
    onError: (err) => toast.error(err.message),
  })

  const updateMutation = useMutation({
    mutationFn: (values: FormValues) =>
      api.put(`/api/v1/ai/models/${model!.id}`, values),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["ai-models"] })
      onOpenChange(false)
      toast.success(t("ai:models.updateSuccess"))
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
            {isEditing ? t("ai:models.edit") : t("ai:models.create")}
          </SheetTitle>
          <SheetDescription className="sr-only">
            {isEditing ? t("ai:models.edit") : t("ai:models.create")}
          </SheetDescription>
        </SheetHeader>
        <Form {...form}>
          <form onSubmit={form.handleSubmit(onSubmit)} className="flex flex-1 flex-col gap-5 px-4">
            <FormField
              control={form.control}
              name="modelId"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t("ai:models.modelId")}</FormLabel>
                  <FormControl>
                    <Input placeholder={t("ai:models.modelIdPlaceholder")} {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name="displayName"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t("ai:models.displayName")}</FormLabel>
                  <FormControl>
                    <Input placeholder={t("ai:models.displayNamePlaceholder")} {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            {!defaultProviderId && (
              <FormField
                control={form.control}
                name="providerId"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t("ai:models.provider")}</FormLabel>
                    <Select
                      value={field.value ? String(field.value) : ""}
                      onValueChange={(v) => field.onChange(Number(v))}
                      disabled={isEditing}
                    >
                      <FormControl>
                        <SelectTrigger>
                          <SelectValue placeholder={t("ai:models.selectProvider")} />
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent>
                        {providers.map((p) => (
                          <SelectItem key={p.id} value={String(p.id)}>
                            {p.name}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                    <FormMessage />
                  </FormItem>
                )}
              />
            )}
            <div className="flex items-center gap-2">
              <FormField
                control={form.control}
                name="type"
                render={({ field }) => (
                  <FormItem className="flex-1">
                    <FormLabel>{t("ai:models.type")}</FormLabel>
                    <Select value={field.value || undefined} onValueChange={field.onChange}>
                      <FormControl>
                        <SelectTrigger>
                          <SelectValue placeholder={t("ai:models.selectType")} />
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent>
                        {MODEL_TYPES.map((mt) => (
                          <SelectItem key={mt} value={mt}>
                            {t(`ai:modelTypes.${mt}`)}
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
                name="status"
                render={({ field }) => (
                  <FormItem className="flex-1">
                    <FormLabel>{t("ai:models.status")}</FormLabel>
                    <Select value={field.value} onValueChange={field.onChange}>
                      <FormControl>
                        <SelectTrigger>
                          <SelectValue />
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent>
                        <SelectItem value="active">{t("ai:statusLabels.active")}</SelectItem>
                        <SelectItem value="deprecated">{t("ai:statusLabels.deprecated")}</SelectItem>
                      </SelectContent>
                    </Select>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>

            {watchedType === "llm" && (
              <FormField
                control={form.control}
                name="capabilities"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t("ai:models.capabilities")}</FormLabel>
                    <div className="flex flex-wrap gap-3">
                      {CAPABILITIES.map((cap) => {
                        const checked = field.value.includes(cap)
                        return (
                          <label key={cap} className="flex items-center gap-1.5 text-sm cursor-pointer">
                            <Checkbox
                              checked={checked}
                              onCheckedChange={(c) => {
                                const next = c
                                  ? [...field.value, cap]
                                  : field.value.filter((v) => v !== cap)
                                field.onChange(next)
                              }}
                            />
                            {t(`ai:capabilityLabels.${cap}`)}
                          </label>
                        )
                      })}
                    </div>
                    <FormMessage />
                  </FormItem>
                )}
              />
            )}

            <div className="flex items-center gap-2">
              <FormField
                control={form.control}
                name="contextWindow"
                render={({ field }) => (
                  <FormItem className="flex-1">
                    <FormLabel>{t("ai:models.contextWindow")}</FormLabel>
                    <FormControl>
                      <Input type="number" {...field} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name="maxOutputTokens"
                render={({ field }) => (
                  <FormItem className="flex-1">
                    <FormLabel>{t("ai:models.maxOutputTokens")}</FormLabel>
                    <FormControl>
                      <Input type="number" {...field} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>

            <div className="flex items-center gap-2">
              <FormField
                control={form.control}
                name="inputPrice"
                render={({ field }) => (
                  <FormItem className="flex-1">
                    <FormLabel>{t("ai:models.inputPrice")}</FormLabel>
                    <FormControl>
                      <Input type="number" step="0.01" placeholder={t("ai:models.inputPricePlaceholder")} {...field} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name="outputPrice"
                render={({ field }) => (
                  <FormItem className="flex-1">
                    <FormLabel>{t("ai:models.outputPrice")}</FormLabel>
                    <FormControl>
                      <Input type="number" step="0.01" placeholder={t("ai:models.outputPricePlaceholder")} {...field} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>

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

import { useEffect, useMemo } from "react"
import { useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { z } from "zod"
import { zodResolver } from "@hookform/resolvers/zod"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { api, type PaginatedResponse } from "@/lib/api"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Textarea } from "@/components/ui/textarea"
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
import type { KnowledgeBaseItem } from "../index"

interface ProviderOption {
  id: number
  name: string
}

interface ModelOption {
  id: number
  displayName: string
  modelId: string
  providerId: number
}

function useKnowledgeBaseSchema() {
  const { t } = useTranslation("ai")
  return z.object({
    name: z.string().min(1, t("validation.nameRequired")).max(128),
    description: z.string().max(512).optional(),
    compileMethod: z.string(),
    providerId: z.string().optional(),
    compileModelId: z.coerce.number().optional(),
    embeddingProviderId: z.string().optional(),
    embeddingModelId: z.string().optional(),
    autoCompile: z.boolean(),
    targetContentLength: z.coerce.number().min(100).max(50000).optional(),
    minContentLength: z.coerce.number().min(50).max(10000).optional(),
  })
}

type FormValues = z.infer<ReturnType<typeof useKnowledgeBaseSchema>>

interface KnowledgeBaseFormProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  knowledgeBase: KnowledgeBaseItem | null
}

export function KnowledgeBaseForm({ open, onOpenChange, knowledgeBase }: KnowledgeBaseFormProps) {
  const { t } = useTranslation(["ai", "common"])
  const queryClient = useQueryClient()
  const isEditing = knowledgeBase !== null
  const schema = useKnowledgeBaseSchema()

  // Fetch providers
  const { data: providersData } = useQuery({
    queryKey: ["ai-providers"],
    queryFn: () => api.get<PaginatedResponse<ProviderOption>>("/api/v1/ai/providers?pageSize=100"),
    enabled: open,
  })
  const providers = providersData?.items ?? []

  // For edit mode: resolve the provider from the selected compile model
  const { data: editModelDetail } = useQuery({
    queryKey: ["ai-model-detail", knowledgeBase?.compileModelId],
    queryFn: () =>
      api.get<ModelOption>(`/api/v1/ai/models/${knowledgeBase!.compileModelId}`),
    enabled: open && isEditing && knowledgeBase?.compileModelId != null,
  })

  // Derive the initial provider ID for edit mode
  const editProviderId = editModelDetail?.providerId ? String(editModelDetail.providerId) : ""

  const form = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: {
      name: "",
      description: "",
      compileMethod: "knowledge_graph",
      providerId: "",
      compileModelId: undefined,
      embeddingProviderId: "",
      embeddingModelId: "",
      autoCompile: false,
      targetContentLength: 4000,
      minContentLength: 200,
    },
  })

  const selectedProviderId = form.watch("providerId") ?? ""
  const selectedEmbeddingProviderId = form.watch("embeddingProviderId") ?? ""

  // Fetch LLM models filtered by selected provider
  const { data: modelsData } = useQuery({
    queryKey: ["ai-models-llm", selectedProviderId],
    queryFn: () =>
      api.get<PaginatedResponse<ModelOption>>(
        `/api/v1/ai/models?type=llm&providerId=${selectedProviderId}&pageSize=100`,
      ),
    enabled: open && selectedProviderId !== "",
  })
  const llmModels = modelsData?.items ?? []

  // Fetch embedding models filtered by selected embedding provider
  const { data: embModelsData } = useQuery({
    queryKey: ["ai-models-embedding", selectedEmbeddingProviderId],
    queryFn: () =>
      api.get<PaginatedResponse<ModelOption>>(
        `/api/v1/ai/models?type=embed&providerId=${selectedEmbeddingProviderId}&pageSize=100`,
      ),
    enabled: open && selectedEmbeddingProviderId !== "",
  })
  const embeddingModels = embModelsData?.items ?? []

  // Compute default form values based on mode
  const resetValues = useMemo(() => {
    if (knowledgeBase) {
      return {
        name: knowledgeBase.name,
        description: knowledgeBase.description ?? "",
        compileMethod: knowledgeBase.compileMethod || "knowledge_graph",
        providerId: editProviderId,
        compileModelId: knowledgeBase.compileModelId || undefined,
        embeddingProviderId: knowledgeBase.embeddingProviderId ? String(knowledgeBase.embeddingProviderId) : "",
        embeddingModelId: knowledgeBase.embeddingModelId || "",
        autoCompile: knowledgeBase.autoCompile,
        targetContentLength: knowledgeBase.compileConfig?.targetContentLength ?? 4000,
        minContentLength: knowledgeBase.compileConfig?.minContentLength ?? 200,
      }
    }
    return {
      name: "",
      description: "",
      compileMethod: "knowledge_graph",
      providerId: "",
      compileModelId: undefined as number | undefined,
      embeddingProviderId: "",
      embeddingModelId: "",
      autoCompile: false,
      targetContentLength: 4000,
      minContentLength: 200,
    }
  }, [knowledgeBase, editProviderId])

  useEffect(() => {
    if (open) {
      form.reset(resetValues)
    }
  }, [open, resetValues, form])

  function handleProviderChange(value: string) {
    form.setValue("providerId", value)
    form.setValue("compileModelId", undefined)
  }

  function handleEmbeddingProviderChange(value: string) {
    form.setValue("embeddingProviderId", value)
    form.setValue("embeddingModelId", "")
  }

  const createMutation = useMutation({
    mutationFn: (values: FormValues) => {
      const { name, description, compileMethod, compileModelId, autoCompile, embeddingProviderId, embeddingModelId, targetContentLength, minContentLength } = values
      return api.post("/api/v1/ai/knowledge-bases", {
        name, description, compileMethod, compileModelId, autoCompile,
        embeddingProviderId: embeddingProviderId ? Number(embeddingProviderId) : null,
        embeddingModelId: embeddingModelId || "",
        compileConfig: { targetContentLength: targetContentLength ?? 4000, minContentLength: minContentLength ?? 200, maxChunkSize: 0 },
      })
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["ai-knowledge-bases"] })
      onOpenChange(false)
      toast.success(t("ai:knowledge.createSuccess"))
    },
    onError: (err) => toast.error(err.message),
  })

  const updateMutation = useMutation({
    mutationFn: (values: FormValues) => {
      const { name, description, compileMethod, compileModelId, autoCompile, embeddingProviderId, embeddingModelId, targetContentLength, minContentLength } = values
      return api.put(`/api/v1/ai/knowledge-bases/${knowledgeBase!.id}`, {
        name, description, compileMethod, compileModelId, autoCompile,
        embeddingProviderId: embeddingProviderId ? Number(embeddingProviderId) : null,
        embeddingModelId: embeddingModelId || "",
        compileConfig: { targetContentLength: targetContentLength ?? 4000, minContentLength: minContentLength ?? 200, maxChunkSize: 0 },
      })
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["ai-knowledge-bases"] })
      onOpenChange(false)
      toast.success(t("ai:knowledge.updateSuccess"))
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
            {isEditing ? t("ai:knowledge.edit") : t("ai:knowledge.create")}
          </SheetTitle>
          <SheetDescription className="sr-only">
            {isEditing ? t("ai:knowledge.edit") : t("ai:knowledge.create")}
          </SheetDescription>
        </SheetHeader>
        <Form {...form}>
          <form onSubmit={form.handleSubmit(onSubmit)} className="flex flex-1 flex-col gap-5 px-4">
            <FormField
              control={form.control}
              name="name"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t("ai:knowledge.name")}</FormLabel>
                  <FormControl>
                    <Input placeholder={t("ai:knowledge.namePlaceholder")} {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name="description"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t("ai:knowledge.description")}</FormLabel>
                  <FormControl>
                    <Textarea
                      placeholder={t("ai:knowledge.descriptionPlaceholder")}
                      className="resize-none"
                      rows={3}
                      {...field}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            {/* Compile Method */}
            <FormField
              control={form.control}
              name="compileMethod"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t("ai:knowledge.compileMethod")}</FormLabel>
                  <Select value={field.value} onValueChange={field.onChange}>
                    <FormControl>
                      <SelectTrigger>
                        <SelectValue placeholder={t("ai:knowledge.selectCompileMethod")} />
                      </SelectTrigger>
                    </FormControl>
                    <SelectContent>
                      <SelectItem value="knowledge_graph">
                        {t("ai:knowledge.compileMethods.knowledge_graph")}
                      </SelectItem>
                    </SelectContent>
                  </Select>
                  <FormMessage />
                </FormItem>
              )}
            />
            {/* Compile Provider (cascade step 1) */}
            <FormField
              control={form.control}
              name="providerId"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t("ai:knowledge.compileProvider")}</FormLabel>
                  <Select value={field.value ?? ""} onValueChange={handleProviderChange}>
                    <FormControl>
                      <SelectTrigger>
                        <SelectValue placeholder={t("ai:knowledge.selectCompileProvider")} />
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
                </FormItem>
              )}
            />
            {/* Compile Model (cascade step 2) */}
            <FormField
              control={form.control}
              name="compileModelId"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t("ai:knowledge.compileModel")}</FormLabel>
                  <Select
                    value={field.value ? String(field.value) : ""}
                    onValueChange={(v) => field.onChange(v ? Number(v) : undefined)}
                    disabled={selectedProviderId === ""}
                  >
                    <FormControl>
                      <SelectTrigger>
                        <SelectValue placeholder={t("ai:knowledge.selectCompileModel")} />
                      </SelectTrigger>
                    </FormControl>
                    <SelectContent>
                      {llmModels.map((m) => (
                        <SelectItem key={m.id} value={String(m.id)}>
                          {m.displayName}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                  <FormMessage />
                </FormItem>
              )}
            />
            {/* Embedding Provider (cascade step 1) */}
            <FormField
              control={form.control}
              name="embeddingProviderId"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t("ai:knowledge.embeddingProvider")}</FormLabel>
                  <Select value={field.value ?? ""} onValueChange={handleEmbeddingProviderChange}>
                    <FormControl>
                      <SelectTrigger>
                        <SelectValue placeholder={t("ai:knowledge.selectEmbeddingProvider")} />
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
                </FormItem>
              )}
            />
            {/* Embedding Model (cascade step 2) */}
            <FormField
              control={form.control}
              name="embeddingModelId"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t("ai:knowledge.embeddingModel")}</FormLabel>
                  <Select
                    value={field.value ?? ""}
                    onValueChange={field.onChange}
                    disabled={selectedEmbeddingProviderId === ""}
                  >
                    <FormControl>
                      <SelectTrigger>
                        <SelectValue placeholder={t("ai:knowledge.selectEmbeddingModel")} />
                      </SelectTrigger>
                    </FormControl>
                    <SelectContent>
                      {embeddingModels.map((m) => (
                        <SelectItem key={m.id} value={m.modelId}>
                          {m.displayName}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                  <FormMessage />
                </FormItem>
              )}
            />
            {/* Compile Config */}
            <div className="space-y-3 rounded-lg border p-3">
              <p className="text-sm font-medium">{t("ai:knowledge.compileConfig.title")}</p>
              <FormField
                control={form.control}
                name="targetContentLength"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t("ai:knowledge.compileConfig.targetContentLength")}</FormLabel>
                    <FormControl>
                      <Input type="number" min={100} max={50000} {...field} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name="minContentLength"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t("ai:knowledge.compileConfig.minContentLength")}</FormLabel>
                    <FormControl>
                      <Input type="number" min={50} max={10000} {...field} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>
            <FormField
              control={form.control}
              name="autoCompile"
              render={({ field }) => (
                <FormItem className="flex items-center justify-between rounded-lg border p-3">
                  <div className="space-y-0.5">
                    <FormLabel>{t("ai:knowledge.autoCompile")}</FormLabel>
                  </div>
                  <FormControl>
                    <Switch
                      checked={field.value}
                      onCheckedChange={field.onChange}
                    />
                  </FormControl>
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

import { useEffect, useMemo } from "react"
import { useForm, useWatch } from "react-hook-form"
import { z } from "zod"
import { zodResolver } from "@/lib/form"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { api, type PaginatedResponse } from "@/lib/api"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Textarea } from "@/components/ui/textarea"
import { Switch } from "@/components/ui/switch"
import {
  Sheet, SheetContent, SheetHeader, SheetTitle, SheetDescription, SheetFooter,
} from "@/components/ui/sheet"
import {
  Form, FormControl, FormField, FormItem, FormLabel, FormMessage,
} from "@/components/ui/form"
import {
  Select, SelectContent, SelectItem, SelectTrigger, SelectValue,
} from "@/components/ui/select"
import { cn } from "@/lib/utils"
import type { KnowledgeAsset, KnowledgeType } from "../../_shared/types"

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

const schema = z.object({
  name: z.string().min(1, "名称不能为空").max(128),
  description: z.string().max(512).optional(),
  type: z.string().min(1, "请选择类型"),
  compileProviderId: z.string().optional(),
  compileModelId: z.coerce.number().optional(),
  embeddingProviderId: z.string().optional(),
  embeddingModelId: z.string().optional(),
  autoBuild: z.boolean(),
  targetContentLength: z.coerce.number().min(100).max(50000).optional(),
  minContentLength: z.coerce.number().min(50).max(10000).optional(),
})

type FormValues = z.infer<typeof schema>

interface KgFormSheetProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  knowledgeGraph: KnowledgeAsset | null
}

export function KgFormSheet({ open, onOpenChange, knowledgeGraph }: KgFormSheetProps) {
  const queryClient = useQueryClient()
  const isEditing = knowledgeGraph !== null

  const { data: kgTypes = [] } = useQuery({
    queryKey: ["ai-knowledge-types-kg"],
    queryFn: () => api.get<KnowledgeType[]>("/api/v1/ai/knowledge/types?category=kg"),
    enabled: open,
  })

  const { data: providers = [] } = useQuery({
    queryKey: ["ai-providers"],
    queryFn: () =>
      api.get<PaginatedResponse<ProviderOption>>("/api/v1/ai/providers?pageSize=100").then((r) => r?.items ?? []),
    enabled: open,
  })

  const form = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: {
      name: "",
      description: "",
      type: "",
      compileProviderId: "",
      compileModelId: undefined,
      embeddingProviderId: "",
      embeddingModelId: "",
      autoBuild: false,
      targetContentLength: 4000,
      minContentLength: 200,
    },
  })

  const selectedCompileProviderId = useWatch({ control: form.control, name: "compileProviderId" }) ?? ""
  const selectedEmbeddingProviderId = useWatch({ control: form.control, name: "embeddingProviderId" }) ?? ""

  // Fetch LLM models for compile
  const { data: llmModelsData } = useQuery({
    queryKey: ["ai-models-llm", selectedCompileProviderId],
    queryFn: () =>
      api.get<PaginatedResponse<ModelOption>>(
        `/api/v1/ai/models?type=llm&providerId=${selectedCompileProviderId}&pageSize=100`,
      ),
    enabled: open && selectedCompileProviderId !== "",
  })
  const llmModels = llmModelsData?.items ?? []

  // For edit: resolve compile provider
  const { data: editModelDetail } = useQuery({
    queryKey: ["ai-model-detail", knowledgeGraph?.compileModelId],
    queryFn: () => api.get<ModelOption>(`/api/v1/ai/models/${knowledgeGraph!.compileModelId}`),
    enabled: open && isEditing && knowledgeGraph?.compileModelId != null,
  })
  const editCompileProviderId = editModelDetail?.providerId ? String(editModelDetail.providerId) : ""

  // Fetch embedding models
  const { data: embModelsData } = useQuery({
    queryKey: ["ai-models-embedding", selectedEmbeddingProviderId],
    queryFn: () =>
      api.get<PaginatedResponse<ModelOption>>(
        `/api/v1/ai/models?type=embed&providerId=${selectedEmbeddingProviderId}&pageSize=100`,
      ),
    enabled: open && selectedEmbeddingProviderId !== "",
  })
  const embeddingModels = embModelsData?.items ?? []

  const resetValues = useMemo(() => {
    if (knowledgeGraph) {
      return {
        name: knowledgeGraph.name,
        description: knowledgeGraph.description ?? "",
        type: knowledgeGraph.type,
        compileProviderId: editCompileProviderId,
        compileModelId: knowledgeGraph.compileModelId || undefined,
        embeddingProviderId: knowledgeGraph.embeddingProviderId ? String(knowledgeGraph.embeddingProviderId) : "",
        embeddingModelId: knowledgeGraph.embeddingModelId || "",
        autoBuild: knowledgeGraph.autoBuild,
        targetContentLength: (knowledgeGraph.config?.targetContentLength as number) ?? 4000,
        minContentLength: (knowledgeGraph.config?.minContentLength as number) ?? 200,
      }
    }
    return {
      name: "",
      description: "",
      type: kgTypes[0]?.type ?? "",
      compileProviderId: "",
      compileModelId: undefined as number | undefined,
      embeddingProviderId: "",
      embeddingModelId: "",
      autoBuild: false,
      targetContentLength: 4000,
      minContentLength: 200,
    }
  }, [knowledgeGraph, editCompileProviderId, kgTypes])

  useEffect(() => {
    if (open) form.reset(resetValues)
  }, [open, resetValues, form])

  function handleCompileProviderChange(value: string) {
    form.setValue("compileProviderId", value)
    form.setValue("compileModelId", undefined)
  }

  function handleEmbeddingProviderChange(value: string) {
    form.setValue("embeddingProviderId", value)
    form.setValue("embeddingModelId", "")
  }

  const createMutation = useMutation({
    mutationFn: (values: FormValues) =>
      api.post("/api/v1/ai/knowledge/graphs", {
        name: values.name,
        description: values.description,
        type: values.type,
        config: {
          targetContentLength: values.targetContentLength ?? 4000,
          minContentLength: values.minContentLength ?? 200,
        },
        compileModelId: values.compileModelId,
        embeddingProviderId: values.embeddingProviderId ? Number(values.embeddingProviderId) : null,
        embeddingModelId: values.embeddingModelId || "",
        autoBuild: values.autoBuild,
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["ai-kg-list"] })
      onOpenChange(false)
      toast.success("知识图谱创建成功")
    },
    onError: (err) => toast.error(err.message),
  })

  const updateMutation = useMutation({
    mutationFn: (values: FormValues) =>
      api.put(`/api/v1/ai/knowledge/graphs/${knowledgeGraph!.id}`, {
        name: values.name,
        description: values.description,
        type: values.type,
        config: {
          targetContentLength: values.targetContentLength ?? 4000,
          minContentLength: values.minContentLength ?? 200,
        },
        compileModelId: values.compileModelId,
        embeddingProviderId: values.embeddingProviderId ? Number(values.embeddingProviderId) : null,
        embeddingModelId: values.embeddingModelId || "",
        autoBuild: values.autoBuild,
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["ai-kg-list"] })
      onOpenChange(false)
      toast.success("知识图谱更新成功")
    },
    onError: (err) => toast.error(err.message),
  })

  function onSubmit(values: FormValues) {
    if (isEditing) updateMutation.mutate(values)
    else createMutation.mutate(values)
  }

  const isPending = createMutation.isPending || updateMutation.isPending

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className="sm:max-w-lg overflow-y-auto">
        <SheetHeader>
          <SheetTitle>{isEditing ? "编辑知识图谱" : "新建知识图谱"}</SheetTitle>
          <SheetDescription className="sr-only">
            {isEditing ? "编辑知识图谱" : "新建知识图谱"}
          </SheetDescription>
        </SheetHeader>
        <Form {...form}>
          <form onSubmit={form.handleSubmit(onSubmit)} className="flex flex-1 flex-col gap-5 px-4">
            <FormField
              control={form.control}
              name="name"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>名称</FormLabel>
                  <FormControl>
                    <Input placeholder="输入知识图谱名称" {...field} />
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
                  <FormLabel>描述</FormLabel>
                  <FormControl>
                    <Textarea placeholder="描述知识图谱用途" className="resize-none" rows={3} {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            {/* Type selector */}
            <FormField
              control={form.control}
              name="type"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>类型</FormLabel>
                  <div className="grid gap-2">
                    {kgTypes.map((kt) => (
                      <button
                        key={kt.type}
                        type="button"
                        disabled={isEditing}
                        onClick={() => field.onChange(kt.type)}
                        className={cn(
                          "rounded-lg border p-3 text-left transition-colors",
                          field.value === kt.type
                            ? "border-primary bg-primary/5"
                            : "hover:bg-muted/50",
                          isEditing && "opacity-60 cursor-not-allowed",
                        )}
                      >
                        <p className="text-sm font-medium">{kt.displayName}</p>
                        <p className="text-xs text-muted-foreground mt-0.5">{kt.description}</p>
                      </button>
                    ))}
                  </div>
                  <FormMessage />
                </FormItem>
              )}
            />

            {/* Compile Provider */}
            <FormField
              control={form.control}
              name="compileProviderId"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>编译供应商</FormLabel>
                  <Select value={field.value ?? ""} onValueChange={handleCompileProviderChange}>
                    <FormControl>
                      <SelectTrigger>
                        <SelectValue placeholder="选择供应商" />
                      </SelectTrigger>
                    </FormControl>
                    <SelectContent>
                      {providers.map((p) => (
                        <SelectItem key={p.id} value={String(p.id)}>{p.name}</SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </FormItem>
              )}
            />

            {/* Compile Model */}
            <FormField
              control={form.control}
              name="compileModelId"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>编译模型</FormLabel>
                  <Select
                    value={field.value ? String(field.value) : ""}
                    onValueChange={(v) => field.onChange(v ? Number(v) : undefined)}
                    disabled={selectedCompileProviderId === ""}
                  >
                    <FormControl>
                      <SelectTrigger>
                        <SelectValue placeholder="选择模型" />
                      </SelectTrigger>
                    </FormControl>
                    <SelectContent>
                      {llmModels.map((m) => (
                        <SelectItem key={m.id} value={String(m.id)}>{m.displayName}</SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                  <FormMessage />
                </FormItem>
              )}
            />

            {/* Embedding Provider */}
            <FormField
              control={form.control}
              name="embeddingProviderId"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>向量化供应商</FormLabel>
                  <Select value={field.value ?? ""} onValueChange={handleEmbeddingProviderChange}>
                    <FormControl>
                      <SelectTrigger>
                        <SelectValue placeholder="选择供应商" />
                      </SelectTrigger>
                    </FormControl>
                    <SelectContent>
                      {providers.map((p) => (
                        <SelectItem key={p.id} value={String(p.id)}>{p.name}</SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </FormItem>
              )}
            />

            {/* Embedding Model */}
            <FormField
              control={form.control}
              name="embeddingModelId"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>向量化模型</FormLabel>
                  <Select
                    value={field.value ?? ""}
                    onValueChange={field.onChange}
                    disabled={selectedEmbeddingProviderId === ""}
                  >
                    <FormControl>
                      <SelectTrigger>
                        <SelectValue placeholder="选择模型" />
                      </SelectTrigger>
                    </FormControl>
                    <SelectContent>
                      {embeddingModels.map((m) => (
                        <SelectItem key={m.id} value={m.modelId}>{m.displayName}</SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                  <FormMessage />
                </FormItem>
              )}
            />

            {/* Config */}
            <div className="space-y-3 rounded-lg border p-3">
              <p className="text-sm font-medium">图谱配置</p>
              <FormField
                control={form.control}
                name="targetContentLength"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>目标内容长度</FormLabel>
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
                    <FormLabel>最小内容长度</FormLabel>
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
              name="autoBuild"
              render={({ field }) => (
                <FormItem className="flex items-center justify-between rounded-lg border p-3">
                  <div className="space-y-0.5">
                    <FormLabel>自动构建</FormLabel>
                  </div>
                  <FormControl>
                    <Switch checked={field.value} onCheckedChange={field.onChange} />
                  </FormControl>
                </FormItem>
              )}
            />

            <SheetFooter>
              <Button type="submit" size="sm" disabled={isPending}>
                {isPending ? "保存中..." : isEditing ? "保存" : "创建"}
              </Button>
            </SheetFooter>
          </form>
        </Form>
      </SheetContent>
    </Sheet>
  )
}

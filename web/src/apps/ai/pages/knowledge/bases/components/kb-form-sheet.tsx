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
  embeddingProviderId: z.string().optional(),
  embeddingModelId: z.string().optional(),
  autoBuild: z.boolean(),
  chunkSize: z.coerce.number().min(100).max(10000).optional(),
  chunkOverlap: z.coerce.number().min(0).max(2000).optional(),
})

type FormValues = z.infer<typeof schema>

interface KbFormSheetProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  knowledgeBase: KnowledgeAsset | null
}

export function KbFormSheet({ open, onOpenChange, knowledgeBase }: KbFormSheetProps) {
  const queryClient = useQueryClient()
  const isEditing = knowledgeBase !== null

  const { data: kbTypes = [] } = useQuery({
    queryKey: ["ai-knowledge-types-kb"],
    queryFn: () => api.get<KnowledgeType[]>("/api/v1/ai/knowledge/types?category=kb"),
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
      embeddingProviderId: "",
      embeddingModelId: "",
      autoBuild: false,
      chunkSize: 1000,
      chunkOverlap: 200,
    },
  })

  const selectedEmbeddingProviderId = useWatch({ control: form.control, name: "embeddingProviderId" }) ?? ""
  const selectedType = useWatch({ control: form.control, name: "type" })

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
    if (knowledgeBase) {
      return {
        name: knowledgeBase.name,
        description: knowledgeBase.description ?? "",
        type: knowledgeBase.type,
        embeddingProviderId: knowledgeBase.embeddingProviderId ? String(knowledgeBase.embeddingProviderId) : "",
        embeddingModelId: knowledgeBase.embeddingModelId || "",
        autoBuild: knowledgeBase.autoBuild,
        chunkSize: (knowledgeBase.config?.chunkSize as number) ?? 1000,
        chunkOverlap: (knowledgeBase.config?.chunkOverlap as number) ?? 200,
      }
    }
    return {
      name: "",
      description: "",
      type: kbTypes[0]?.type ?? "",
      embeddingProviderId: "",
      embeddingModelId: "",
      autoBuild: false,
      chunkSize: 1000,
      chunkOverlap: 200,
    }
  }, [knowledgeBase, kbTypes])

  useEffect(() => {
    if (open) form.reset(resetValues)
  }, [open, resetValues, form])

  function handleEmbeddingProviderChange(value: string) {
    form.setValue("embeddingProviderId", value)
    form.setValue("embeddingModelId", "")
  }

  const createMutation = useMutation({
    mutationFn: (values: FormValues) =>
      api.post("/api/v1/ai/knowledge/bases", {
        name: values.name,
        description: values.description,
        type: values.type,
        config: { chunkSize: values.chunkSize ?? 1000, chunkOverlap: values.chunkOverlap ?? 200 },
        embeddingProviderId: values.embeddingProviderId ? Number(values.embeddingProviderId) : null,
        embeddingModelId: values.embeddingModelId || "",
        autoBuild: values.autoBuild,
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["ai-kb-list"] })
      onOpenChange(false)
      toast.success("知识库创建成功")
    },
    onError: (err) => toast.error(err.message),
  })

  const updateMutation = useMutation({
    mutationFn: (values: FormValues) =>
      api.put(`/api/v1/ai/knowledge/bases/${knowledgeBase!.id}`, {
        name: values.name,
        description: values.description,
        type: values.type,
        config: { chunkSize: values.chunkSize ?? 1000, chunkOverlap: values.chunkOverlap ?? 200 },
        embeddingProviderId: values.embeddingProviderId ? Number(values.embeddingProviderId) : null,
        embeddingModelId: values.embeddingModelId || "",
        autoBuild: values.autoBuild,
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["ai-kb-list"] })
      onOpenChange(false)
      toast.success("知识库更新成功")
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
          <SheetTitle>{isEditing ? "编辑知识库" : "新建知识库"}</SheetTitle>
          <SheetDescription className="sr-only">
            {isEditing ? "编辑知识库" : "新建知识库"}
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
                    <Input placeholder="输入知识库名称" {...field} />
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
                    <Textarea placeholder="描述知识库用途" className="resize-none" rows={3} {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            {/* Type selector - card style radio */}
            <FormField
              control={form.control}
              name="type"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>类型</FormLabel>
                  <div className="grid gap-2">
                    {kbTypes.map((kt) => (
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

            {/* Config fields for naive_chunk type */}
            {selectedType === "naive_chunk" && (
              <div className="space-y-3 rounded-lg border p-3">
                <p className="text-sm font-medium">分块配置</p>
                <FormField
                  control={form.control}
                  name="chunkSize"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>分块大小</FormLabel>
                      <FormControl>
                        <Input type="number" min={100} max={10000} {...field} />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name="chunkOverlap"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>分块重叠</FormLabel>
                      <FormControl>
                        <Input type="number" min={0} max={2000} {...field} />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </div>
            )}

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

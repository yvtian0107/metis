import { useEffect } from "react"
import { useForm, useWatch } from "react-hook-form"
import { z } from "zod"
import { zodResolver } from "@/lib/form"
import { useMutation, useQueryClient } from "@tanstack/react-query"
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

const schema = z.object({
  sourceUrl: z.string().url("请输入有效的网址"),
  title: z.string().max(256).optional(),
  crawlDepth: z.coerce.number().min(0).max(3),
  urlPattern: z.string().max(512).optional(),
  crawlEnabled: z.boolean(),
  crawlSchedule: z.string().max(128).optional(),
})

type FormValues = z.infer<typeof schema>

interface SourceUrlSheetProps {
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function SourceUrlSheet({ open, onOpenChange }: SourceUrlSheetProps) {
  const queryClient = useQueryClient()

  const form = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: {
      sourceUrl: "",
      title: "",
      crawlDepth: 0,
      urlPattern: "",
      crawlEnabled: false,
      crawlSchedule: "",
    },
  })
  const crawlEnabled = useWatch({ control: form.control, name: "crawlEnabled" })

  useEffect(() => {
    if (open) {
      form.reset()
    }
  }, [open, form])

  const mutation = useMutation({
    mutationFn: (values: FormValues) =>
      api.post("/api/v1/ai/knowledge/sources/url", {
        title: values.title || undefined,
        sourceUrl: values.sourceUrl,
        crawlDepth: values.crawlDepth,
        urlPattern: values.urlPattern || undefined,
        crawlEnabled: values.crawlEnabled,
        crawlSchedule: values.crawlSchedule || undefined,
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["ai-knowledge-sources"] })
      toast.success("网址素材添加成功")
      onOpenChange(false)
    },
    onError: (err) => toast.error(err.message),
  })

  function onSubmit(values: FormValues) {
    mutation.mutate(values)
  }

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className="sm:max-w-lg overflow-y-auto">
        <SheetHeader>
          <SheetTitle>添加网址</SheetTitle>
          <SheetDescription className="sr-only">添加网址</SheetDescription>
        </SheetHeader>
        <Form {...form}>
          <form onSubmit={form.handleSubmit(onSubmit)} className="flex flex-1 flex-col gap-5 px-4">
            <FormField
              control={form.control}
              name="sourceUrl"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>网址 *</FormLabel>
                  <FormControl>
                    <Input placeholder="https://example.com" {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name="title"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>标题（可选）</FormLabel>
                  <FormControl>
                    <Input placeholder="输入素材标题" {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name="crawlDepth"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>爬取深度</FormLabel>
                  <Select value={String(field.value)} onValueChange={(v) => field.onChange(Number(v))}>
                    <FormControl>
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                    </FormControl>
                    <SelectContent>
                      <SelectItem value="0">0 — 仅当前页面</SelectItem>
                      <SelectItem value="1">1 — 包含一级链接</SelectItem>
                      <SelectItem value="2">2 — 包含二级链接</SelectItem>
                      <SelectItem value="3">3 — 包含三级链接</SelectItem>
                    </SelectContent>
                  </Select>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name="urlPattern"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>URL 匹配模式（可选）</FormLabel>
                  <FormControl>
                    <Input placeholder="例如: /docs/*" {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name="crawlEnabled"
              render={({ field }) => (
                <FormItem className="flex items-center justify-between rounded-lg border p-3">
                  <div className="space-y-0.5">
                    <FormLabel>启用定时爬取</FormLabel>
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
            {crawlEnabled && (
              <FormField
                control={form.control}
                name="crawlSchedule"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>爬取计划（Cron 表达式）</FormLabel>
                    <FormControl>
                      <Input placeholder="0 0 * * *" {...field} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            )}
            <SheetFooter>
              <Button type="submit" size="sm" disabled={mutation.isPending}>
                {mutation.isPending ? "提交中..." : "添加"}
              </Button>
            </SheetFooter>
          </form>
        </Form>
      </SheetContent>
    </Sheet>
  )
}

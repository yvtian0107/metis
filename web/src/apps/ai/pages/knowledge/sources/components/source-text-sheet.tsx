import { useEffect } from "react"
import { useForm } from "react-hook-form"
import { z } from "zod"
import { zodResolver } from "@/lib/form"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { api } from "@/lib/api"
import { toast } from "sonner"
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

const schema = z.object({
  title: z.string().min(1, "请输入标题").max(256),
  content: z.string().min(1, "请输入内容"),
})

type FormValues = z.infer<typeof schema>

interface SourceTextSheetProps {
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function SourceTextSheet({ open, onOpenChange }: SourceTextSheetProps) {
  const queryClient = useQueryClient()

  const form = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: {
      title: "",
      content: "",
    },
  })

  useEffect(() => {
    if (open) {
      form.reset()
    }
  }, [open, form])

  const mutation = useMutation({
    mutationFn: (values: FormValues) =>
      api.post("/api/v1/ai/knowledge/sources/text", values),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["ai-knowledge-sources"] })
      toast.success("文本素材添加成功")
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
          <SheetTitle>添加文本</SheetTitle>
          <SheetDescription className="sr-only">添加文本</SheetDescription>
        </SheetHeader>
        <Form {...form}>
          <form onSubmit={form.handleSubmit(onSubmit)} className="flex flex-1 flex-col gap-5 px-4">
            <FormField
              control={form.control}
              name="title"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>标题 *</FormLabel>
                  <FormControl>
                    <Input placeholder="输入素材标题" {...field} />
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
                  <FormLabel>内容 *</FormLabel>
                  <FormControl>
                    <Textarea
                      placeholder="输入文本内容..."
                      className="resize-none"
                      rows={12}
                      {...field}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
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

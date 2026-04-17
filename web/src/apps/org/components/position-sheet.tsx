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

export interface PositionItem {
  id: number
  name: string
  code: string
  description: string
  isActive: boolean
  createdAt: string
  updatedAt: string
}

function usePositionSchema() {
  const { t } = useTranslation("org")
  return z.object({
    name: z.string().min(1, t("validation.nameRequired")),
    code: z.string().min(1, t("validation.codeRequired")),
    description: z.string().optional(),
  })
}

type FormValues = z.infer<ReturnType<typeof usePositionSchema>>

interface PositionSheetProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  position: PositionItem | null
}

export function PositionSheet({ open, onOpenChange, position }: PositionSheetProps) {
  const { t } = useTranslation(["org", "common"])
  const queryClient = useQueryClient()
  const isEditing = position !== null
  const schema = usePositionSchema()

  const form = useForm<FormValues>({
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    resolver: zodResolver(schema as any),
    defaultValues: { name: "", code: "", description: "" },
  })

  useEffect(() => {
    if (open) {
      if (position) {
        form.reset({
          name: position.name,
          code: position.code,
          description: position.description,
        })
      } else {
        form.reset({ name: "", code: "", description: "" })
      }
    }
  }, [open, position, form])

  const createMutation = useMutation({
    mutationFn: (values: FormValues) =>
      api.post("/api/v1/org/positions", {
        name: values.name,
        code: values.code,
        description: values.description,
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["positions"] })
      onOpenChange(false)
      toast.success(t("org:positions.createSuccess"))
    },
    onError: (err) => toast.error(err.message),
  })

  const updateMutation = useMutation({
    mutationFn: (values: FormValues) =>
      api.put(`/api/v1/org/positions/${position!.id}`, {
        name: values.name,
        code: values.code,
        description: values.description,
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["positions"] })
      onOpenChange(false)
      toast.success(t("org:positions.updateSuccess"))
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
      <SheetContent className="sm:max-w-lg">
        <SheetHeader>
          <SheetTitle>
            {isEditing ? t("org:positions.edit") : t("org:positions.create")}
          </SheetTitle>
          <SheetDescription className="sr-only">
            {isEditing ? t("org:positions.edit") : t("org:positions.create")}
          </SheetDescription>
        </SheetHeader>
        <Form {...form}>
          <form onSubmit={form.handleSubmit(onSubmit)} className="flex flex-1 flex-col gap-5 px-4">
            <FormField
              control={form.control}
              name="name"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t("org:positions.name")}</FormLabel>
                  <FormControl>
                    <Input placeholder={t("org:positions.namePlaceholder")} {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name="code"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t("org:positions.code")}</FormLabel>
                  <FormControl>
                    <Input placeholder={t("org:positions.codePlaceholder")} {...field} />
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
                  <FormLabel>{t("org:positions.description")}</FormLabel>
                  <FormControl>
                    <Textarea rows={3} {...field} />
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

import { useEffect } from "react"
import { useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { z } from "zod"
import { zodResolver } from "@hookform/resolvers/zod"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { Loader2 } from "lucide-react"
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

export interface ProductItem {
  id: number
  name: string
  code: string
  description: string
  status: string
  planCount: number
  createdAt: string
  updatedAt: string
}

function useProductSchema() {
  const { t } = useTranslation("license")
  return z.object({
    name: z.string().min(1, t("validation.nameRequired")).max(128),
    code: z
      .string()
      .min(1, t("validation.codeRequired"))
      .max(64)
      .regex(/^[a-z0-9-]+$/, t("validation.codeFormat")),
    description: z.string().optional(),
  })
}

type FormValues = z.infer<ReturnType<typeof useProductSchema>>

interface ProductSheetProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  product: ProductItem | null
}

export function ProductSheet({ open, onOpenChange, product }: ProductSheetProps) {
  const { t } = useTranslation(["license", "common"])
  const queryClient = useQueryClient()
  const isEditing = product !== null
  const schema = useProductSchema()

  const form = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: { name: "", code: "", description: "" },
  })

  useEffect(() => {
    if (open) {
      if (product) {
        form.reset({ name: product.name, code: product.code, description: product.description })
      } else {
        form.reset({ name: "", code: "", description: "" })
      }
    }
  }, [open, product, form])

  const createMutation = useMutation({
    mutationFn: (values: FormValues) =>
      api.post<ProductItem>("/api/v1/license/products", values),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["license-products"] })
      queryClient.invalidateQueries({ queryKey: ["license-products-published"] })
      onOpenChange(false)
      toast.success(t("license:products.createSuccess"))
    },
    onError: (err) => toast.error(err.message),
  })

  const updateMutation = useMutation({
    mutationFn: (values: FormValues) =>
      api.put(`/api/v1/license/products/${product!.id}`, {
        name: values.name,
        description: values.description,
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["license-products"] })
      queryClient.invalidateQueries({ queryKey: ["license-products-published"] })
      queryClient.invalidateQueries({ queryKey: ["license-product"] })
      onOpenChange(false)
      toast.success(t("license:products.updateSuccess"))
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
      <SheetContent className="sm:max-w-md flex flex-col">
        <SheetHeader className="pb-2">
          <SheetTitle>{isEditing ? t("license:products.editProduct") : t("license:products.create")}</SheetTitle>
          <SheetDescription className="sr-only">
            {isEditing ? t("license:products.editProduct") : t("license:products.create")}
          </SheetDescription>
        </SheetHeader>
        <Form {...form}>
          <form onSubmit={form.handleSubmit(onSubmit)} className="flex flex-1 flex-col gap-4 px-4">
            <div className="text-sm font-medium text-muted-foreground">{t("license:products.basicInfo")}</div>

            <FormField
              control={form.control}
              name="name"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t("license:products.productName")}</FormLabel>
                  <FormControl>
                    <Input placeholder={t("license:products.productNamePlaceholder")} {...field} />
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
                  <FormLabel>{t("license:products.productCode")}</FormLabel>
                  {isEditing ? (
                    <div className="rounded-md border bg-muted px-3 py-2 text-sm font-mono text-muted-foreground">
                      {field.value}
                    </div>
                  ) : (
                    <>
                      <FormControl>
                        <Input placeholder={t("license:products.productCodePlaceholder")} {...field} />
                      </FormControl>
                      <p className="text-xs text-muted-foreground">
                        {t("license:products.codeHint")}
                      </p>
                    </>
                  )}
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name="description"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t("common:description")}</FormLabel>
                  <FormControl>
                    <Textarea placeholder={t("license:products.descriptionPlaceholder")} rows={3} {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            <SheetFooter className="pt-2">
              <Button type="submit" size="sm" disabled={isPending}>
                {isPending && <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />}
                {isPending ? t("common:saving") : isEditing ? t("common:save") : t("common:create")}
              </Button>
            </SheetFooter>
          </form>
        </Form>
      </SheetContent>
    </Sheet>
  )
}

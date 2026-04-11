import { useEffect } from "react"
import { useTranslation } from "react-i18next"
import { useForm } from "react-hook-form"
import { z } from "zod"
import { zodResolver } from "@hookform/resolvers/zod"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { api } from "@/lib/api"
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
import type { Role } from "./types"

const schema = (t: (key: string) => string) => z.object({
  name: z.string().min(1, t("roles:validation.nameRequired")).max(64),
  code: z.string().min(1, t("roles:validation.codeRequired")).max(64),
  description: z.string().max(255).optional(),
})

type FormValues = z.infer<ReturnType<typeof schema>>

interface RoleSheetProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  role: Role | null
}

export function RoleSheet({ open, onOpenChange, role }: RoleSheetProps) {
  const { t } = useTranslation(["roles", "common"])
  const queryClient = useQueryClient()
  const isEditing = role !== null

  const form = useForm<FormValues>({
    resolver: zodResolver(schema(t)),
    defaultValues: { name: "", code: "", description: "" },
  })

  useEffect(() => {
    if (open) {
      if (role) {
        form.reset({
          name: role.name,
          code: role.code,
          description: role.description || "",
        })
      } else {
        form.reset({ name: "", code: "", description: "" })
      }
    }
  }, [open, role, form])

  const createMutation = useMutation({
    mutationFn: (values: FormValues) => api.post("/api/v1/roles", values),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["roles"] })
      onOpenChange(false)
    },
  })

  const updateMutation = useMutation({
    mutationFn: (values: FormValues) =>
      api.put(`/api/v1/roles/${role!.id}`, values),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["roles"] })
      onOpenChange(false)
    },
  })

  function onSubmit(values: FormValues) {
    if (isEditing) {
      updateMutation.mutate(values)
    } else {
      createMutation.mutate(values)
    }
  }

  const isPending = createMutation.isPending || updateMutation.isPending
  const error = createMutation.error || updateMutation.error

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className="sm:max-w-md">
        <SheetHeader>
          <SheetTitle>{isEditing ? t("roles:sheet.editTitle") : t("roles:sheet.createTitle")}</SheetTitle>
          <SheetDescription className="sr-only">
            {isEditing ? t("roles:sheet.editDescription") : t("roles:sheet.createDescription")}
          </SheetDescription>
        </SheetHeader>
        <Form {...form}>
          <form onSubmit={form.handleSubmit(onSubmit)} className="flex flex-1 flex-col gap-4 px-4">
            <FormField
              control={form.control}
              name="name"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t("roles:roleName")}</FormLabel>
                  <FormControl>
                    <Input placeholder={t("roles:form.namePlaceholder")} {...field} />
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
                  <FormLabel>{t("roles:roleCode")}</FormLabel>
                  <FormControl>
                    <Input
                      placeholder={t("roles:form.codePlaceholder")}
                      disabled={isEditing && role?.isSystem}
                      {...field}
                    />
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
                  <FormLabel>{t("common:description")}</FormLabel>
                  <FormControl>
                    <Input placeholder={t("roles:form.descriptionPlaceholder")} {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            {error && (
              <p className="text-sm text-destructive">{error.message}</p>
            )}

            <SheetFooter>
              <Button variant="outline" size="sm" type="button" onClick={() => onOpenChange(false)}>
                {t("common:cancel")}
              </Button>
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

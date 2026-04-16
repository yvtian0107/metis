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

export interface LicenseeItem {
  id: number
  name: string
  code: string
  notes: string
  status: string
  createdAt: string
  updatedAt: string
}

function useLicenseeSchema() {
  const { t } = useTranslation("license")
  return z.object({
    name: z.string().min(1, t("validation.nameRequired")).max(128),
    notes: z.string().default(""),
  })
}

type FormValues = z.input<ReturnType<typeof useLicenseeSchema>>

interface LicenseeSheetProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  licensee: LicenseeItem | null
}

export function LicenseeSheet({ open, onOpenChange, licensee }: LicenseeSheetProps) {
  const { t } = useTranslation(["license", "common"])
  const queryClient = useQueryClient()
  const isEditing = licensee !== null
  const schema = useLicenseeSchema()

  const form = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: {
      name: "",
      notes: "",
    },
  })

  useEffect(() => {
    if (open) {
      if (licensee) {
        form.reset({
          name: licensee.name,
          notes: licensee.notes,
        })
      } else {
        form.reset({
          name: "",
          notes: "",
        })
      }
    }
  }, [open, licensee, form])

  const createMutation = useMutation({
    mutationFn: (values: FormValues) => {
      return api.post<LicenseeItem>("/api/v1/license/licensees", values)
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["license-licensees"] })
      onOpenChange(false)
      toast.success(t("license:licensees.createSuccess"))
    },
    onError: (err) => toast.error(err.message),
  })

  const updateMutation = useMutation({
    mutationFn: (values: FormValues) => {
      return api.put(`/api/v1/license/licensees/${licensee!.id}`, values)
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["license-licensees"] })
      onOpenChange(false)
      toast.success(t("license:licensees.updateSuccess"))
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
          <SheetTitle>{isEditing ? t("license:licensees.editLicensee") : t("license:licensees.create")}</SheetTitle>
          <SheetDescription className="sr-only">
            {isEditing ? t("license:licensees.editLicensee") : t("license:licensees.create")}
          </SheetDescription>
        </SheetHeader>
        <Form {...form}>
          <form onSubmit={form.handleSubmit(onSubmit as any)} className="flex flex-1 flex-col gap-5 px-4">
            <FormField
              control={form.control as any}
              name="name"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t("license:licensees.entityName")}</FormLabel>
                  <FormControl>
                    <Input placeholder={t("license:licensees.entityNamePlaceholder")} {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control as any}
              name="notes"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t("license:licenses.notes")}</FormLabel>
                  <FormControl>
                    <Textarea placeholder={t("license:licensees.notesPlaceholder")} rows={3} {...field} />
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

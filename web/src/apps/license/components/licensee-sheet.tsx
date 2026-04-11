import { useState, useEffect } from "react"
import { useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { z } from "zod"
import { zodResolver } from "@hookform/resolvers/zod"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { api } from "@/lib/api"
import { toast } from "sonner"
import { ChevronDown } from "lucide-react"
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
  contactName: string
  contactPhone: string
  contactEmail: string
  businessInfo: {
    address?: string
    taxId?: string
    bankName?: string
    bankAccount?: string
    swift?: string
    iban?: string
  }
  notes: string
  status: string
  createdAt: string
  updatedAt: string
}

function useLicenseeSchema() {
  const { t } = useTranslation("license")
  return z.object({
    name: z.string().min(1, t("validation.nameRequired")).max(128),
    contactName: z.string().max(64).optional().default(""),
    contactPhone: z.string().max(32).optional().default(""),
    contactEmail: z.string().max(128).optional().default(""),
    notes: z.string().optional().default(""),
    address: z.string().optional().default(""),
    taxId: z.string().optional().default(""),
    bankName: z.string().optional().default(""),
    bankAccount: z.string().optional().default(""),
    swift: z.string().optional().default(""),
    iban: z.string().optional().default(""),
  })
}

type FormValues = z.infer<ReturnType<typeof useLicenseeSchema>>

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

  const bi = licensee?.businessInfo
  const hasBizData = !!(bi?.address || bi?.taxId || bi?.bankName || bi?.bankAccount || bi?.swift || bi?.iban)
  const [bizOpen, setBizOpen] = useState(hasBizData)

  const form = useForm<FormValues>({
    resolver: zodResolver(schema) as any,
    defaultValues: {
      name: "", contactName: "", contactPhone: "", contactEmail: "",
      notes: "", address: "", taxId: "", bankName: "", bankAccount: "", swift: "", iban: "",
    },
  })

  useEffect(() => {
    if (open) {
      if (licensee) {
        const bi = licensee.businessInfo ?? {}
        form.reset({
          name: licensee.name,
          contactName: licensee.contactName,
          contactPhone: licensee.contactPhone,
          contactEmail: licensee.contactEmail,
          notes: licensee.notes,
          address: bi.address ?? "",
          taxId: bi.taxId ?? "",
          bankName: bi.bankName ?? "",
          bankAccount: bi.bankAccount ?? "",
          swift: bi.swift ?? "",
          iban: bi.iban ?? "",
        })
      } else {
        form.reset({
          name: "", contactName: "", contactPhone: "", contactEmail: "",
          notes: "", address: "", taxId: "", bankName: "", bankAccount: "", swift: "", iban: "",
        })
      }
    }
  }, [open, licensee, form])

  function handleOpenChange(nextOpen: boolean) {
    if (nextOpen) {
      setBizOpen(hasBizData)
    }
    onOpenChange(nextOpen)
  }

  const createMutation = useMutation({
    mutationFn: (values: FormValues) => {
      const { address, taxId, bankName, bankAccount, swift, iban, ...rest } = values
      return api.post<LicenseeItem>("/api/v1/license/licensees", {
        ...rest,
        businessInfo: { address, taxId, bankName, bankAccount, swift, iban },
      })
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
      const { address, taxId, bankName, bankAccount, swift, iban, ...rest } = values
      return api.put(`/api/v1/license/licensees/${licensee!.id}`, {
        ...rest,
        businessInfo: { address, taxId, bankName, bankAccount, swift, iban },
      })
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
    <Sheet open={open} onOpenChange={handleOpenChange}>
      <SheetContent className="sm:max-w-lg overflow-y-auto">
        <SheetHeader>
          <SheetTitle>{isEditing ? t("license:licensees.editLicensee") : t("license:licensees.create")}</SheetTitle>
          <SheetDescription className="sr-only">
            {isEditing ? t("license:licensees.editLicensee") : t("license:licensees.create")}
          </SheetDescription>
        </SheetHeader>
        <Form {...form}>
          <form onSubmit={form.handleSubmit(onSubmit)} className="flex flex-1 flex-col gap-5 px-4">
            {/* Basic info */}
            <FormField
              control={form.control}
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
              control={form.control}
              name="notes"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t("license:licenses.notes")}</FormLabel>
                  <FormControl>
                    <Textarea placeholder={t("license:licensees.notesPlaceholder")} rows={2} {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            {/* Contact info */}
            <div className="space-y-3">
              <h4 className="text-sm font-medium text-muted-foreground">{t("license:licensees.contactInfo")}</h4>
              <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                <FormField
                  control={form.control}
                  name="contactName"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t("license:licensees.contactNameLabel")}</FormLabel>
                      <FormControl>
                        <Input placeholder={t("license:licensees.contactNamePlaceholder")} {...field} />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name="contactPhone"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t("license:licensees.phone")}</FormLabel>
                      <FormControl>
                        <Input placeholder={t("license:licensees.phonePlaceholder")} {...field} />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </div>
              <FormField
                control={form.control}
                name="contactEmail"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t("license:licensees.email")}</FormLabel>
                    <FormControl>
                      <Input placeholder="email@example.com" {...field} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>

            {/* Business info - collapsible */}
            <div className="space-y-3">
              <button
                type="button"
                className="flex w-full items-center gap-1 text-sm font-medium text-muted-foreground hover:text-foreground"
                onClick={() => setBizOpen(!bizOpen)}
              >
                <ChevronDown className={`h-4 w-4 transition-transform ${bizOpen ? "" : "-rotate-90"}`} />
                {t("license:licensees.businessInfo")}
              </button>
              {bizOpen && (
                <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                  <FormField
                    control={form.control}
                    name="address"
                    render={({ field }) => (
                      <FormItem className="sm:col-span-2">
                        <FormLabel>{t("license:licensees.address")}</FormLabel>
                        <FormControl>
                          <Input placeholder={t("license:licensees.addressPlaceholder")} {...field} />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                  <FormField
                    control={form.control}
                    name="taxId"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t("license:licensees.taxId")}</FormLabel>
                        <FormControl>
                          <Input placeholder={t("license:licensees.taxIdPlaceholder")} {...field} />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                  <FormField
                    control={form.control}
                    name="bankName"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t("license:licensees.bankName")}</FormLabel>
                        <FormControl>
                          <Input placeholder={t("license:licensees.bankNamePlaceholder")} {...field} />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                  <FormField
                    control={form.control}
                    name="bankAccount"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t("license:licensees.bankAccount")}</FormLabel>
                        <FormControl>
                          <Input placeholder={t("license:licensees.bankAccountPlaceholder")} {...field} />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                  <FormField
                    control={form.control}
                    name="swift"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>SWIFT</FormLabel>
                        <FormControl>
                          <Input placeholder={t("license:licensees.swiftPlaceholder")} {...field} />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                  <FormField
                    control={form.control}
                    name="iban"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>IBAN</FormLabel>
                        <FormControl>
                          <Input placeholder="IBAN" {...field} />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                </div>
              )}
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

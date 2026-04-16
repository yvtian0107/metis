"use client"

import { useNavigate } from "react-router"
import { useTranslation } from "react-i18next"
import { useForm } from "react-hook-form"
import { z } from "zod"
import { zodResolver } from "@hookform/resolvers/zod"
import { useQuery, useMutation } from "@tanstack/react-query"
import { ArrowLeft, Info } from "lucide-react"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Textarea } from "@/components/ui/textarea"
import {
  Select, SelectContent, SelectGroup, SelectItem, SelectLabel, SelectTrigger, SelectValue,
} from "@/components/ui/select"
import {
  Form, FormControl, FormField, FormItem, FormLabel, FormMessage,
} from "@/components/ui/form"
import {
  fetchCatalogTree, fetchSLATemplates, createServiceDef,
} from "../../../api"
import { SmartServiceConfig } from "../../../components/smart-service-config"
import { Alert, AlertDescription } from "@/components/ui/alert"

// ─── Schema ───────────────────────────────────────────

function useCreateSchema() {
  const { t } = useTranslation("itsm")
  return z.object({
    name: z.string().min(1, t("validation.nameRequired")),
    code: z.string().min(1, t("validation.codeRequired")),
    catalogId: z.number().min(1),
    engineType: z.string().default("classic"),
    slaId: z.number().nullable(),
    description: z.string().optional(),
    collaborationSpec: z.string().default(""),
    agentId: z.number().nullable().default(null),
    confidenceThreshold: z.number().default(0.8),
    decisionTimeout: z.number().default(30),
  })
}

type FormValues = z.infer<ReturnType<typeof useCreateSchema>>

export function Component() {
  const { t } = useTranslation(["itsm", "common"])
  const navigate = useNavigate()
  const schema = useCreateSchema()

  const { data: catalogs = [] } = useQuery({
    queryKey: ["itsm-catalogs"],
    queryFn: () => fetchCatalogTree(),
  })

  const { data: slaTemplates = [] } = useQuery({
    queryKey: ["itsm-sla"],
    queryFn: () => fetchSLATemplates(),
  })

  const form = useForm<FormValues>({
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    resolver: zodResolver(schema as any),
    defaultValues: {
      name: "", code: "", catalogId: 0, engineType: "classic",
      slaId: null, description: "",
      collaborationSpec: "", agentId: null,
      confidenceThreshold: 0.8, decisionTimeout: 30,
    },
  })

  const createMut = useMutation({
    mutationFn: (v: FormValues) => createServiceDef({
      ...v,
      description: v.description ?? "",
      collaborationSpec: v.engineType === "smart" ? v.collaborationSpec : undefined,
      agentId: v.engineType === "smart" ? v.agentId : undefined,
      agentConfig: v.engineType === "smart" ? JSON.stringify({
        confidence_threshold: v.confidenceThreshold,
        decision_timeout_seconds: v.decisionTimeout,
      }) : undefined,
    } as Parameters<typeof createServiceDef>[0]),
    onSuccess: (data) => {
      toast.success(t("itsm:services.createSuccess"))
      navigate(`/itsm/services/${data.id}`)
    },
    onError: (err) => toast.error(err.message),
  })

  return (
    <div className="space-y-4">
      {/* Header */}
      <div className="flex items-center gap-3">
        <Button variant="ghost" size="icon" onClick={() => navigate("/itsm/services")}>
          <ArrowLeft className="h-4 w-4" />
        </Button>
        <h2 className="text-lg font-semibold">{t("itsm:services.create")}</h2>
      </div>

      {/* Form */}
      <Form {...form}>
        <form onSubmit={form.handleSubmit((v) => createMut.mutate(v))} className="space-y-6">
          {/* Row 1: Name + Code */}
          <div className="grid grid-cols-1 gap-5 sm:grid-cols-2">
            <FormField control={form.control} name="name" render={({ field }) => (
              <FormItem>
                <FormLabel>{t("itsm:services.name")}</FormLabel>
                <FormControl><Input placeholder={t("itsm:services.namePlaceholder")} {...field} /></FormControl>
                <FormMessage />
              </FormItem>
            )} />
            <FormField control={form.control} name="code" render={({ field }) => (
              <FormItem>
                <FormLabel>{t("itsm:services.code")}</FormLabel>
                <FormControl><Input placeholder={t("itsm:services.codePlaceholder")} {...field} /></FormControl>
                <FormMessage />
              </FormItem>
            )} />
          </div>

          {/* Row 2: Catalog + SLA */}
          <div className="grid grid-cols-1 gap-5 sm:grid-cols-2">
            <FormField control={form.control} name="catalogId" render={({ field }) => (
              <FormItem>
                <FormLabel>{t("itsm:services.catalog")}</FormLabel>
                <Select onValueChange={(v) => field.onChange(Number(v))} value={String(field.value)}>
                  <FormControl><SelectTrigger><SelectValue placeholder={t("itsm:services.catalogPlaceholder")} /></SelectTrigger></FormControl>
                  <SelectContent>
                    {catalogs.map((parent) => (
                      <SelectGroup key={parent.id}>
                        <SelectLabel className="text-xs font-semibold text-muted-foreground">{parent.name}</SelectLabel>
                        {parent.children?.length ? (
                          parent.children.map((child) => (
                            <SelectItem key={child.id} value={String(child.id)} className="pl-6">{child.name}</SelectItem>
                          ))
                        ) : (
                          <SelectItem value={String(parent.id)} className="pl-6">{parent.name}</SelectItem>
                        )}
                      </SelectGroup>
                    ))}
                  </SelectContent>
                </Select>
                <FormMessage />
              </FormItem>
            )} />
            <FormField control={form.control} name="slaId" render={({ field }) => (
              <FormItem>
                <FormLabel>{t("itsm:services.sla")}</FormLabel>
                <Select onValueChange={(v) => field.onChange(v === "0" ? null : Number(v))} value={String(field.value ?? 0)}>
                  <FormControl><SelectTrigger><SelectValue placeholder={t("itsm:services.slaPlaceholder")} /></SelectTrigger></FormControl>
                  <SelectContent>
                    <SelectItem value="0">—</SelectItem>
                    {slaTemplates.map((s) => (
                      <SelectItem key={s.id} value={String(s.id)}>{s.name}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                <FormMessage />
              </FormItem>
            )} />
          </div>

          {/* Row 3: Engine Type (half width) */}
          <div className="grid grid-cols-1 gap-5 sm:grid-cols-2">
            <FormField control={form.control} name="engineType" render={({ field }) => (
              <FormItem>
                <FormLabel>{t("itsm:services.engineType")}</FormLabel>
                <Select onValueChange={field.onChange} value={field.value}>
                  <FormControl><SelectTrigger><SelectValue /></SelectTrigger></FormControl>
                  <SelectContent>
                    <SelectItem value="classic">{t("itsm:services.engineClassic")}</SelectItem>
                    <SelectItem value="smart">{t("itsm:services.engineSmart")}</SelectItem>
                  </SelectContent>
                </Select>
                <FormMessage />
              </FormItem>
            )} />
          </div>

          {/* Description (full width) */}
          <FormField control={form.control} name="description" render={({ field }) => (
            <FormItem>
              <FormLabel>{t("itsm:services.description")}</FormLabel>
              <FormControl><Textarea rows={3} {...field} /></FormControl>
              <FormMessage />
            </FormItem>
          )} />

          {/* Smart Engine Config (full width) */}
          {form.watch("engineType") === "smart" && (
            <>
              <SmartServiceConfig
                collaborationSpec={form.watch("collaborationSpec")}
                onCollaborationSpecChange={(v) => form.setValue("collaborationSpec", v)}
              />
              <Alert>
                <Info className="h-4 w-4" />
                <AlertDescription>{t("itsm:knowledge.saveServiceFirst")}</AlertDescription>
              </Alert>
            </>
          )}

          <div className="flex gap-3 pt-2">
            <Button type="submit" disabled={createMut.isPending}>
              {createMut.isPending ? t("common:saving") : t("common:create")}
            </Button>
            <Button type="button" variant="outline" onClick={() => navigate("/itsm/services")}>
              {t("common:cancel")}
            </Button>
          </div>
        </form>
      </Form>
    </div>
  )
}

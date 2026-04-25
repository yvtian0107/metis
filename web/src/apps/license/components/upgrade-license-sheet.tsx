import { useEffect, useMemo, useState } from "react"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { api } from "@/lib/api"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Textarea } from "@/components/ui/textarea"
import { Switch } from "@/components/ui/switch"
import { Badge } from "@/components/ui/badge"
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetDescription,
  SheetFooter,
} from "@/components/ui/sheet"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { useTranslation } from "react-i18next"

interface ConstraintFeature {
  key: string
  label: string
  type: string
  min?: number
  max?: number
  default?: unknown
  options?: string[]
}

interface ConstraintModule {
  key: string
  label: string
  features: ConstraintFeature[]
}

interface PlanOption {
  id: number
  name: string
  constraintValues: Record<string, Record<string, unknown>>
  isDefault: boolean
}

interface ProductOption {
  id: number
  name: string
  code: string
  constraintSchema: ConstraintModule[] | null
  plans: PlanOption[] | null
}

export interface UpgradeLicenseItem {
  id: number
  productId: number | null
  licenseeId: number | null
  planId?: number | null
  planName: string
  licenseeName?: string
  registrationCode: string
  constraintValues?: Record<string, Record<string, unknown>>
  validFrom: string
  validUntil: string | null
  notes?: string
}

type ModuleValues = { enabled: boolean; [featureKey: string]: unknown }
type PlanValues = Record<string, ModuleValues>

interface UpgradeLicenseSheetProps {
  license: UpgradeLicenseItem | null
  open: boolean
  onOpenChange: (open: boolean) => void
}

function buildDefaults(schema: ConstraintModule[], existing: PlanValues = {}): PlanValues {
  const defaults: PlanValues = {}
  for (const mod of schema) {
    const existingMod = existing[mod.key]
    const modValues: ModuleValues = { enabled: existingMod?.enabled !== false }
    for (const feature of mod.features) {
      if (existingMod && existingMod[feature.key] !== undefined) {
        modValues[feature.key] = existingMod[feature.key]
      } else if (feature.default !== undefined) {
        modValues[feature.key] = feature.default
      } else if (feature.type === "number") {
        modValues[feature.key] = feature.min ?? 0
      } else if (feature.type === "multiSelect") {
        modValues[feature.key] = []
      }
    }
    defaults[mod.key] = modValues
  }
  return defaults
}

export function UpgradeLicenseSheet({ license, open, onOpenChange }: UpgradeLicenseSheetProps) {
  const { t } = useTranslation(["license", "common"])
  const queryClient = useQueryClient()

  const [planId, setPlanId] = useState<string>("")
  const [validFrom, setValidFrom] = useState("")
  const [validUntil, setValidUntil] = useState("")
  const [notes, setNotes] = useState("")
  const [constraintValues, setConstraintValues] = useState<PlanValues>({})

  // Reset form when sheet opens with a license
  useEffect(() => {
    if (open && license) {
      queueMicrotask(() => {
        const initialPlanId = license.planId ? String(license.planId) : "custom"
        setPlanId(initialPlanId)
        setValidFrom(license.validFrom ? license.validFrom.split("T")[0] : "")
        setValidUntil(license.validUntil ? license.validUntil.split("T")[0] : "")
        setNotes(license.notes || "")
        setConstraintValues((license.constraintValues as PlanValues) ?? {})
      })
    }
  }, [open, license])

  // Fetch product with plans and schema
  const { data: product } = useQuery({
    queryKey: ["license-product-detail", license?.productId],
    queryFn: () => api.get<ProductOption>(`/api/v1/license/products/${license!.productId}`),
    enabled: open && !!license?.productId,
  })

  const plans = useMemo(() => product?.plans ?? [], [product])
  const schema = useMemo(
    () => (product?.constraintSchema && Array.isArray(product.constraintSchema) ? product.constraintSchema : []),
    [product]
  )

  function handlePlanChange(value: string) {
    setPlanId(value)
    if (value === "custom") {
      setConstraintValues(buildDefaults(schema, constraintValues))
      return
    }
    const plan = plans.find((p) => String(p.id) === value)
    if (plan?.constraintValues) {
      setConstraintValues(buildDefaults(schema, plan.constraintValues as PlanValues))
    }
  }

  function setModuleEnabled(moduleKey: string, enabled: boolean) {
    const mod = schema.find((m) => m.key === moduleKey)
    const current = constraintValues[moduleKey] ?? { enabled: false }
    if (enabled && mod) {
      const featureDefaults: Record<string, unknown> = {}
      for (const f of mod.features) {
        if (current[f.key] !== undefined) {
          featureDefaults[f.key] = current[f.key]
        } else if (f.default !== undefined) {
          featureDefaults[f.key] = f.default
        } else if (f.type === "number") {
          featureDefaults[f.key] = f.min ?? 0
        } else if (f.type === "multiSelect") {
          featureDefaults[f.key] = []
        }
      }
      setConstraintValues({ ...constraintValues, [moduleKey]: { ...current, ...featureDefaults, enabled: true } })
    } else {
      setConstraintValues({ ...constraintValues, [moduleKey]: { ...current, enabled: false } })
    }
  }

  function setFeatureValue(moduleKey: string, featureKey: string, value: unknown) {
    const current = constraintValues[moduleKey] ?? { enabled: true }
    setConstraintValues({ ...constraintValues, [moduleKey]: { ...current, [featureKey]: value } })
  }

  const upgradeMutation = useMutation({
    mutationFn: (payload: Record<string, unknown>) =>
      api.post(`/api/v1/license/licenses/${license!.id}/upgrade`, payload),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["license-licenses"] })
      onOpenChange(false)
      toast.success(t("license:licenses.upgradeSuccess"))
    },
    onError: (err) => toast.error(err.message),
  })

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (!license || !validFrom) return

    const selectedPlan = plans.find((p) => String(p.id) === planId)
    const planName = planId === "custom" ? t("license:licenses.custom") : (selectedPlan?.name ?? t("license:licenses.custom"))

    upgradeMutation.mutate({
      productId: license.productId ?? undefined,
      licenseeId: license.licenseeId ?? undefined,
      planId: planId && planId !== "custom" ? Number(planId) : null,
      planName,
      registrationCode: license.registrationCode,
      constraintValues,
      validFrom,
      validUntil: validUntil || null,
      notes,
    })
  }

  const isPlanSelected = planId && planId !== "custom"
  const isCustomPlan = planId === "custom"

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className="overflow-y-auto sm:max-w-lg">
        <SheetHeader>
          <SheetTitle>{t("license:licenses.upgradeTitle")}</SheetTitle>
          <SheetDescription className="sr-only">{t("license:licenses.upgradeDesc")}</SheetDescription>
        </SheetHeader>
        <form onSubmit={handleSubmit} className="flex flex-1 flex-col gap-4 px-4">
          {/* Product (read-only) */}
          <div className="space-y-1.5">
            <Label className="text-xs text-muted-foreground">{t("license:licenses.product")}</Label>
            <Input value={product ? `${product.name} (${product.code})` : "-"} disabled />
          </div>

          {/* Licensee (read-only) */}
          <div className="space-y-1.5">
            <Label className="text-xs text-muted-foreground">{t("license:licenses.licensee")}</Label>
            <Input value={license?.licenseeName ?? (license?.licenseeId ? `#${license.licenseeId}` : "-")} disabled />
          </div>

          {/* Plan */}
          {product && (
            <div className="space-y-1.5">
              <Label className="text-xs text-muted-foreground">{t("license:licenses.selectPlanOrCustom")}</Label>
              <Select value={planId} onValueChange={handlePlanChange}>
                <SelectTrigger className="w-full">
                  <SelectValue placeholder={t("license:licenses.selectPlanOrCustom")} />
                </SelectTrigger>
                <SelectContent>
                  {plans.map((p) => (
                    <SelectItem key={p.id} value={String(p.id)}>
                      {p.name} {p.isDefault && t("license:licenses.defaultSuffix")}
                    </SelectItem>
                  ))}
                  <SelectItem value="custom">{t("license:licenses.custom")}</SelectItem>
                </SelectContent>
              </Select>
            </div>
          )}

          {/* Constraint Values - read-only summary for preset plan */}
          {product && schema.length > 0 && isPlanSelected && (
            <div className="space-y-2.5 rounded-lg border bg-muted/15 p-3">
              <div className="flex items-center justify-between">
                <Label className="text-xs text-muted-foreground">{t("license:licenses.planSummary")}</Label>
                <Badge variant="outline" className="rounded-md border-0 bg-muted/60 text-[11px] text-muted-foreground">
                  {schema.length} {t("license:plans.moduleCount", { count: schema.length })}
                </Badge>
              </div>
              {schema.map((mod) => {
                const modValues = constraintValues[mod.key] ?? { enabled: false }
                const isEnabled = !!modValues.enabled
                const enabledFeatures = mod.features.filter((f) => modValues[f.key] !== undefined)
                return (
                  <div key={mod.key} className="text-sm">
                    <div className="flex items-center gap-2">
                      <span className="font-medium">{mod.label || mod.key}</span>
                      <Badge variant={isEnabled ? "default" : "outline"} className="text-[10px]">
                        {isEnabled ? t("license:licenses.moduleEnabled") : t("license:licenses.moduleDisabled")}
                      </Badge>
                    </div>
                    {isEnabled && enabledFeatures.length > 0 && (
                      <div className="mt-1 flex flex-wrap gap-1 text-xs text-muted-foreground">
                        {enabledFeatures.map((f) => (
                          <span key={f.key} className="rounded bg-muted/60 px-1.5 py-0.5">
                            {f.label || f.key}: {String(modValues[f.key])}
                          </span>
                        ))}
                      </div>
                    )}
                  </div>
                )
              })}
            </div>
          )}

          {/* Constraint Values - editable for custom plan */}
          {product && schema.length > 0 && isCustomPlan && (
            <div className="space-y-2.5">
              <div className="flex items-center justify-between">
                <Label className="text-xs text-muted-foreground">{t("license:plans.constraintConfig")}</Label>
                <Badge variant="outline" className="rounded-md border-0 bg-muted/60 text-[11px] text-muted-foreground">
                  {schema.length} {t("license:plans.moduleCount", { count: schema.length })}
                </Badge>
              </div>
              {schema.map((mod) => {
                const modValues = constraintValues[mod.key] ?? { enabled: false }
                const isEnabled = !!modValues.enabled

                return (
                  <div
                    key={mod.key}
                    className={`rounded-lg border transition-colors ${
                      isEnabled ? "bg-muted/15" : "bg-muted/8 opacity-85"
                    }`}
                  >
                    <div className="flex items-center justify-between border-b bg-muted/20 px-3 py-2.5">
                      <div className="min-w-0">
                        <p className="text-sm font-medium">{mod.label || mod.key}</p>
                      </div>
                      <Switch
                        checked={isEnabled}
                        onCheckedChange={(checked) => setModuleEnabled(mod.key, !!checked)}
                      />
                    </div>
                    {isEnabled && mod.features.length > 0 && (
                      <div className="space-y-2.5 px-3 py-3">
                        {mod.features.map((feature) => (
                          <FeatureField
                            key={feature.key}
                            feature={feature}
                            value={modValues[feature.key]}
                            onChange={(v) => setFeatureValue(mod.key, feature.key, v)}
                          />
                        ))}
                      </div>
                    )}
                  </div>
                )
              })}
            </div>
          )}

          {/* Registration Code (read-only) */}
          <div className="space-y-1.5">
            <Label className="text-xs text-muted-foreground">{t("license:licenses.registrationCode")}</Label>
            <Input value={license?.registrationCode ?? ""} disabled />
          </div>

          {/* Valid From / Valid Until */}
          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-1.5">
              <Label className="text-xs text-muted-foreground">{t("license:licenses.validFromDate")}</Label>
              <Input
                type="date"
                value={validFrom}
                onChange={(e) => setValidFrom(e.target.value)}
                required
              />
            </div>
            <div className="space-y-1.5">
              <Label className="text-xs text-muted-foreground">{t("license:licenses.validUntilDate")}</Label>
              <Input
                type="date"
                value={validUntil}
                onChange={(e) => setValidUntil(e.target.value)}
                placeholder={t("license:licenses.emptyForPermanent")}
              />
            </div>
          </div>

          {/* Notes */}
          <div className="space-y-1.5">
            <Label className="text-xs text-muted-foreground">{t("license:licenses.notes")}</Label>
            <Textarea
              value={notes}
              onChange={(e) => setNotes(e.target.value)}
              placeholder={t("license:licenses.optionalNotes")}
              rows={2}
            />
          </div>

          <SheetFooter>
            <Button
              type="submit"
              size="sm"
              className="h-8 rounded-lg px-3"
              disabled={upgradeMutation.isPending || !validFrom}
            >
              {upgradeMutation.isPending ? t("common:processing") : t("common:confirm")}
            </Button>
          </SheetFooter>
        </form>
      </SheetContent>
    </Sheet>
  )
}

function FeatureField({
  feature,
  value,
  onChange,
}: {
  feature: ConstraintFeature
  value: unknown
  onChange: (v: unknown) => void
}) {
  const { t } = useTranslation("license")
  if (feature.type === "number") {
    return (
      <div className="rounded-md bg-background/60 px-3 py-2.5">
        <div className="flex items-center justify-between gap-2">
          <Label className="text-xs font-medium">{feature.label || feature.key}</Label>
          {(feature.min != null || feature.max != null) && (
            <Badge variant="outline" className="rounded-md border-0 bg-muted/60 text-[11px] text-muted-foreground">
              {feature.min ?? t("license:plans.noLimit")} ~ {feature.max ?? t("license:plans.noLimit")}
            </Badge>
          )}
        </div>
        <Input
          type="number"
          value={value != null ? String(value) : ""}
          min={feature.min}
          max={feature.max}
          onChange={(e) => onChange(e.target.value ? Number(e.target.value) : undefined)}
          className="mt-1.5 h-8 rounded-md bg-background/80 text-sm"
        />
      </div>
    )
  }

  if (feature.type === "enum") {
    return (
      <div className="rounded-md bg-background/60 px-3 py-2.5">
        <Label className="text-xs font-medium">{feature.label || feature.key}</Label>
        <Select value={value != null ? String(value) : ""} onValueChange={onChange}>
          <SelectTrigger className="mt-1.5 w-full rounded-md bg-background/80 text-sm h-8">
            <SelectValue placeholder={t("license:plans.selectPlaceholder")} />
          </SelectTrigger>
          <SelectContent>
            {(feature.options ?? []).map((opt) => (
              <SelectItem key={opt} value={opt}>{opt}</SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
    )
  }

  if (feature.type === "multiSelect") {
    const selected: string[] = Array.isArray(value) ? (value as string[]) : []
    return (
      <div className="rounded-md bg-background/60 px-3 py-2.5">
        <Label className="text-xs font-medium">{feature.label || feature.key}</Label>
        <div className="mt-1.5 flex flex-wrap gap-1.5">
          {(feature.options ?? []).map((opt) => {
            const isSelected = selected.includes(opt)
            return (
              <label
                key={opt}
                className={`inline-flex cursor-pointer items-center gap-1 rounded-md border px-2 py-1 text-xs transition-colors ${
                  isSelected
                    ? "border-primary/30 bg-primary/10 text-foreground"
                    : "border-border bg-transparent hover:bg-accent/60"
                }`}
              >
                <input
                  type="checkbox"
                  checked={isSelected}
                  onChange={(e) => {
                    if (e.target.checked) {
                      onChange([...selected, opt])
                    } else {
                      onChange(selected.filter((s) => s !== opt))
                    }
                  }}
                  className="sr-only"
                />
                {opt}
              </label>
            )
          })}
        </div>
      </div>
    )
  }

  return null
}

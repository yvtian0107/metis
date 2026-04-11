import { useMemo, useState } from "react"
import { useParams, useNavigate } from "react-router"
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { useTranslation } from "react-i18next"
import { ArrowLeft, Ban, Check, Copy, Download, Loader2 } from "lucide-react"
import { api } from "@/lib/api"
import { usePermission } from "@/hooks/use-permission"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "@/components/ui/alert-dialog"
import { formatDateTime } from "@/lib/utils"

interface LicenseDetail {
  id: number
  productId: number | null
  licenseeId: number | null
  planId: number | null
  planName: string
  registrationCode: string
  constraintValues: Record<string, Record<string, unknown>>
  validFrom: string
  validUntil: string | null
  activationCode: string
  keyVersion: number
  signature: string
  status: string
  issuedBy: number
  revokedAt: string | null
  revokedBy: number | null
  notes: string
  productName: string
  productCode: string
  licenseeName: string
  licenseeCode: string
  createdAt: string
  updatedAt: string
}

interface ConstraintFeature {
  key: string
  label: string
}

interface ConstraintModule {
  key: string
  label: string
  features: ConstraintFeature[]
}

interface ProductConstraintDetail {
  constraintSchema: ConstraintModule[] | null
}

interface SignedActivationClaims {
  pid?: string
  lic?: string
  licn?: string
  reg?: string
  iat?: number
  nbf?: number
  exp?: number | null
}

const STATUS_VARIANTS: Record<string, "default" | "destructive"> = {
  issued: "default",
  revoked: "destructive",
}

export function Component() {
  const { t } = useTranslation(["license", "common"])
  const { id } = useParams()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const canRevoke = usePermission("license:license:revoke")
  const [copied, setCopied] = useState(false)

  const { data: license, isLoading } = useQuery({
    queryKey: ["license-license", id],
    queryFn: () => api.get<LicenseDetail>(`/api/v1/license/licenses/${id}`),
    enabled: !!id,
  })

  const { data: productDetail } = useQuery({
    queryKey: ["license-product-constraint", license?.productId],
    queryFn: () => api.get<ProductConstraintDetail>(`/api/v1/license/products/${license?.productId}`),
    enabled: !!license?.productId,
  })

  const revokeMutation = useMutation({
    mutationFn: () => api.patch(`/api/v1/license/licenses/${id}/revoke`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["license-license", id] })
      queryClient.invalidateQueries({ queryKey: ["license-licenses"] })
      toast.success(t("license:licenses.revokeSuccess"))
    },
    onError: (err) => toast.error(err.message),
  })

  async function handleExport() {
    if (!id) return
    try {
      const blob = await api.download(`/api/v1/license/licenses/${id}/export`)
      const url = URL.createObjectURL(blob)
      const anchor = document.createElement("a")
      anchor.href = url
      anchor.download = `${license?.productCode || "license"}_${id}.lic`
      document.body.appendChild(anchor)
      anchor.click()
      anchor.remove()
      URL.revokeObjectURL(url)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("license:licenses.exportFailed"))
    }
  }

  async function handleCopyRegistrationCode() {
    if (!license?.registrationCode) return
    try {
      await navigator.clipboard.writeText(license.registrationCode)
      setCopied(true)
      toast.success(t("license:licenses.codeCopied"))
      window.setTimeout(() => setCopied(false), 1500)
    } catch {
      toast.error(t("license:licenses.copyFailed"))
    }
  }

  const modules = useMemo(() => {
    const constraintValues = license?.constraintValues ?? {}
    const constraintSchema = Array.isArray(productDetail?.constraintSchema) ? productDetail.constraintSchema : []

    const schemaByModule = new Map(constraintSchema.map((m) => [m.key, m]))
    const valueModuleKeys = Object.keys(constraintValues)

    const orderedModuleKeys = [
      ...constraintSchema.map((m) => m.key).filter((key) => key in constraintValues),
      ...valueModuleKeys.filter((key) => !schemaByModule.has(key)),
    ]

    return orderedModuleKeys.map((moduleKey) => {
      const moduleSchema = schemaByModule.get(moduleKey)
      const rawModuleValues = constraintValues[moduleKey]
      const moduleValues =
        rawModuleValues && typeof rawModuleValues === "object" && !Array.isArray(rawModuleValues)
          ? (rawModuleValues as Record<string, unknown>)
          : {}

      const featureLabelByKey = new Map(
        (moduleSchema?.features ?? []).map((feature) => [feature.key, feature.label || feature.key]),
      )

      const features = Object.entries(moduleValues)
        .filter(([key]) => key !== "enabled")
        .map(([key, value]) => ({
          key,
          label: featureLabelByKey.get(key) ?? key,
          value,
        }))

      return {
        key: moduleKey,
        label: moduleSchema?.label || moduleKey,
        isEnabled: moduleValues.enabled !== false,
        features,
      }
    })
  }, [license, productDetail])

  const signedClaims = useMemo(
    () => decodeActivationClaims(license?.activationCode),
    [license?.activationCode],
  )

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-20">
        <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
      </div>
    )
  }

  if (!license) {
    return (
      <div className="py-20 text-center text-muted-foreground">{t("license:licenses.notFound")}</div>
    )
  }

  const variant = STATUS_VARIANTS[license.status] ?? ("default" as const)
  const statusKey = license.status as string

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <Button variant="ghost" size="icon" className="h-8 w-8" onClick={() => navigate("/license/licenses")}>
            <ArrowLeft className="h-4 w-4" />
          </Button>
          <h2 className="text-lg font-semibold">{t("license:licenses.licenseDetail")}</h2>
          <Badge variant={variant}>{t(`license:status.${statusKey}`, license.status)}</Badge>
        </div>
        {license.status === "issued" && (
          <div className="flex items-center gap-2">
            <Button variant="outline" size="sm" onClick={handleExport}>
              <Download className="mr-1.5 h-4 w-4" />
              {t("license:licenses.exportLic")}
            </Button>
            {canRevoke && (
              <AlertDialog>
                <AlertDialogTrigger asChild>
                  <Button variant="destructive" size="sm">
                    <Ban className="mr-1.5 h-4 w-4" />
                    {t("license:licenses.revoke")}
                  </Button>
                </AlertDialogTrigger>
                <AlertDialogContent>
                  <AlertDialogHeader>
                    <AlertDialogTitle>{t("license:licenses.revokeTitle")}</AlertDialogTitle>
                    <AlertDialogDescription>
                      {t("license:licenses.revokeDesc")}
                    </AlertDialogDescription>
                  </AlertDialogHeader>
                  <AlertDialogFooter>
                    <AlertDialogCancel>{t("common:cancel")}</AlertDialogCancel>
                    <AlertDialogAction
                      onClick={() => revokeMutation.mutate()}
                      disabled={revokeMutation.isPending}
                    >
                      {revokeMutation.isPending ? t("common:processing") : t("license:licenses.confirmRevoke")}
                    </AlertDialogAction>
                  </AlertDialogFooter>
                </AlertDialogContent>
              </AlertDialog>
            )}
          </div>
        )}
      </div>

      <div className="grid gap-6 lg:grid-cols-2">
        {/* Basic Info */}
        <div className="rounded-lg border p-4 space-y-3">
          <h3 className="text-sm font-medium text-muted-foreground">{t("license:licenses.basicInfo")}</h3>
          <dl className="space-y-2 text-sm">
            <InfoRow label={t("license:licenses.product")} value={license.productName ? `${license.productName} (${license.productCode})` : "-"} />
            <InfoRow label={t("license:licenses.licensee")} value={license.licenseeName ? `${license.licenseeName} (${license.licenseeCode})` : "-"} />
            <InfoRow label={t("license:licenses.plan")} value={license.planName} />
            <div className="flex items-start justify-between gap-4">
              <dt className="text-muted-foreground shrink-0">{t("license:licenses.registrationCode")}</dt>
              <dd className="flex items-center gap-2">
                <span className="text-right break-all font-mono text-xs">{license.registrationCode}</span>
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  className="h-7 px-2"
                  onClick={handleCopyRegistrationCode}
                >
                  {copied ? <Check className="h-3.5 w-3.5" /> : <Copy className="h-3.5 w-3.5" />}
                  <span className="ml-1">{copied ? t("common:copied") : t("common:copy")}</span>
                </Button>
              </dd>
            </div>
          </dl>
        </div>

        {/* Validity */}
        <div className="rounded-lg border p-4 space-y-3">
          <h3 className="text-sm font-medium text-muted-foreground">{t("license:licenses.validity")}</h3>
          <dl className="space-y-2 text-sm">
            <InfoRow label={t("license:licenses.validFrom")} value={formatDateTime(license.validFrom)} />
            <InfoRow
              label={t("license:licenses.validUntil")}
              value={license.validUntil ? formatDateTime(license.validUntil) : t("license:licenses.permanentValid")}
            />
          </dl>
        </div>

        {/* Issuance Info */}
        <div className="rounded-lg border p-4 space-y-3">
          <h3 className="text-sm font-medium text-muted-foreground">{t("license:licenses.issuanceInfo")}</h3>
          <dl className="space-y-2 text-sm">
            <InfoRow label={t("license:licenses.issuedAt")} value={formatDateTime(license.createdAt)} />
            <InfoRow label={t("license:licenses.keyVersion")} value={`v${license.keyVersion}`} />
            {license.notes && <InfoRow label={t("license:licenses.notes")} value={license.notes} />}
          </dl>
        </div>

        {signedClaims && (
          <div className="rounded-lg border p-4 space-y-3">
            <h3 className="text-sm font-medium text-muted-foreground">{t("license:licenses.activationClaims")}</h3>
            <dl className="space-y-2 text-sm">
              <InfoRow label={t("license:licenses.productCode")} value={signedClaims.pid || license.productCode || "-"} mono />
              <InfoRow label={t("license:licenses.licensee")} value={signedClaims.licn || license.licenseeName || "-"} />
              <InfoRow label={t("license:licenses.licenseeCode")} value={signedClaims.lic || license.licenseeCode || "-"} mono />
            </dl>
          </div>
        )}

        {/* Revocation Info */}
        {license.status === "revoked" && (
          <div className="rounded-lg border border-destructive/20 bg-destructive/5 p-4 space-y-3">
            <h3 className="text-sm font-medium text-destructive">{t("license:licenses.revocationInfo")}</h3>
            <dl className="space-y-2 text-sm">
              <InfoRow label={t("license:licenses.revokedAt")} value={license.revokedAt ? formatDateTime(license.revokedAt) : "-"} />
            </dl>
          </div>
        )}
      </div>

      {/* Constraint Values */}
      {modules.length > 0 && (
        <div className="rounded-lg border p-4 space-y-2.5">
          <h3 className="text-sm font-medium text-muted-foreground">{t("license:licenses.constraintValues")}</h3>
          <div className="space-y-2">
            {modules.map((module) => {
              return (
                <div key={module.key} className="rounded-md border bg-muted/10 p-2.5">
                  <div className="mb-1.5 flex items-center gap-2">
                    <span className="text-sm font-medium leading-5">{module.label}</span>
                    <Badge variant={module.isEnabled ? "default" : "outline"} className="text-[11px]">
                      {module.isEnabled ? t("license:licenses.moduleEnabled") : t("license:licenses.moduleDisabled")}
                    </Badge>
                  </div>
                  {module.isEnabled && module.features.length > 0 && (
                    <div className="grid gap-1 text-sm">
                      {module.features.map((feature) => (
                        <div
                          key={feature.key}
                          className="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-3 rounded-md bg-background/65 px-2 py-1"
                        >
                          <span className="truncate text-muted-foreground">{feature.label}</span>
                          <span className="min-w-16 pr-1 text-right font-mono tabular-nums text-foreground">
                            {formatConstraintValue(feature.value, t)}
                          </span>
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              )
            })}
          </div>
        </div>
      )}
    </div>
  )
}

function decodeActivationClaims(activationCode?: string): SignedActivationClaims | null {
  if (!activationCode) {
    return null
  }

  try {
    const normalized = activationCode.replace(/-/g, "+").replace(/_/g, "/")
    const padded = normalized.padEnd(Math.ceil(normalized.length / 4) * 4, "=")
    const binary = window.atob(padded)
    const bytes = Uint8Array.from(binary, (char) => char.charCodeAt(0))
    const json = new TextDecoder().decode(bytes)
    return JSON.parse(json) as SignedActivationClaims
  } catch {
    return null
  }
}

function formatConstraintValue(value: unknown, t: (key: string) => string): string {
  if (value == null) {
    return "-"
  }
  if (Array.isArray(value)) {
    return value.length > 0 ? value.join(", ") : "-"
  }
  if (typeof value === "boolean") {
    return value ? t("common:yes") : t("common:no")
  }
  return String(value)
}

function InfoRow({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="flex items-start justify-between gap-4">
      <dt className="text-muted-foreground shrink-0">{label}</dt>
      <dd className={`text-right break-all ${mono ? "font-mono text-xs" : ""}`}>{value}</dd>
    </div>
  )
}

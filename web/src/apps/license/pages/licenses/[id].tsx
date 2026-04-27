import { useMemo, useState } from "react"
import { useParams, useNavigate } from "react-router"
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { useTranslation } from "react-i18next"
import {
  ArrowLeft,
  Ban,
  Check,
  Copy,
  Download,
  Loader2,
  Clock,
  Pause,
  Play,
  Eye,
  EyeOff,
  AlertTriangle,
  Code,
  Package,
  User,
  Calendar,
  FileKey,
  KeyRound,
  Shield,
  Fingerprint,
  Layers,
  Terminal,
  AlertCircle,
} from "lucide-react"
import { api } from "@/lib/api"
import { usePermission } from "@/hooks/use-permission"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Separator } from "@/components/ui/separator"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from "@/components/ui/sheet"
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
import { RenewLicenseSheet } from "../../components/renew-license-sheet"

interface LicenseDetail {
  id: number
  productId: number | null
  licenseeId: number | null
  planId: number | null
  planName: string
  registrationCode: string
  licenseKey: string
  constraintValues: Record<string, Record<string, unknown>>
  validFrom: string
  validUntil: string | null
  activationCode: string
  keyVersion: number
  signature: string
  status: string
  lifecycleStatus: string
  originalLicenseId: number | null
  suspendedAt: string | null
  suspendedBy: number | null
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

const LIFECYCLE_VARIANTS: Record<string, "default" | "destructive" | "outline" | "secondary"> = {
  pending: "secondary",
  active: "default",
  expired: "outline",
  suspended: "secondary",
  revoked: "destructive",
}

export function Component() {
  const { t } = useTranslation(["license", "common"])
  const { id } = useParams()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const canRevoke = usePermission("license:license:revoke")
  const canRenew = usePermission("license:license:renew")
  const canSuspend = usePermission("license:license:suspend")
  const canReactivate = usePermission("license:license:reactivate")
  const [copiedField, setCopiedField] = useState<string | null>(null)
  const [renewOpen, setRenewOpen] = useState(false)
  const [showKey, setShowKey] = useState(false)
  const [exampleOpen, setExampleOpen] = useState(false)

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

  const { data: publicKeyData } = useQuery({
    queryKey: ["license-product-public-key", license?.productId],
    queryFn: () => api.get<{ publicKey: string }>(`/api/v1/license/products/${license?.productId}/public-key`),
    enabled: !!license?.productId,
  })

  function useLifecycleMutation(method: "patch" | "post", path: string, successKey: string) {
    return useMutation({
      mutationFn: () => api[method](path),
      onSuccess: () => {
        queryClient.invalidateQueries({ queryKey: ["license-license", id] })
        queryClient.invalidateQueries({ queryKey: ["license-licenses"] })
        toast.success(t(`license:licenses.${successKey}`))
      },
      onError: (err: Error) => toast.error(err.message),
    })
  }

  const revokeMutation = useLifecycleMutation("patch", `/api/v1/license/licenses/${id}/revoke`, "revokeSuccess")
  const suspendMutation = useLifecycleMutation("post", `/api/v1/license/licenses/${id}/suspend`, "suspendSuccess")
  const reactivateMutation = useLifecycleMutation("post", `/api/v1/license/licenses/${id}/reactivate`, "reactivateSuccess")

  async function handleExport() {
    if (!id) return
    try {
      const blob = await api.download(`/api/v1/license/licenses/${id}/export?format=v2`)
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

  async function handleCopy(text: string, field: string) {
    if (!text) return
    try {
      await navigator.clipboard.writeText(text)
      setCopiedField(field)
      toast.success(t("license:licenses.codeCopied"))
      window.setTimeout(() => setCopiedField((prev) => (prev === field ? null : prev)), 1500)
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

  const variant = LIFECYCLE_VARIANTS[license.lifecycleStatus] ?? ("default" as const)
  const statusKey = license.lifecycleStatus as string
  const canExport = license.lifecycleStatus === "active" || license.lifecycleStatus === "pending" || license.lifecycleStatus === "expired"
  const canLifecycleAction = license.lifecycleStatus === "active" || license.lifecycleStatus === "pending" || license.lifecycleStatus === "expired"

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex items-center gap-3">
          <Button variant="ghost" size="icon" className="h-8 w-8" onClick={() => navigate("/license/licenses")}>
            <ArrowLeft className="h-4 w-4" />
          </Button>
          <div>
            <div className="flex items-center gap-2">
              <h2 className="text-lg font-semibold">{t("license:licenses.licenseDetail")}</h2>
              <Badge variant={variant}>{t(`license:lifecycleStatus.${statusKey}`, license.lifecycleStatus)}</Badge>
            </div>
            <p className="text-sm text-muted-foreground font-mono">{license.productCode} #{license.id}</p>
          </div>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          {canLifecycleAction && canRenew && (
            <Button variant="outline" size="sm" onClick={() => setRenewOpen(true)}>
              <Clock className="mr-1.5 h-4 w-4" />
              {t("license:licenses.renew")}
            </Button>
          )}
          {canLifecycleAction && canSuspend && (
            <Button variant="outline" size="sm" onClick={() => suspendMutation.mutate()} disabled={suspendMutation.isPending}>
              <Pause className="mr-1.5 h-4 w-4" />
              {t("license:licenses.suspend")}
            </Button>
          )}
          {license.lifecycleStatus === "suspended" && canReactivate && (
            <Button variant="outline" size="sm" onClick={() => reactivateMutation.mutate()} disabled={reactivateMutation.isPending}>
              <Play className="mr-1.5 h-4 w-4" />
              {t("license:licenses.reactivate")}
            </Button>
          )}
          {canLifecycleAction && canRevoke && (
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
      </div>

      {/* Status Banner */}
      {license.lifecycleStatus === "suspended" && (
        <Alert className="border-amber-200 bg-amber-50 text-amber-800 dark:border-amber-900/50 dark:bg-amber-900/20 dark:text-amber-400">
          <AlertTriangle className="h-4 w-4" />
          <AlertTitle>{t("license:licenses.suspendTitle")}</AlertTitle>
          <AlertDescription>
            {license.suspendedAt ? formatDateTime(license.suspendedAt) : "-"}
          </AlertDescription>
        </Alert>
      )}
      {license.lifecycleStatus === "revoked" && (
        <Alert variant="destructive">
          <Ban className="h-4 w-4" />
          <AlertTitle>{t("license:licenses.revocationInfo")}</AlertTitle>
          <AlertDescription>
            {license.revokedAt ? formatDateTime(license.revokedAt) : "-"}
          </AlertDescription>
        </Alert>
      )}

      {/* Main Grid — 2:1 layout */}
      <div className="grid gap-6 lg:grid-cols-12 lg:items-start">
        {/* Left: Main Content */}
        <div className="space-y-5 lg:col-span-8">
          {/* Basic Info */}
          <Card>
            <CardHeader>
              <CardTitle className="text-sm font-medium text-muted-foreground flex items-center gap-2">
                <Package className="h-4 w-4" />
                {t("license:licenses.basicInfo")}
              </CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="grid gap-4 sm:grid-cols-2">
                <div className="space-y-1">
                  <div className="flex items-center gap-2 text-xs text-muted-foreground">
                    <Package className="h-3.5 w-3.5" />
                    {t("license:licenses.product")}
                  </div>
                  <div className="font-medium">
                    {license.productName ? `${license.productName}` : "-"}
                  </div>
                  <div className="text-xs font-mono text-muted-foreground">{license.productCode}</div>
                </div>
                <div className="space-y-1">
                  <div className="flex items-center gap-2 text-xs text-muted-foreground">
                    <User className="h-3.5 w-3.5" />
                    {t("license:licenses.licensee")}
                  </div>
                  <div className="font-medium">
                    {license.licenseeName ? `${license.licenseeName}` : "-"}
                  </div>
                  <div className="text-xs font-mono text-muted-foreground">{license.licenseeCode}</div>
                </div>
                <div className="space-y-1">
                  <div className="flex items-center gap-2 text-xs text-muted-foreground">
                    <Layers className="h-3.5 w-3.5" />
                    {t("license:licenses.plan")}
                  </div>
                  <div className="font-medium">{license.planName}</div>
                </div>
                <div className="space-y-1">
                  <div className="flex items-center gap-2 text-xs text-muted-foreground">
                    <KeyRound className="h-3.5 w-3.5" />
                    {t("license:licenses.registrationCode")}
                  </div>
                  <div className="flex items-center gap-2">
                    <code className="rounded bg-muted px-2 py-1 text-xs font-mono">{license.registrationCode}</code>
                    <Button
                      type="button"
                      variant="ghost"
                      size="icon"
                      className="h-6 w-6"
                      onClick={() => handleCopy(license.registrationCode, "registrationCode")}
                    >
                      {copiedField === "registrationCode" ? <Check className="h-3.5 w-3.5" /> : <Copy className="h-3.5 w-3.5" />}
                    </Button>
                  </div>
                </div>
              </div>
              {license.originalLicenseId && (
                <div className="pt-2 border-t">
                  <div className="flex items-center gap-2 text-xs text-muted-foreground mb-1">
                    <AlertCircle className="h-3.5 w-3.5" />
                    {t("license:licenses.originalLicense")}
                  </div>
                  <Button variant="link" size="sm" className="h-auto p-0" onClick={() => navigate(`/license/licenses/${license.originalLicenseId}`)}>
                    #{license.originalLicenseId}
                  </Button>
                </div>
              )}
              {license.notes && (
                <div className="pt-2 border-t">
                  <div className="text-xs text-muted-foreground mb-1">{t("license:licenses.notes")}</div>
                  <p className="text-sm leading-relaxed">{license.notes}</p>
                </div>
              )}
            </CardContent>
          </Card>

          {/* Validity + Issuance */}
          <div className="grid gap-4 sm:grid-cols-2">
            <Card>
              <CardHeader>
                <CardTitle className="text-sm font-medium text-muted-foreground flex items-center gap-2">
                  <Calendar className="h-4 w-4" />
                  {t("license:licenses.validity")}
                </CardTitle>
              </CardHeader>
              <CardContent className="space-y-3">
                <InfoRow label={t("license:licenses.validFrom")} value={formatDateTime(license.validFrom)} />
                <InfoRow
                  label={t("license:licenses.validUntil")}
                  value={license.validUntil ? formatDateTime(license.validUntil) : t("license:licenses.permanentValid")}
                />
              </CardContent>
            </Card>
            <Card>
              <CardHeader>
                <CardTitle className="text-sm font-medium text-muted-foreground flex items-center gap-2">
                  <Clock className="h-4 w-4" />
                  {t("license:licenses.issuanceInfo")}
                </CardTitle>
              </CardHeader>
              <CardContent className="space-y-3">
                <InfoRow label={t("license:licenses.issuedAt")} value={formatDateTime(license.createdAt)} />
                <InfoRow label={t("license:licenses.keyVersion")} value={`v${license.keyVersion}`} />
              </CardContent>
            </Card>
          </div>

          {/* Activation Claims */}
          {signedClaims && (
            <Card>
              <CardHeader>
                <CardTitle className="text-sm font-medium text-muted-foreground flex items-center gap-2">
                  <Fingerprint className="h-4 w-4" />
                  {t("license:licenses.activationClaims")}
                </CardTitle>
              </CardHeader>
              <CardContent className="space-y-3">
                <InfoRow label={t("license:licenses.productCode")} value={signedClaims.pid || license.productCode || "-"} mono />
                <InfoRow label={t("license:licenses.licensee")} value={signedClaims.licn || license.licenseeName || "-"} />
                <InfoRow label={t("license:licenses.licenseeCode")} value={signedClaims.lic || license.licenseeCode || "-"} mono />
              </CardContent>
            </Card>
          )}

          {/* Constraint Values */}
          {modules.length > 0 && (
            <Card>
              <CardHeader>
                <CardTitle className="text-sm font-medium text-muted-foreground flex items-center gap-2">
                  <Layers className="h-4 w-4" />
                  {t("license:licenses.constraintValues")}
                </CardTitle>
              </CardHeader>
              <CardContent className="space-y-3">
                {modules.map((module) => {
                  return (
                    <div
                      key={module.key}
                      className={`rounded-lg border p-3 transition-colors ${module.isEnabled ? "bg-card" : "bg-muted/30"}`}
                    >
                      <div className="mb-2 flex items-center justify-between gap-2">
                        <span className="text-sm font-medium">{module.label}</span>
                        <Badge variant={module.isEnabled ? "default" : "outline"} className="text-[11px]">
                          {module.isEnabled ? t("license:licenses.moduleEnabled") : t("license:licenses.moduleDisabled")}
                        </Badge>
                      </div>
                      {module.isEnabled && module.features.length > 0 && (
                        <div className="grid gap-1.5">
                          {module.features.map((feature) => (
                            <div
                              key={feature.key}
                              className="flex items-center justify-between gap-3 rounded-md bg-muted/50 px-3 py-1.5 text-sm"
                            >
                              <span className="text-muted-foreground">{feature.label}</span>
                              <span className="font-mono tabular-nums text-foreground">
                                {formatConstraintValue(feature.value, t)}
                              </span>
                            </div>
                          ))}
                        </div>
                      )}
                    </div>
                  )
                })}
              </CardContent>
            </Card>
          )}
        </div>

        {/* Right: Developer Delivery */}
        <aside className="space-y-4 lg:col-span-4 lg:sticky lg:top-6">
          <Card className="border-primary/10 bg-gradient-to-b from-primary/5 to-card shadow-sm">
            <CardHeader>
              <CardTitle className="text-sm font-medium flex items-center gap-2">
                <Terminal className="h-4 w-4" />
                {t("license:licenses.developerDelivery")}
              </CardTitle>
              <CardDescription>集成验证所需的密钥与凭证</CardDescription>
            </CardHeader>
            <CardContent className="space-y-5">
              {/* License Key */}
              <div className="space-y-2">
                <div className="flex items-center gap-2 text-xs font-medium text-muted-foreground">
                  <FileKey className="h-3.5 w-3.5" />
                  {t("license:licenses.licenseKey")}
                </div>
                <div className="flex items-center gap-2">
                  <div className="flex-1 min-w-0 rounded-md border bg-background px-3 py-2 text-xs font-mono shadow-sm">
                    <span className="block truncate">
                      {showKey ? (license.licenseKey || "-") : (license.licenseKey ? "••••••••••••••••••••••••••••••••" : "-")}
                    </span>
                  </div>
                  <Button type="button" variant="ghost" size="icon" className="h-8 w-8 shrink-0" onClick={() => setShowKey((v) => !v)} disabled={!license.licenseKey}>
                    {showKey ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                  </Button>
                  <Button type="button" variant="outline" size="sm" className="h-8 px-2 shrink-0" onClick={() => handleCopy(license.licenseKey, "licenseKey")} disabled={!license.licenseKey}>
                    {copiedField === "licenseKey" ? <Check className="h-3.5 w-3.5" /> : <Copy className="h-3.5 w-3.5" />}
                    <span className="ml-1 hidden sm:inline">{copiedField === "licenseKey" ? t("common:copied") : t("common:copy")}</span>
                  </Button>
                </div>
              </div>

              {/* Registration Code */}
              <div className="space-y-2">
                <div className="flex items-center gap-2 text-xs font-medium text-muted-foreground">
                  <KeyRound className="h-3.5 w-3.5" />
                  {t("license:licenses.registrationCode")}
                </div>
                <div className="flex items-center gap-2">
                  <div className="flex-1 min-w-0 rounded-md border bg-background px-3 py-2 text-xs font-mono shadow-sm">
                    <span className="block truncate">{license.registrationCode}</span>
                  </div>
                  <Button type="button" variant="outline" size="sm" className="h-8 px-2 shrink-0" onClick={() => handleCopy(license.registrationCode, "registrationCode")}>
                    {copiedField === "registrationCode" ? <Check className="h-3.5 w-3.5" /> : <Copy className="h-3.5 w-3.5" />}
                    <span className="ml-1 hidden sm:inline">{copiedField === "registrationCode" ? t("common:copied") : t("common:copy")}</span>
                  </Button>
                </div>
              </div>

              {/* Public Key */}
              <div className="space-y-2">
                <div className="flex items-center gap-2 text-xs font-medium text-muted-foreground">
                  <Shield className="h-3.5 w-3.5" />
                  {t("license:licenses.publicKey")}
                </div>
                <div className="flex items-start gap-2">
                  <div className="flex-1 min-w-0 rounded-md border bg-background px-3 py-2 text-xs font-mono shadow-sm">
                    <span className="block truncate">{publicKeyData?.publicKey || "-"}</span>
                  </div>
                  <Button type="button" variant="outline" size="sm" className="h-8 px-2 shrink-0" disabled={!publicKeyData?.publicKey} onClick={() => handleCopy(publicKeyData?.publicKey || "", "publicKey")}>
                    {copiedField === "publicKey" ? <Check className="h-3.5 w-3.5" /> : <Copy className="h-3.5 w-3.5" />}
                    <span className="ml-1 hidden sm:inline">{copiedField === "publicKey" ? t("common:copied") : t("common:copy")}</span>
                  </Button>
                </div>
              </div>

              <Separator className="bg-primary/10" />

              <div className="grid gap-2">
                <Button variant="default" size="sm" className="w-full" onClick={handleExport} disabled={!canExport}>
                  <Download className="mr-1.5 h-4 w-4" />
                  {t("license:licenses.downloadLic")}
                </Button>
                <Sheet open={exampleOpen} onOpenChange={setExampleOpen}>
                  <SheetTrigger asChild>
                    <Button variant="outline" size="sm" className="w-full">
                      <Code className="mr-1.5 h-4 w-4" />
                      {t("license:licenses.verifyExample")}
                    </Button>
                  </SheetTrigger>
                  <SheetContent className="sm:max-w-xl overflow-y-auto">
                    <SheetHeader>
                      <SheetTitle>{t("license:licenses.verifyExample")}</SheetTitle>
                    </SheetHeader>
                    <div className="mt-2 space-y-4 px-6 pb-6">
                      <div className="space-y-3">
                        <div className="space-y-1">
                          <div className="text-xs text-muted-foreground">{t("license:licenses.licenseKey")}</div>
                          <div className="flex items-center gap-2">
                            <code className="flex-1 truncate rounded bg-muted px-2 py-1 text-xs font-mono">{license.licenseKey || "-"}</code>
                            <Button type="button" variant="outline" size="sm" className="h-7 px-2 shrink-0" disabled={!license.licenseKey} onClick={() => handleCopy(license.licenseKey, "licenseKey")}>
                              {copiedField === "licenseKey" ? <Check className="h-3.5 w-3.5" /> : <Copy className="h-3.5 w-3.5" />}
                              <span className="ml-1">{copiedField === "licenseKey" ? t("common:copied") : t("common:copy")}</span>
                            </Button>
                          </div>
                        </div>
                        <div className="space-y-1">
                          <div className="text-xs text-muted-foreground">{t("license:licenses.registrationCode")}</div>
                          <div className="flex items-center gap-2">
                            <code className="flex-1 truncate rounded bg-muted px-2 py-1 text-xs font-mono">{license.registrationCode}</code>
                            <Button type="button" variant="outline" size="sm" className="h-7 px-2 shrink-0" onClick={() => handleCopy(license.registrationCode, "registrationCode")}>
                              {copiedField === "registrationCode" ? <Check className="h-3.5 w-3.5" /> : <Copy className="h-3.5 w-3.5" />}
                              <span className="ml-1">{copiedField === "registrationCode" ? t("common:copied") : t("common:copy")}</span>
                            </Button>
                          </div>
                        </div>
                        <div className="space-y-1">
                          <div className="text-xs text-muted-foreground">{t("license:licenses.publicKey")}</div>
                          <div className="flex items-center gap-2">
                            <code className="flex-1 truncate rounded bg-muted px-2 py-1 text-xs font-mono">{publicKeyData?.publicKey || "-"}</code>
                            <Button type="button" variant="outline" size="sm" className="h-7 px-2 shrink-0" disabled={!publicKeyData?.publicKey} onClick={() => handleCopy(publicKeyData?.publicKey || "", "publicKey")}>
                              {copiedField === "publicKey" ? <Check className="h-3.5 w-3.5" /> : <Copy className="h-3.5 w-3.5" />}
                              <span className="ml-1">{copiedField === "publicKey" ? t("common:copied") : t("common:copy")}</span>
                            </Button>
                          </div>
                        </div>
                      </div>

                      <Button variant="default" size="sm" className="w-full" onClick={handleExport} disabled={!canExport}>
                        <Download className="mr-1.5 h-4 w-4" />
                        {t("license:licenses.downloadLic")}
                      </Button>

                      <Tabs defaultValue="go" className="w-full">
                        <TabsList className="h-8">
                          <TabsTrigger value="go" className="text-xs px-3">Go</TabsTrigger>
                          <TabsTrigger value="js" className="text-xs px-3">JavaScript</TabsTrigger>
                          <TabsTrigger value="py" className="text-xs px-3">Python</TabsTrigger>
                        </TabsList>
                        <TabsContent value="go">
                          <pre className="rounded bg-muted p-3 text-xs overflow-x-auto font-mono">{goVerifyExample}</pre>
                        </TabsContent>
                        <TabsContent value="js">
                          <pre className="rounded bg-muted p-3 text-xs overflow-x-auto font-mono">{jsVerifyExample}</pre>
                        </TabsContent>
                        <TabsContent value="py">
                          <pre className="rounded bg-muted p-3 text-xs overflow-x-auto font-mono">{pyVerifyExample}</pre>
                        </TabsContent>
                      </Tabs>
                    </div>
                  </SheetContent>
                </Sheet>
              </div>
            </CardContent>
          </Card>
        </aside>
      </div>

      <RenewLicenseSheet license={license} open={renewOpen} onOpenChange={setRenewOpen} />
    </div>
  )
}

const goVerifyExample = `package main

import (
    "crypto/aes"
    "crypto/cipher"
    "crypto/ed25519"
    "crypto/sha256"
    "encoding/base64"
    "encoding/json"
    "fmt"
    "strings"
)

func main() {
    lic := "A1.xxx"               // .lic file content
    licenseKey := "YOUR_LICENSE_KEY"
    regCode := "YOUR_REGISTRATION_CODE"

    dot := strings.IndexRune(lic, '.')
    fileToken := lic[:dot]
    encoded := lic[dot+1:]

    h := sha256.Sum256([]byte(fileToken + ":" + licenseKey + ":" + regCode))
    key := h[:]

    ct, _ := base64.RawURLEncoding.DecodeString(encoded)
    block, _ := aes.NewCipher(key)
    aead, _ := cipher.NewGCM(block)
    nonceSize := aead.NonceSize()
    plain, _ := aead.Open(nil, ct[:nonceSize], ct[nonceSize:], nil)

    var payload struct {
        ActivationCode string \`json:"activationCode"\`
        PublicKey      string \`json:"publicKey"\`
    }
    json.Unmarshal(plain, &payload)

    // Decode activationCode -> payload + sig
    acData, _ := base64.RawURLEncoding.DecodeString(payload.ActivationCode)
    var claims map[string]any
    json.Unmarshal(acData, &claims)
    sig, _ := base64.RawURLEncoding.DecodeString(claims["sig"].(string))

    // Canonicalize and verify (simplified)
    canon, _ := json.Marshal(claims) // use canonicalization as needed
    pub, _ := base64.StdEncoding.DecodeString(payload.PublicKey)
    ok := ed25519.Verify(ed25519.PublicKey(pub), canon, sig)
    fmt.Println("valid:", ok)
}`

const jsVerifyExample = `async function verifyLicense(lic, licenseKey, regCode) {
  const dot = lic.indexOf('.');
  const fileToken = lic.slice(0, dot);
  const encoded = lic.slice(dot + 1);

  const enc = new TextEncoder();
  const msg = enc.encode(fileToken + ':' + licenseKey + ':' + regCode);
  const hashBuffer = await crypto.subtle.digest('SHA-256', msg);
  const keyBytes = new Uint8Array(hashBuffer);

  const ct = Uint8Array.from(atob(encoded.replace(/-/g, '+').replace(/_/g, '/')), c => c.charCodeAt(0));
  const aesKey = await crypto.subtle.importKey('raw', keyBytes, 'AES-GCM', false, ['decrypt']);
  const plain = await crypto.subtle.decrypt({ name: 'AES-GCM', iv: ct.slice(0, 12) }, aesKey, ct.slice(12));
  const { activationCode, publicKey } = JSON.parse(new TextDecoder().decode(plain));

  const acData = Uint8Array.from(atob(activationCode.replace(/-/g, '+').replace(/_/g, '/').padEnd(Math.ceil(activationCode.length/4)*4,'=')), c => c.charCodeAt(0));
  const { sig, ...claims } = JSON.parse(new TextDecoder().decode(acData));

  // For Ed25519 verification in browser, use tweetnacl or @noble/ed25519
  console.log('claims:', claims, 'publicKey:', publicKey);
}`

const pyVerifyExample = `import hashlib
import base64
import json
from cryptography.hazmat.primitives.ciphers.aead import AESGCM

def verify_license(lic, license_key, reg_code):
    dot = lic.find('.')
    file_token = lic[:dot]
    encoded = lic[dot+1:]

    key = hashlib.sha256(f"{file_token}:{license_key}:{reg_code}".encode()).digest()
    ct = base64.urlsafe_b64decode(encoded + '==')
    nonce = ct[:12]
    cipher = AESGCM(key)
    plain = cipher.decrypt(nonce, ct[12:], None)

    payload = json.loads(plain)
    activation_code = payload['activationCode']
    public_key = payload['publicKey']

    ac_data = base64.urlsafe_b64decode(activation_code + '==')
    claims = json.loads(ac_data)

    print('claims:', claims, 'publicKey:', public_key)
    return claims, public_key`

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
      <dt className="text-muted-foreground shrink-0 text-sm">{label}</dt>
      <dd className={`text-right break-all text-sm ${mono ? "font-mono text-xs" : "font-medium"}`}>{value}</dd>
    </div>
  )
}

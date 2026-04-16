import { useState } from "react"
import { useParams, useNavigate, useSearchParams } from "react-router"
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { useTranslation } from "react-i18next"
import {
  ArrowLeft,
  Check,
  Copy,
  Eye,
  EyeOff,
  Key,
  Loader2,
  Pencil,
  RefreshCw,
} from "lucide-react"
import { api } from "@/lib/api"
import { usePermission } from "@/hooks/use-permission"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import {
  Card,
  CardContent,
  CardTitle,
  CardDescription,
} from "@/components/ui/card"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
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
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { formatDateTime } from "@/lib/utils"
import { ProductSheet, type ProductItem } from "../../components/product-sheet"
import { ConstraintEditor } from "../../components/constraint-editor"
import { PlanTab } from "../../components/plan-tab"
import { STATUS_STYLES, STATUS_ACTION_CONFIG } from "../../constants"

interface ConstraintFeature {
  key: string
  label: string
  type: string
  options?: string[]
}

interface ConstraintModule {
  key: string
  label: string
  features: ConstraintFeature[]
}

interface ProductDetail {
  id: number
  name: string
  code: string
  description: string
  status: string
  licenseKey: string
  constraintSchema: ConstraintModule[] | null
  planCount: number
  plans: Array<{
    id: number
    productId: number
    name: string
    constraintValues: Record<string, Record<string, unknown>>
    isDefault: boolean
    sortOrder: number
    createdAt: string
    updatedAt: string
  }>
  createdAt: string
  updatedAt: string
}

interface PublicKeyInfo {
  id: number
  version: number
  publicKey: string
  isCurrent: boolean
  createdAt: string
}

interface RotateKeyImpact {
  affectedCount: number
  currentVersion: number
}

export function Component() {
  const { t } = useTranslation(["license", "common"])
  const { id } = useParams()
  const navigate = useNavigate()
  const [searchParams, setSearchParams] = useSearchParams()
  const queryClient = useQueryClient()
  const [editOpen, setEditOpen] = useState(false)
  const [impactOpen, setImpactOpen] = useState(false)
  const [bulkReissueOpen, setBulkReissueOpen] = useState(false)
  const [showLicenseKey, setShowLicenseKey] = useState(false)
  const [copiedField, setCopiedField] = useState<string | null>(null)

  const canUpdate = usePermission("license:product:update")
  const canManagePlan = usePermission("license:plan:manage")
  const canManageKey = usePermission("license:key:manage")

  const { data: product, isLoading } = useQuery({
    queryKey: ["license-product", id],
    queryFn: () => api.get<ProductDetail>(`/api/v1/license/products/${id}`),
    enabled: !!id,
  })

  const statusMutation = useMutation({
    mutationFn: (status: string) =>
      api.patch(`/api/v1/license/products/${id}/status`, { status }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["license-product", id] })
      queryClient.invalidateQueries({ queryKey: ["license-products"] })
      queryClient.invalidateQueries({ queryKey: ["license-products-published"] })
      toast.success(t("license:products.statusUpdateSuccess"))
    },
    onError: (err) => toast.error(err.message),
  })

  const { data: publicKey } = useQuery({
    queryKey: ["license-product-key", id],
    queryFn: () => api.get<PublicKeyInfo>(`/api/v1/license/products/${id}/public-key`),
    enabled: !!id,
  })

  const { data: impact } = useQuery({
    queryKey: ["license-product-key-impact", id],
    queryFn: () => api.get<RotateKeyImpact>(`/api/v1/license/products/${id}/rotate-key-impact`),
    enabled: !!id && impactOpen,
  })

  const rotateKeyMutation = useMutation({
    mutationFn: () => api.post(`/api/v1/license/products/${id}/rotate-key`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["license-product-key", id] })
      queryClient.invalidateQueries({ queryKey: ["license-product-key-impact", id] })
      toast.success(t("license:products.rotateKeySuccess"))
      setImpactOpen(false)
    },
    onError: (err) => toast.error(err.message),
  })

  const bulkReissueMutation = useMutation({
    mutationFn: () => api.post(`/api/v1/license/products/${id}/bulk-reissue`, { licenseIds: [] }),
    onSuccess: (data: { reissued: number }) => {
      queryClient.invalidateQueries({ queryKey: ["license-licenses"] })
      queryClient.invalidateQueries({ queryKey: ["license-product-key-impact", id] })
      toast.success(t("license:products.bulkReissueSuccess", { count: data.reissued }))
      setBulkReissueOpen(false)
    },
    onError: (err) => toast.error(err.message),
  })

  const modules = Array.isArray(product?.constraintSchema) ? product.constraintSchema : []
  const requestedTab = searchParams.get("tab")
  const activeTab =
    requestedTab === "info" || requestedTab === "schema" || requestedTab === "plans" || requestedTab === "keys"
      ? requestedTab
      : "info"

  if (isLoading || !product) {
    return (
      <div className="flex min-h-[200px] items-center justify-center text-muted-foreground">
        {t("common:loading")}
      </div>
    )
  }

  const statusStyle = STATUS_STYLES[product.status] ?? STATUS_STYLES.unpublished
  const statusKey = product.status as string
  const actions = STATUS_ACTION_CONFIG[product.status] ?? []

  function handleTabChange(value: string) {
    const nextParams = new URLSearchParams(searchParams)
    nextParams.set("tab", value)
    setSearchParams(nextParams, { replace: true })
  }

  async function handleCopy(text: string, field: string) {
    if (!text) return
    try {
      await navigator.clipboard.writeText(text)
      setCopiedField(field)
      toast.success(t("common:copied"))
      window.setTimeout(() => setCopiedField((prev) => (prev === field ? null : prev)), 1500)
    } catch {
      toast.error(t("license:licenses.copyFailed"))
    }
  }

  function handleRotateKeyClick() {
    setImpactOpen(true)
  }

  const editableProduct: ProductItem = {
    id: product.id,
    name: product.name,
    code: product.code,
    description: product.description,
    status: product.status,
    planCount: product.planCount,
    createdAt: product.createdAt,
    updatedAt: product.updatedAt,
  }

  return (
    <div className="space-y-6">
      <Card className="flex-row items-center gap-4 py-4">
        <Button variant="ghost" size="icon" onClick={() => navigate("/license/products")} className="ml-4 shrink-0">
          <ArrowLeft className="h-4 w-4" />
        </Button>
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2 flex-wrap">
            <CardTitle className="text-lg">{product.name}</CardTitle>
            <Badge
              variant={statusStyle.variant}
              className={statusStyle.className}
            >
              {t(`license:status.${statusKey}`, product.status)}
            </Badge>
          </div>
          <CardDescription className="font-mono mt-1">{product.code}</CardDescription>
        </div>
        <div className="mr-4 flex shrink-0 items-center gap-2">
          {canUpdate && actions.map((action) => {
            const ActionIcon = action.icon
            const actionLabel = t(`license:${action.labelKey}`)
            return (
              <AlertDialog key={action.status}>
                <AlertDialogTrigger asChild>
                  <Button variant={action.variant} size="sm">
                    <ActionIcon className="mr-1.5 h-3.5 w-3.5" />
                    <span className="hidden sm:inline">{actionLabel}</span>
                  </Button>
                </AlertDialogTrigger>
                <AlertDialogContent>
                  <AlertDialogHeader>
                    <AlertDialogTitle>{t("license:products.confirmAction", { action: actionLabel })}</AlertDialogTitle>
                    <AlertDialogDescription>
                      {t("license:products.confirmActionDesc", { name: product.name, action: actionLabel })}
                    </AlertDialogDescription>
                  </AlertDialogHeader>
                  <AlertDialogFooter>
                    <AlertDialogCancel>{t("common:cancel")}</AlertDialogCancel>
                    <AlertDialogAction
                      onClick={() => statusMutation.mutate(action.status)}
                      disabled={statusMutation.isPending}
                    >
                      {actionLabel}
                    </AlertDialogAction>
                  </AlertDialogFooter>
                </AlertDialogContent>
              </AlertDialog>
            )
          })}
          {canUpdate && (
            <Button variant="outline" size="sm" onClick={() => setEditOpen(true)}>
              <Pencil className="mr-1.5 h-3.5 w-3.5" />
              <span className="hidden sm:inline">{t("common:edit")}</span>
            </Button>
          )}
        </div>
      </Card>

      <Tabs value={activeTab} onValueChange={handleTabChange}>
        <TabsList className="h-auto w-fit max-w-full flex-wrap justify-start gap-1 rounded-lg bg-muted/50 p-1">
          <TabsTrigger
            value="info"
            className="h-8 flex-none px-3 text-xs sm:text-sm data-[state=active]:bg-background data-[state=active]:shadow-sm"
          >
            {t("license:products.basicInfo")}
          </TabsTrigger>
          <TabsTrigger
            value="schema"
            className="h-8 flex-none px-3 text-xs sm:text-sm data-[state=active]:bg-background data-[state=active]:shadow-sm"
          >
            {t("license:products.constraintDef")}
          </TabsTrigger>
          <TabsTrigger
            value="plans"
            className="h-8 flex-none px-3 text-xs sm:text-sm data-[state=active]:bg-background data-[state=active]:shadow-sm"
          >
            {t("license:products.planManagement")}
          </TabsTrigger>
          <TabsTrigger
            value="keys"
            className="h-8 flex-none px-3 text-xs sm:text-sm data-[state=active]:bg-background data-[state=active]:shadow-sm"
          >
            {t("license:products.keyManagement")}
          </TabsTrigger>
        </TabsList>

        <TabsContent value="info" className="space-y-4">
          <Card>
            <CardContent className="pt-6">
              <div className="grid gap-4 text-sm sm:grid-cols-3">
                <div className="rounded-lg border bg-muted/30 p-3">
                  <p className="text-xs text-muted-foreground">{t("license:products.licenseModules")}</p>
                  <p className="mt-1 font-medium tabular-nums">{modules.length}</p>
                </div>
                <div className="rounded-lg border bg-muted/30 p-3">
                  <p className="text-xs text-muted-foreground">{t("license:products.planQuantity")}</p>
                  <p className="mt-1 font-medium tabular-nums">{product.planCount}</p>
                </div>
                <div className="rounded-lg border bg-muted/30 p-3">
                  <p className="text-xs text-muted-foreground">{t("common:updatedAt")}</p>
                  <p className="mt-1">{formatDateTime(product.updatedAt)}</p>
                </div>
                {product.description && (
                  <div className="rounded-lg border bg-muted/30 p-3 sm:col-span-3">
                    <p className="text-xs text-muted-foreground">{t("common:description")}</p>
                    <p className="mt-1 leading-6">{product.description}</p>
                  </div>
                )}
              </div>
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="schema">
          <ConstraintEditor
            productId={product.id}
            schema={product.constraintSchema}
            canEdit={canUpdate}
          />
        </TabsContent>

        <TabsContent value="plans">
          <PlanTab
            productId={product.id}
            plans={product.plans ?? []}
            constraintSchema={product.constraintSchema}
            canManage={canManagePlan}
            onRequestDefineConstraints={() => handleTabChange("schema")}
          />
        </TabsContent>

        <TabsContent value="keys" className="space-y-4">
          {/* License Key */}
          {product.licenseKey && (
            <Card>
              <CardContent className="space-y-3 pt-5">
                <p className="text-xs font-medium text-muted-foreground">{t("license:licenses.licenseKey")}</p>
                <div className="flex items-center gap-2">
                  <code className="flex-1 truncate rounded-md bg-slate-950 px-3 py-2 text-xs font-mono text-slate-50 dark:bg-slate-900">
                    {showLicenseKey ? product.licenseKey : "••••••••••••••••••••••••••••••••"}
                  </code>
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon"
                    className="h-8 w-8 shrink-0"
                    onClick={() => setShowLicenseKey((v) => !v)}
                  >
                    {showLicenseKey ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                  </Button>
                  <Button
                    type="button"
                    variant={copiedField === "licenseKey" ? "default" : "outline"}
                    size="sm"
                    className="h-8 shrink-0 px-2"
                    onClick={() => handleCopy(product.licenseKey, "licenseKey")}
                  >
                    {copiedField === "licenseKey" ? (
                      <>
                        <Check className="h-3.5 w-3.5" />
                        <span className="ml-1">{t("common:copied")}</span>
                      </>
                    ) : (
                      <>
                        <Copy className="h-3.5 w-3.5" />
                        <span className="ml-1">{t("common:copy")}</span>
                      </>
                    )}
                  </Button>
                </div>
              </CardContent>
            </Card>
          )}

          {/* Signing Key */}
          <Card>
            <CardContent className="space-y-4 pt-5">
              <div className="flex items-center gap-2">
                <Key className="h-4 w-4 text-muted-foreground" />
                <p className="text-sm font-medium">{t("license:products.currentKey")}</p>
                {publicKey && (
                  <Badge variant="secondary">v{publicKey.version}</Badge>
                )}
              </div>
              {publicKey ? (
                <>
                  <div>
                    <p className="text-xs text-muted-foreground">{t("license:products.publicKey")}</p>
                    <div className="relative mt-2">
                      <pre className="rounded-md bg-slate-950 p-4 text-xs break-all whitespace-pre-wrap font-mono text-slate-50 dark:bg-slate-900">
                        {publicKey.publicKey}
                      </pre>
                      <Button
                        type="button"
                        variant="secondary"
                        size="sm"
                        className="absolute right-2 top-2 h-7 px-2"
                        onClick={() => handleCopy(publicKey.publicKey, "publicKey")}
                      >
                        {copiedField === "publicKey" ? (
                          <>
                            <Check className="h-3.5 w-3.5" />
                            <span className="ml-1">{t("common:copied")}</span>
                          </>
                        ) : (
                          <>
                            <Copy className="h-3.5 w-3.5" />
                            <span className="ml-1">{t("common:copy")}</span>
                          </>
                        )}
                      </Button>
                    </div>
                  </div>
                  <div className="flex items-center gap-2 text-sm">
                    <p className="text-muted-foreground">{t("common:createdAt")}</p>
                    <p>{formatDateTime(publicKey.createdAt)}</p>
                  </div>
                </>
              ) : (
                <p className="text-sm text-muted-foreground">{t("license:products.noKeyInfo")}</p>
              )}
              {canManageKey && (
                <div className="flex flex-wrap items-center gap-2 border-t pt-4">
                  <Button variant="outline" size="sm" className="border-amber-200 text-amber-700 hover:bg-amber-50 hover:text-amber-800 dark:border-amber-900 dark:text-amber-400 dark:hover:bg-amber-950" onClick={handleRotateKeyClick}>
                    <RefreshCw className="mr-1.5 h-3.5 w-3.5" />
                    {t("license:products.rotateKey")}
                  </Button>
                  {impact && impact.affectedCount > 0 && (
                    <Button variant="secondary" size="sm" onClick={() => setBulkReissueOpen(true)}>
                      {t("license:products.bulkReissue")}
                    </Button>
                  )}
                </div>
              )}
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>

      <ProductSheet open={editOpen} onOpenChange={setEditOpen} product={editableProduct} />

      {/* Rotate Key Impact Dialog */}
      <Dialog open={impactOpen} onOpenChange={setImpactOpen}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>{t("license:products.rotateKeyImpactTitle")}</DialogTitle>
            <DialogDescription>
              {!impact ? (
                <span className="flex items-center gap-2">
                  <Loader2 className="h-4 w-4 animate-spin" />
                  {t("common:loading")}
                </span>
              ) : impact.affectedCount > 0 ? (
                t("license:products.rotateKeyImpactDesc", {
                  version: impact.currentVersion,
                  count: impact.affectedCount,
                })
              ) : (
                t("license:products.rotateKeyNoImpact")
              )}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter className="gap-2">
            <Button variant="outline" onClick={() => setImpactOpen(false)}>
              {t("common:cancel")}
            </Button>
            {impact && impact.affectedCount > 0 && (
              <Button variant="secondary" onClick={() => setBulkReissueOpen(true)}>
                {t("license:products.bulkReissue")}
              </Button>
            )}
            <Button
              onClick={() => rotateKeyMutation.mutate()}
              disabled={rotateKeyMutation.isPending}
            >
              {rotateKeyMutation.isPending && <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />}
              {t("license:products.confirmRotate")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Bulk Reissue Dialog */}
      <Dialog open={bulkReissueOpen} onOpenChange={setBulkReissueOpen}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>{t("license:products.bulkReissueTitle")}</DialogTitle>
            <DialogDescription>{t("license:products.bulkReissueDesc")}</DialogDescription>
          </DialogHeader>
          <DialogFooter className="gap-2">
            <Button variant="outline" onClick={() => setBulkReissueOpen(false)}>
              {t("common:cancel")}
            </Button>
            <Button
              onClick={() => bulkReissueMutation.mutate()}
              disabled={bulkReissueMutation.isPending}
            >
              {bulkReissueMutation.isPending && <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />}
              {t("common:confirm")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}

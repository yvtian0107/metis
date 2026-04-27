import { useState } from "react"
import { useParams, useNavigate, useSearchParams } from "react-router"
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { useTranslation } from "react-i18next"
import {
  ArrowLeft,
  Check,
  Copy,
  Loader2,
  Pencil,
  RefreshCw,
} from "lucide-react"
import { api } from "@/lib/api"
import { usePermission } from "@/hooks/use-permission"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Separator } from "@/components/ui/separator"
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
import { STATUS_STYLES, STATUS_ACTION_CONFIG, type StatusActionConfig } from "../../constants"

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
    mutationFn: () => api.post<{ reissued: number }>(`/api/v1/license/products/${id}/bulk-reissue`, { licenseIds: [] }),
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
    <div className="space-y-4">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex items-center gap-3">
          <Button variant="outline" size="icon" onClick={() => navigate("/license/products")} className="h-8 w-8 shrink-0">
            <ArrowLeft className="h-4 w-4" />
          </Button>
          <div className="flex flex-col gap-1">
            <div className="flex items-center gap-2.5 flex-wrap">
              <h2 className="text-lg font-semibold">{product.name}</h2>
              <Badge
                variant={statusStyle.variant}
                className={statusStyle.className}
              >
                {t(`license:status.${statusKey}`, product.status)}
              </Badge>
            </div>
            <div className="flex items-center gap-2 text-xs text-muted-foreground font-mono">
              <span className="bg-muted px-1.5 py-0.5 rounded text-xs">{product.code}</span>
            </div>
          </div>
        </div>
        <div className="flex shrink-0 items-center gap-2">
          {canUpdate && actions.map((action: StatusActionConfig) => {
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
      </div>

      <Tabs value={activeTab} onValueChange={handleTabChange}>
        <TabsList variant="line" className="h-auto w-full justify-start gap-0 rounded-none border-b bg-transparent p-0">
          <TabsTrigger
            value="info"
            className="h-auto flex-none rounded-md border-0 px-4 pb-3 pt-0 text-sm font-medium focus-visible:ring-0 focus-visible:outline-none data-[state=active]:bg-muted/60 data-[state=active]:shadow-none after:bg-primary"
          >
            {t("license:products.basicInfo")}
          </TabsTrigger>
          <TabsTrigger
            value="schema"
            className="h-auto flex-none rounded-md border-0 px-4 pb-3 pt-0 text-sm font-medium focus-visible:ring-0 focus-visible:outline-none data-[state=active]:bg-muted/60 data-[state=active]:shadow-none after:bg-primary"
          >
            {t("license:products.constraintDef")}
          </TabsTrigger>
          <TabsTrigger
            value="plans"
            className="h-auto flex-none rounded-md border-0 px-4 pb-3 pt-0 text-sm font-medium focus-visible:ring-0 focus-visible:outline-none data-[state=active]:bg-muted/60 data-[state=active]:shadow-none after:bg-primary"
          >
            {t("license:products.planManagement")}
          </TabsTrigger>
          <TabsTrigger
            value="keys"
            className="h-auto flex-none rounded-md border-0 px-4 pb-3 pt-0 text-sm font-medium focus-visible:ring-0 focus-visible:outline-none data-[state=active]:bg-muted/60 data-[state=active]:shadow-none after:bg-primary"
          >
            {t("license:products.keyManagement")}
          </TabsTrigger>
        </TabsList>

        <TabsContent value="info" className="space-y-4">
          <section className="space-y-3">
            <div className="grid gap-x-6 gap-y-4 sm:grid-cols-2 xl:grid-cols-4">
              <div className="space-y-1">
                <p className="text-sm text-muted-foreground">{t("common:name")}</p>
                <p className="font-medium text-foreground">{product.name}</p>
              </div>
              <div className="space-y-1">
                <p className="text-sm text-muted-foreground">{t("license:products.code")}</p>
                <code className="text-sm font-mono text-foreground">{product.code}</code>
              </div>
              <div className="space-y-1">
                <p className="text-sm text-muted-foreground">{t("license:products.licenseModules")}</p>
                <p className="font-medium tabular-nums text-foreground">{modules.length}</p>
              </div>
              <div className="space-y-1">
                <p className="text-sm text-muted-foreground">{t("license:products.planQuantity")}</p>
                <p className="font-medium tabular-nums text-foreground">{product.planCount}</p>
              </div>
              <div className="space-y-1">
                <p className="text-sm text-muted-foreground">{t("common:createdAt")}</p>
                <p className="text-sm text-foreground">{formatDateTime(product.createdAt)}</p>
              </div>
              <div className="space-y-1">
                <p className="text-sm text-muted-foreground">{t("common:updatedAt")}</p>
                <p className="text-sm text-foreground">{formatDateTime(product.updatedAt)}</p>
              </div>
            </div>
          </section>

          {product.description && (
            <>
              <Separator />
              <section className="space-y-2">
                <h3 className="text-sm font-medium text-muted-foreground">{t("common:description")}</h3>
                <p className="text-sm leading-6 text-foreground/90">{product.description}</p>
              </section>
            </>
          )}
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
          {product.licenseKey && (
            <section className="space-y-2.5">
              <div className="flex items-start justify-between gap-3">
                <div className="space-y-1">
                  <h3 className="text-sm font-medium text-foreground">{t("license:licenses.licenseKey")}</h3>
                  <p className="text-xs text-muted-foreground">{t("license:products.licenseKeyHint", "Use this key to issue licenses for this product.")}</p>
                </div>
                <Button
                  type="button"
                  size="sm"
                  variant={copiedField === "licenseKey" ? "default" : "outline"}
                  className="shrink-0"
                  onClick={() => handleCopy(product.licenseKey, "licenseKey")}
                >
                  {copiedField === "licenseKey" ? (
                    <>
                      <Check className="mr-2 h-3.5 w-3.5" />
                      {t("common:copied")}
                    </>
                  ) : (
                    <>
                      <Copy className="mr-2 h-3.5 w-3.5" />
                      {t("common:copy")}
                    </>
                  )}
                </Button>
              </div>
              <div className="rounded-md border bg-muted/20 px-3 py-2.5">
                <code className="block text-xs font-mono leading-6 text-foreground break-all">
                  {product.licenseKey}
                </code>
              </div>
            </section>
          )}

          <Separator />

          <section className="space-y-3">
            <div className="flex items-center justify-between">
              <div className="space-y-1">
                <h3 className="text-sm font-medium text-foreground">{t("license:products.currentKey")}</h3>
                {publicKey ? (
                  <p className="text-xs text-muted-foreground">
                    {t("common:createdAt")} {formatDateTime(publicKey.createdAt)}
                  </p>
                ) : (
                  <p className="text-xs text-muted-foreground">{t("license:products.noKeyInfo")}</p>
                )}
              </div>
              {publicKey && (
                <Badge variant="secondary" className="px-2.5 py-0.5 text-xs font-medium">v{publicKey.version}</Badge>
              )}
            </div>

            {publicKey && (
              <div className="space-y-2.5">
                <div className="flex items-center justify-between gap-3">
                  <h4 className="text-xs font-medium uppercase tracking-wide text-muted-foreground">{t("license:products.publicKey")}</h4>
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    className="h-8 px-3"
                    onClick={() => handleCopy(publicKey.publicKey, "publicKey")}
                  >
                    {copiedField === "publicKey" ? (
                      <>
                        <Check className="mr-2 h-3.5 w-3.5" />
                        {t("common:copied")}
                      </>
                    ) : (
                      <>
                        <Copy className="mr-2 h-3.5 w-3.5" />
                        {t("common:copy")}
                      </>
                    )}
                  </Button>
                </div>
                <div className="rounded-md border bg-muted/20 px-3 py-2.5">
                  <pre className="text-xs break-all whitespace-pre-wrap font-mono leading-6 text-foreground">
                    {publicKey.publicKey}
                  </pre>
                </div>
              </div>
            )}
            {canManageKey && (
              <div className="flex flex-wrap items-center gap-2 pt-1">
                <Button variant="outline" onClick={handleRotateKeyClick}>
                  <RefreshCw className="mr-2 h-4 w-4" />
                  {t("license:products.rotateKey")}
                </Button>
                {impact && impact.affectedCount > 0 && (
                  <Button variant="secondary" onClick={() => setBulkReissueOpen(true)}>
                    {t("license:products.bulkReissue")}
                  </Button>
                )}
              </div>
            )}
          </section>
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

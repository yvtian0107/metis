import { useState } from "react"
import { useParams, useNavigate, useSearchParams } from "react-router"
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { useTranslation } from "react-i18next"
import {
  ArrowLeft,
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
import { formatDateTime } from "@/lib/utils"
import { ProductSheet, type ProductItem } from "../../components/product-sheet"
import { ConstraintEditor } from "../../components/constraint-editor"
import { PlanTab } from "../../components/plan-tab"

const STATUS_VARIANTS: Record<string, "default" | "secondary" | "outline"> = {
  unpublished: "secondary",
  published: "default",
  archived: "outline",
}

const STATUS_ACTION_KEYS: Record<string, Array<{ status: string; labelKey: string }>> = {
  unpublished: [
    { status: "published", labelKey: "status.publish" },
    { status: "archived", labelKey: "status.archiveAction" },
  ],
  published: [
    { status: "unpublished", labelKey: "status.unpublish" },
    { status: "archived", labelKey: "status.archiveAction" },
  ],
  archived: [
    { status: "unpublished", labelKey: "status.restoreAction" },
  ],
}

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

export function Component() {
  const { t } = useTranslation(["license", "common"])
  const { id } = useParams()
  const navigate = useNavigate()
  const [searchParams, setSearchParams] = useSearchParams()
  const queryClient = useQueryClient()
  const [editOpen, setEditOpen] = useState(false)

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
      toast.success(t("license:products.statusUpdateSuccess"))
    },
    onError: (err) => toast.error(err.message),
  })

  const { data: publicKey } = useQuery({
    queryKey: ["license-product-key", id],
    queryFn: () => api.get<PublicKeyInfo>(`/api/v1/license/products/${id}/public-key`),
    enabled: !!id,
  })

  const rotateKeyMutation = useMutation({
    mutationFn: () => api.post(`/api/v1/license/products/${id}/rotate-key`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["license-product-key", id] })
      toast.success(t("license:products.rotateKeySuccess"))
    },
    onError: (err) => toast.error(err.message),
  })

  const modules = Array.isArray(product?.constraintSchema) ? product.constraintSchema : []
  const hasSchema = modules.length > 0
  const hasPlans = (product?.planCount ?? 0) > 0
  const requestedTab = searchParams.get("tab")
  const activeTab =
    requestedTab === "info" || requestedTab === "schema" || requestedTab === "plans" || requestedTab === "keys"
      ? requestedTab
      : !hasSchema
        ? "schema"
        : !hasPlans
          ? "plans"
          : "info"

  if (isLoading || !product) {
    return (
      <div className="flex min-h-[200px] items-center justify-center text-muted-foreground">
        {t("common:loading")}
      </div>
    )
  }

  const variant = STATUS_VARIANTS[product.status] ?? ("secondary" as const)
  const statusKey = product.status as string
  const actions = STATUS_ACTION_KEYS[product.status] ?? []

  function handleTabChange(value: string) {
    const nextParams = new URLSearchParams(searchParams)
    nextParams.set("tab", value)
    setSearchParams(nextParams, { replace: true })
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
      <div className="flex items-center gap-3">
        <Button variant="ghost" size="icon" onClick={() => navigate("/license/products")}>
          <ArrowLeft className="h-4 w-4" />
        </Button>
        <div className="flex-1">
          <div className="flex items-center gap-2">
            <h2 className="text-lg font-semibold">{product.name}</h2>
            <Badge variant={variant}>{t(`license:status.${statusKey}`, product.status)}</Badge>
          </div>
          <p className="text-sm text-muted-foreground font-mono">{product.code}</p>
        </div>
        {canUpdate && (
          <Button variant="outline" size="sm" onClick={() => setEditOpen(true)}>
            <Pencil className="mr-1.5 h-3.5 w-3.5" />
            {t("common:edit")}
          </Button>
        )}
      </div>

      <Tabs value={activeTab} onValueChange={handleTabChange}>
        <TabsList className="h-auto w-fit max-w-full flex-wrap justify-start gap-1 rounded-lg bg-muted/50 p-1">
          <TabsTrigger value="info" className="h-8 flex-none px-3 text-xs sm:text-sm">
            {t("license:products.basicInfo")}
          </TabsTrigger>
          <TabsTrigger value="schema" className="h-8 flex-none px-3 text-xs sm:text-sm">
            {t("license:products.constraintDef")}
          </TabsTrigger>
          <TabsTrigger value="plans" className="h-8 flex-none px-3 text-xs sm:text-sm">
            {t("license:products.planManagement")}
          </TabsTrigger>
          <TabsTrigger value="keys" className="h-8 flex-none px-3 text-xs sm:text-sm">
            {t("license:products.keyManagement")}
          </TabsTrigger>
        </TabsList>

        <TabsContent value="info" className="space-y-4">
          <div className="rounded-lg border">
            <div className="grid gap-x-6 gap-y-4 px-4 py-4 text-sm sm:grid-cols-2 lg:grid-cols-3">
              <div>
                <p className="text-muted-foreground">{t("common:name")}</p>
                <p className="mt-1 font-medium">{product.name}</p>
              </div>
              <div>
                <p className="text-muted-foreground">{t("license:products.code")}</p>
                <p className="mt-1 font-mono">{product.code}</p>
              </div>
              <div>
                <p className="text-muted-foreground">{t("common:status")}</p>
                <div className="mt-1">
                  <Badge variant={variant}>{t(`license:status.${statusKey}`, product.status)}</Badge>
                </div>
              </div>
              <div>
                <p className="text-muted-foreground">{t("license:products.licenseModules")}</p>
                <p className="mt-1">{modules.length}</p>
              </div>
              <div>
                <p className="text-muted-foreground">{t("license:products.planQuantity")}</p>
                <p className="mt-1">{product.planCount}</p>
              </div>
              <div>
                <p className="text-muted-foreground">{t("common:updatedAt")}</p>
                <p className="mt-1">{formatDateTime(product.updatedAt)}</p>
              </div>
              <div className="sm:col-span-2 lg:col-span-3">
                <p className="text-muted-foreground">{t("common:description")}</p>
                <p className="mt-1 leading-6">
                  {product.description || t("license:products.noDescription")}
                </p>
              </div>
            </div>
          </div>

          <div className="flex flex-wrap items-center gap-2">
            <Button
              variant="outline"
              size="sm"
              onClick={() => handleTabChange(hasSchema ? (hasPlans ? "keys" : "plans") : "schema")}
            >
              {!hasSchema ? t("license:products.constraintDef") : !hasPlans ? t("license:products.planManagement") : t("license:products.keyManagement")}
            </Button>
            {canUpdate && actions.length > 0 && (
              <>
                {actions.map((action) => {
                  const actionLabel = t(`license:${action.labelKey}`)
                  return (
                    <AlertDialog key={action.status}>
                      <AlertDialogTrigger asChild>
                        <Button variant="outline" size="sm">
                          {actionLabel}
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
              </>
            )}
          </div>
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
          <div className="rounded-lg border p-4">
            {publicKey ? (
              <div className="space-y-4 text-sm">
                <div className="flex items-center gap-2">
                  <Key className="h-4 w-4 text-muted-foreground" />
                  <span className="font-medium">{t("license:products.currentKey")}</span>
                  <Badge variant="secondary">v{publicKey.version}</Badge>
                </div>
                <div>
                  <p className="text-muted-foreground">{t("license:products.publicKey")}</p>
                  <pre className="mt-1 rounded bg-muted p-3 text-xs break-all whitespace-pre-wrap font-mono">
                    {publicKey.publicKey}
                  </pre>
                </div>
                <div>
                  <p className="text-muted-foreground">{t("common:createdAt")}</p>
                  <p className="mt-1">{formatDateTime(publicKey.createdAt)}</p>
                </div>
              </div>
            ) : (
              <p className="text-sm text-muted-foreground">{t("license:products.noKeyInfo")}</p>
            )}
          </div>

          {canManageKey && (
            <AlertDialog>
              <AlertDialogTrigger asChild>
                <Button variant="outline" size="sm">
                  <RefreshCw className="mr-1.5 h-3.5 w-3.5" />
                  {t("license:products.rotateKey")}
                </Button>
              </AlertDialogTrigger>
              <AlertDialogContent>
                <AlertDialogHeader>
                  <AlertDialogTitle>{t("license:products.confirmRotateKey")}</AlertDialogTitle>
                  <AlertDialogDescription>
                    {t("license:products.rotateKeyDesc")}
                  </AlertDialogDescription>
                </AlertDialogHeader>
                <AlertDialogFooter>
                  <AlertDialogCancel>{t("common:cancel")}</AlertDialogCancel>
                  <AlertDialogAction
                    onClick={() => rotateKeyMutation.mutate()}
                    disabled={rotateKeyMutation.isPending}
                  >
                    {rotateKeyMutation.isPending ? (
                      <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />
                    ) : null}
                    {t("license:products.confirmRotate")}
                  </AlertDialogAction>
                </AlertDialogFooter>
              </AlertDialogContent>
            </AlertDialog>
          )}
        </TabsContent>
      </Tabs>

      <ProductSheet open={editOpen} onOpenChange={setEditOpen} product={editableProduct} />
    </div>
  )
}

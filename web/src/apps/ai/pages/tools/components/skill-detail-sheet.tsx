import { useEffect } from "react"
import { useForm, useWatch } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { api } from "@/lib/api"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { Textarea } from "@/components/ui/textarea"
import { Badge } from "@/components/ui/badge"
import { Separator } from "@/components/ui/separator"
import {
  Sheet, SheetContent, SheetHeader, SheetTitle, SheetDescription, SheetFooter,
} from "@/components/ui/sheet"
import {
  Form, FormControl, FormField, FormItem, FormLabel, FormMessage,
} from "@/components/ui/form"
import {
  Select, SelectContent, SelectItem, SelectTrigger, SelectValue,
} from "@/components/ui/select"
import { Label } from "@/components/ui/label"

export interface SkillItem {
  id: number
  name: string
  displayName: string
  description: string
  sourceType: string
  sourceUrl: string
  manifest: unknown
  instructions: string
  toolsSchema: unknown
  toolCount: number
  hasInstructions: boolean
  authType: string
  isActive: boolean
  createdAt: string
  updatedAt: string
}

const AUTH_TYPES = ["none", "api_key", "bearer", "oauth", "custom_header"] as const

interface SkillDetailSheetProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  skill: SkillItem | null
}

interface AuthFormValues {
  authType: string
  authConfig: string
}

export function SkillDetailSheet({ open, onOpenChange, skill }: SkillDetailSheetProps) {
  const { t } = useTranslation(["ai", "common"])
  const queryClient = useQueryClient()

  const form = useForm<AuthFormValues>({
    defaultValues: { authType: "none", authConfig: "" },
  })

  // Fetch full detail when opened
  const { data: detail } = useQuery({
    queryKey: ["ai-skill-detail", skill?.id],
    queryFn: () => api.get<SkillItem>(`/api/v1/ai/skills/${skill!.id}`),
    enabled: open && skill !== null,
  })
  const activeDetail = detail ?? skill

  useEffect(() => {
    if (open && activeDetail) {
      form.reset({
        authType: activeDetail.authType || "none",
        authConfig: "",
      })
    }
  }, [open, activeDetail, form])

  const authType = useWatch({ control: form.control, name: "authType" })

  const updateMutation = useMutation({
    mutationFn: (payload: AuthFormValues) =>
      api.put(`/api/v1/ai/skills/${skill!.id}`, payload),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["ai-skills"] })
      queryClient.invalidateQueries({ queryKey: ["ai-skill-detail", skill?.id] })
      onOpenChange(false)
      toast.success(t("ai:tools.skills.updateSuccess"))
    },
    onError: (err) => toast.error(err.message),
  })

  function onSubmit(values: AuthFormValues) {
    updateMutation.mutate(values)
  }

  if (!activeDetail) return null

  const toolsDefs = activeDetail.toolsSchema
  const toolsFormatted = toolsDefs ? JSON.stringify(toolsDefs, null, 2) : ""

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className="sm:max-w-xl overflow-y-auto">
        <SheetHeader>
          <SheetTitle>{t("ai:tools.skills.detail")}</SheetTitle>
          <SheetDescription className="sr-only">
            {t("ai:tools.skills.detail")}
          </SheetDescription>
        </SheetHeader>
        <div className="flex flex-1 flex-col gap-5 px-4">
          {/* Basic info */}
          <div className="space-y-2">
            <div className="flex items-center gap-2">
              <span className="text-lg font-semibold">{activeDetail.displayName}</span>
              <Badge variant="outline">
                {t(`ai:tools.skills.sourceTypes.${activeDetail.sourceType}`, activeDetail.sourceType)}
              </Badge>
            </div>
            <p className="text-sm text-muted-foreground font-mono">{activeDetail.name}</p>
            {activeDetail.description && (
              <p className="text-sm text-muted-foreground">{activeDetail.description}</p>
            )}
            {activeDetail.sourceUrl && (
              <p className="text-xs text-muted-foreground break-all">{activeDetail.sourceUrl}</p>
            )}
          </div>

          <Separator />

          {/* Instructions */}
          {activeDetail.instructions && (
            <div className="space-y-1.5">
              <Label className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                {t("ai:tools.skills.instructions")}
              </Label>
              <div className="rounded-md border bg-muted/30 p-3 text-sm whitespace-pre-wrap max-h-48 overflow-y-auto">
                {activeDetail.instructions}
              </div>
            </div>
          )}

          {/* Tools schema */}
          {toolsFormatted && (
            <div className="space-y-1.5">
              <Label className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                {t("ai:tools.skills.toolsSchema")} ({activeDetail.toolCount})
              </Label>
              <div className="rounded-md border bg-muted/30 p-3 text-xs font-mono whitespace-pre-wrap max-h-64 overflow-y-auto">
                {toolsFormatted}
              </div>
            </div>
          )}

          <Separator />

          {/* Auth config edit */}
          <Form {...form}>
            <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-3">
              <Label className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                {t("ai:tools.skills.editAuth")}
              </Label>
              <FormField
                control={form.control}
                name="authType"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t("ai:tools.skills.authType")}</FormLabel>
                    <Select value={field.value} onValueChange={field.onChange}>
                      <FormControl>
                        <SelectTrigger>
                          <SelectValue />
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent>
                        {AUTH_TYPES.map((at) => (
                          <SelectItem key={at} value={at}>
                            {t(`ai:tools.mcp.authTypes.${at}`)}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                    <FormMessage />
                  </FormItem>
                )}
              />
              {authType !== "none" && (
                <FormField
                  control={form.control}
                  name="authConfig"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t("ai:tools.skills.authConfig")}</FormLabel>
                      <FormControl>
                        <Textarea
                          placeholder={t("ai:tools.skills.authConfigHint")}
                          rows={4}
                          className="font-mono text-xs"
                          {...field}
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              )}
              <SheetFooter>
                <Button
                  type="submit"
                  size="sm"
                  disabled={updateMutation.isPending}
                >
                  {updateMutation.isPending ? t("common:saving") : t("common:save")}
                </Button>
              </SheetFooter>
            </form>
          </Form>
        </div>
      </SheetContent>
    </Sheet>
  )
}

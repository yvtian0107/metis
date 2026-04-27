import { useEffect, useState } from "react"
import { useForm, useWatch } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { z } from "zod"
import { zodResolver } from "@/lib/form"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { api } from "@/lib/api"
import { toast } from "sonner"
import { ChevronDown, ChevronRight } from "lucide-react"
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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"

export interface ProcessDefItem {
  id: number
  name: string
  displayName: string
  description: string
  startCommand: string
  stopCommand: string
  reloadCommand: string
  env: Record<string, string> | null
  configFiles: unknown[] | null
  probeType: string
  probeConfig: { type?: string; endpoint?: string; command?: string; timeout?: number; interval?: number } | null
  restartPolicy: string
  maxRestarts: number
  createdAt: string
  updatedAt: string
}

function useProcessDefSchema() {
  const { t } = useTranslation("node")
  return z.object({
    name: z.string().min(1, t("validation.nameRequired")).max(128),
    displayName: z.string().min(1, t("validation.displayNameRequired")).max(128),
    description: z.string().optional(),
    startCommand: z.string().min(1, t("validation.startCommandRequired")).max(512),
    stopCommand: z.string().max(512).optional(),
    reloadCommand: z.string().max(512).optional(),
    env: z.string().optional(),
    configFiles: z.string().optional(),
    restartPolicy: z.string(),
    maxRestarts: z.coerce.number().int().min(0).max(100),
    probeType: z.string(),
    probeEndpoint: z.string().optional(),
    probeCommand: z.string().optional(),
    probeTimeout: z.coerce.number().int().min(1).max(300).optional(),
    probeInterval: z.coerce.number().int().min(5).max(3600).optional(),
  })
}

type FormValues = z.infer<ReturnType<typeof useProcessDefSchema>>

interface ProcessDefSheetProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  processDef: ProcessDefItem | null
}

export function ProcessDefSheet({ open, onOpenChange, processDef }: ProcessDefSheetProps) {
  const { t } = useTranslation(["node", "common"])
  const queryClient = useQueryClient()
  const isEditing = processDef !== null
  const schema = useProcessDefSchema()
  const [showAdvanced, setShowAdvanced] = useState(false)

  const form = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: {
      name: "",
      displayName: "",
      description: "",
      startCommand: "",
      stopCommand: "",
      reloadCommand: "",
      env: "",
      configFiles: "",
      restartPolicy: "always",
      maxRestarts: 10,
      probeType: "none",
      probeEndpoint: "",
      probeCommand: "",
      probeTimeout: 5,
      probeInterval: 30,
    },
  })

  useEffect(() => {
    if (open) {
      if (processDef) {
        const pc = processDef.probeConfig
        form.reset({
          name: processDef.name,
          displayName: processDef.displayName,
          description: processDef.description || "",
          startCommand: processDef.startCommand,
          stopCommand: processDef.stopCommand || "",
          reloadCommand: processDef.reloadCommand || "",
          env: processDef.env ? JSON.stringify(processDef.env, null, 2) : "",
          configFiles: processDef.configFiles ? JSON.stringify(processDef.configFiles, null, 2) : "",
          restartPolicy: processDef.restartPolicy,
          maxRestarts: processDef.maxRestarts,
          probeType: processDef.probeType,
          probeEndpoint: pc?.endpoint || "",
          probeCommand: pc?.command || "",
          probeTimeout: pc?.timeout || 5,
          probeInterval: pc?.interval || 30,
        })
      } else {
        form.reset()
      }
    }
  }, [open, processDef, form])

  function buildBody(values: FormValues) {
    const body: Record<string, unknown> = {
      name: values.name,
      displayName: values.displayName,
      description: values.description,
      startCommand: values.startCommand,
      stopCommand: values.stopCommand,
      reloadCommand: values.reloadCommand,
      restartPolicy: values.restartPolicy,
      maxRestarts: values.maxRestarts,
      probeType: values.probeType,
    }
    if (values.env?.trim()) body.env = JSON.parse(values.env)
    if (values.configFiles?.trim()) body.configFiles = JSON.parse(values.configFiles)

    if (values.probeType && values.probeType !== "none") {
      const probeConfig: Record<string, unknown> = {
        type: values.probeType,
        timeout: values.probeTimeout || 5,
        interval: values.probeInterval || 30,
      }
      if (values.probeType === "http" || values.probeType === "tcp") {
        probeConfig.endpoint = values.probeEndpoint || ""
      }
      if (values.probeType === "exec") {
        probeConfig.command = values.probeCommand || ""
      }
      body.probeConfig = probeConfig
    } else {
      body.probeConfig = {}
    }

    return body
  }

  const createMutation = useMutation({
    mutationFn: (values: FormValues) =>
      api.post("/api/v1/process-defs", buildBody(values)),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["process-defs"] })
      onOpenChange(false)
      toast.success(t("node:processDefs.createSuccess"))
    },
    onError: (err) => toast.error(err.message),
  })

  const updateMutation = useMutation({
    mutationFn: (values: FormValues) =>
      api.put(`/api/v1/process-defs/${processDef!.id}`, buildBody(values)),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["process-defs"] })
      onOpenChange(false)
      toast.success(t("node:processDefs.updateSuccess"))
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
  const probeType = useWatch({ control: form.control, name: "probeType" })

  function handleOpenChange(nextOpen: boolean) {
    if (!nextOpen) setShowAdvanced(false)
    onOpenChange(nextOpen)
  }

  return (
    <Sheet open={open} onOpenChange={handleOpenChange}>
      <SheetContent className="sm:max-w-lg overflow-y-auto">
        <SheetHeader>
          <SheetTitle>
            {isEditing ? t("node:processDefs.editProcessDef") : t("node:processDefs.create")}
          </SheetTitle>
          <SheetDescription className="sr-only">
            {isEditing ? t("node:processDefs.editProcessDef") : t("node:processDefs.create")}
          </SheetDescription>
        </SheetHeader>
        <Form {...form}>
          <form onSubmit={form.handleSubmit(onSubmit)} className="flex flex-1 flex-col gap-5 px-4">
            <FormField
              control={form.control}
              name="name"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t("node:processDefs.name")}</FormLabel>
                  <FormControl>
                    <Input placeholder={t("node:processDefs.namePlaceholder")} {...field} disabled={isEditing} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name="displayName"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t("node:processDefs.displayName")}</FormLabel>
                  <FormControl>
                    <Input placeholder={t("node:processDefs.displayNamePlaceholder")} {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name="description"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t("common:description")}</FormLabel>
                  <FormControl>
                    <Textarea placeholder={t("node:processDefs.descriptionPlaceholder")} rows={2} {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name="startCommand"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t("node:processDefs.startCommand")}</FormLabel>
                  <FormControl>
                    <Input placeholder={t("node:processDefs.startCommandPlaceholder")} {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            <div className="flex items-center gap-2">
              <FormField
                control={form.control}
                name="restartPolicy"
                render={({ field }) => (
                  <FormItem className="flex-1">
                    <FormLabel>{t("node:processDefs.restartPolicy")}</FormLabel>
                    <Select value={field.value} onValueChange={field.onChange}>
                      <FormControl>
                        <SelectTrigger>
                          <SelectValue />
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent>
                        <SelectItem value="always">{t("node:processDefs.always")}</SelectItem>
                        <SelectItem value="on_failure">{t("node:processDefs.on_failure")}</SelectItem>
                        <SelectItem value="never">{t("node:processDefs.never")}</SelectItem>
                      </SelectContent>
                    </Select>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name="probeType"
                render={({ field }) => (
                  <FormItem className="flex-1">
                    <FormLabel>{t("node:processDefs.probeType")}</FormLabel>
                    <Select value={field.value} onValueChange={field.onChange}>
                      <FormControl>
                        <SelectTrigger>
                          <SelectValue />
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent>
                        <SelectItem value="none">{t("node:processDefs.none")}</SelectItem>
                        <SelectItem value="http">{t("node:processDefs.http")}</SelectItem>
                        <SelectItem value="tcp">{t("node:processDefs.tcp")}</SelectItem>
                        <SelectItem value="exec">{t("node:processDefs.exec")}</SelectItem>
                      </SelectContent>
                    </Select>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>

            {probeType !== "none" && (
              <div className="rounded-md border p-3 space-y-3">
                <p className="text-sm font-medium">{t("node:processDefs.probeConfig")}</p>
                {(probeType === "http" || probeType === "tcp") && (
                  <FormField
                    control={form.control}
                    name="probeEndpoint"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t("node:processDefs.probeEndpoint")}</FormLabel>
                        <FormControl>
                          <Input
                            placeholder={
                              probeType === "http"
                                ? t("node:processDefs.probeEndpointHttpPlaceholder")
                                : t("node:processDefs.probeEndpointTcpPlaceholder")
                            }
                            {...field}
                          />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                )}
                {probeType === "exec" && (
                  <FormField
                    control={form.control}
                    name="probeCommand"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t("node:processDefs.probeCommand")}</FormLabel>
                        <FormControl>
                          <Input placeholder={t("node:processDefs.probeCommandPlaceholder")} {...field} />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                )}
                <div className="flex items-center gap-2">
                  <FormField
                    control={form.control}
                    name="probeTimeout"
                    render={({ field }) => (
                      <FormItem className="flex-1">
                        <FormLabel>{t("node:processDefs.probeTimeout")}</FormLabel>
                        <FormControl>
                          <Input type="number" {...field} />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                  <FormField
                    control={form.control}
                    name="probeInterval"
                    render={({ field }) => (
                      <FormItem className="flex-1">
                        <FormLabel>{t("node:processDefs.probeInterval")}</FormLabel>
                        <FormControl>
                          <Input type="number" {...field} />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                </div>
              </div>
            )}

            <button
              type="button"
              className="flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground"
              onClick={() => setShowAdvanced(!showAdvanced)}
            >
              {showAdvanced ? <ChevronDown className="h-4 w-4" /> : <ChevronRight className="h-4 w-4" />}
              {t("node:processDefs.advancedSettings")}
            </button>

            {showAdvanced && (
              <>
                <FormField
                  control={form.control}
                  name="stopCommand"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t("node:processDefs.stopCommand")}</FormLabel>
                      <FormControl>
                        <Input placeholder={t("node:processDefs.stopCommandPlaceholder")} {...field} />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name="reloadCommand"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t("node:processDefs.reloadCommand")}</FormLabel>
                      <FormControl>
                        <Input placeholder={t("node:processDefs.reloadCommandPlaceholder")} {...field} />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name="env"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>Env</FormLabel>
                      <FormControl>
                        <Textarea
                          placeholder={t("node:processDefs.envPlaceholder")}
                          rows={3}
                          className="font-mono text-sm"
                          {...field}
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name="configFiles"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>Config Files</FormLabel>
                      <FormControl>
                        <Textarea
                          placeholder={t("node:processDefs.configFilesPlaceholder")}
                          rows={4}
                          className="font-mono text-sm"
                          {...field}
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name="maxRestarts"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t("node:processDefs.maxRestarts")}</FormLabel>
                      <FormControl>
                        <Input type="number" {...field} />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <p className="text-xs text-muted-foreground">
                  {t("node:processDefs.commandHint")}
                </p>
              </>
            )}

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

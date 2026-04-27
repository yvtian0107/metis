import { useEffect } from "react"
import { useForm, useWatch } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { z } from "zod"
import { zodResolver } from "@hookform/resolvers/zod"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { api } from "@/lib/api"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Textarea } from "@/components/ui/textarea"
import {
  Sheet, SheetContent, SheetHeader, SheetTitle, SheetDescription, SheetFooter,
} from "@/components/ui/sheet"
import {
  Form, FormControl, FormField, FormItem, FormLabel, FormMessage,
} from "@/components/ui/form"
import {
  Select, SelectContent, SelectItem, SelectTrigger, SelectValue,
} from "@/components/ui/select"

export interface MCPServerItem {
  id: number
  name: string
  description: string
  transport: string
  url: string
  command: string
  args: unknown[] | null
  env: Record<string, string> | null
  authType: string
  authMasked: string
  isActive: boolean
  createdAt: string
  updatedAt: string
}

const TRANSPORTS = ["sse", "stdio"] as const
const AUTH_TYPES = ["none", "api_key", "bearer", "oauth", "custom_header"] as const

function useMCPServerSchema() {
  const { t } = useTranslation("ai")
  return z.object({
    name: z.string().min(1, t("validation.nameRequired")).max(128),
    description: z.string().max(1000).optional(),
    transport: z.enum(TRANSPORTS),
    url: z.string().max(512).optional(),
    command: z.string().max(256).optional(),
    argsText: z.string().optional(),
    envText: z.string().optional(),
    authType: z.enum(AUTH_TYPES),
    authConfig: z.string().optional(),
  }).superRefine((data, ctx) => {
    if (data.transport === "sse" && !data.url) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: t("validation.urlRequired"),
        path: ["url"],
      })
    }
    if (data.transport === "stdio" && !data.command) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: t("validation.commandRequired"),
        path: ["command"],
      })
    }
  })
}

type FormValues = z.infer<ReturnType<typeof useMCPServerSchema>>

interface MCPServerSheetProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  server: MCPServerItem | null
}

function argsToText(args: unknown[] | null): string {
  if (!args || !Array.isArray(args)) return ""
  return args.map(String).join("\n")
}

function envToText(env: Record<string, string> | null): string {
  if (!env || typeof env !== "object") return ""
  return Object.entries(env).map(([k, v]) => `${k}=${v}`).join("\n")
}

function textToArgs(text: string): string[] {
  return text.split("\n").map((s) => s.trim()).filter(Boolean)
}

function textToEnv(text: string): Record<string, string> {
  const env: Record<string, string> = {}
  for (const line of text.split("\n")) {
    const trimmed = line.trim()
    if (!trimmed) continue
    const idx = trimmed.indexOf("=")
    if (idx > 0) {
      env[trimmed.slice(0, idx)] = trimmed.slice(idx + 1)
    }
  }
  return env
}

export function MCPServerSheet({ open, onOpenChange, server }: MCPServerSheetProps) {
  const { t } = useTranslation(["ai", "common"])
  const queryClient = useQueryClient()
  const isEditing = server !== null
  const schema = useMCPServerSchema()

  const form = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: {
      name: "",
      description: "",
      transport: "sse",
      url: "",
      command: "",
      argsText: "",
      envText: "",
      authType: "none",
      authConfig: "",
    },
  })

  const watchedTransport = useWatch({ control: form.control, name: "transport" })
  const watchedAuthType = useWatch({ control: form.control, name: "authType" })

  useEffect(() => {
    if (open) {
      if (server) {
        form.reset({
          name: server.name,
          description: server.description || "",
          transport: server.transport as "sse" | "stdio",
          url: server.url || "",
          command: server.command || "",
          argsText: argsToText(server.args),
          envText: envToText(server.env),
          authType: server.authType as (typeof AUTH_TYPES)[number],
          authConfig: "",
        })
      } else {
        form.reset({
          name: "",
          description: "",
          transport: "sse",
          url: "",
          command: "",
          argsText: "",
          envText: "",
          authType: "none",
          authConfig: "",
        })
      }
    }
  }, [open, server, form])

  const createMutation = useMutation({
    mutationFn: (payload: Record<string, unknown>) =>
      api.post("/api/v1/ai/mcp-servers", payload),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["ai-mcp-servers"] })
      onOpenChange(false)
      toast.success(t("ai:tools.mcp.createSuccess"))
    },
    onError: (err) => toast.error(err.message),
  })

  const updateMutation = useMutation({
    mutationFn: (payload: Record<string, unknown>) =>
      api.put(`/api/v1/ai/mcp-servers/${server!.id}`, payload),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["ai-mcp-servers"] })
      onOpenChange(false)
      toast.success(t("ai:tools.mcp.updateSuccess"))
    },
    onError: (err) => toast.error(err.message),
  })

  function onSubmit(values: FormValues) {
    const payload: Record<string, unknown> = {
      name: values.name,
      description: values.description || "",
      transport: values.transport,
      authType: values.authType,
    }
    if (values.transport === "sse") {
      payload.url = values.url
    } else {
      payload.command = values.command
      const args = textToArgs(values.argsText || "")
      if (args.length > 0) payload.args = args
      const env = textToEnv(values.envText || "")
      if (Object.keys(env).length > 0) payload.env = env
    }
    if (values.authConfig) {
      payload.authConfig = values.authConfig
    }
    if (isEditing) {
      updateMutation.mutate(payload)
    } else {
      createMutation.mutate(payload)
    }
  }

  const isPending = createMutation.isPending || updateMutation.isPending

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className="sm:max-w-lg overflow-y-auto">
        <SheetHeader>
          <SheetTitle>
            {isEditing ? t("ai:tools.mcp.edit") : t("ai:tools.mcp.create")}
          </SheetTitle>
          <SheetDescription className="sr-only">
            {isEditing ? t("ai:tools.mcp.edit") : t("ai:tools.mcp.create")}
          </SheetDescription>
        </SheetHeader>
        <Form {...form}>
          <form onSubmit={form.handleSubmit(onSubmit)} className="flex flex-1 flex-col gap-5 px-4">
            <FormField
              control={form.control}
              name="name"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t("ai:tools.mcp.name")}</FormLabel>
                  <FormControl>
                    <Input placeholder={t("ai:tools.mcp.namePlaceholder")} {...field} />
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
                  <FormLabel>{t("ai:tools.mcp.description")}</FormLabel>
                  <FormControl>
                    <Textarea placeholder={t("ai:tools.mcp.descriptionPlaceholder")} rows={2} {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name="transport"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t("ai:tools.mcp.transport")}</FormLabel>
                  <Select value={field.value} onValueChange={field.onChange}>
                    <FormControl>
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                    </FormControl>
                    <SelectContent>
                      {TRANSPORTS.map((tp) => (
                        <SelectItem key={tp} value={tp}>
                          {t(`ai:tools.mcp.transportTypes.${tp}`)}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                  <FormMessage />
                </FormItem>
              )}
            />
            {watchedTransport === "sse" && (
              <FormField
                control={form.control}
                name="url"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t("ai:tools.mcp.url")}</FormLabel>
                    <FormControl>
                      <Input placeholder={t("ai:tools.mcp.urlPlaceholder")} {...field} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            )}
            {watchedTransport === "stdio" && (
              <>
                <FormField
                  control={form.control}
                  name="command"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t("ai:tools.mcp.command")}</FormLabel>
                      <FormControl>
                        <Input placeholder={t("ai:tools.mcp.commandPlaceholder")} {...field} />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name="argsText"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t("ai:tools.mcp.args")}</FormLabel>
                      <FormControl>
                        <Textarea placeholder={t("ai:tools.mcp.argsPlaceholder")} rows={3} {...field} />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name="envText"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t("ai:tools.mcp.env")}</FormLabel>
                      <FormControl>
                        <Textarea placeholder={t("ai:tools.mcp.envPlaceholder")} rows={3} {...field} />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </>
            )}
            <FormField
              control={form.control}
              name="authType"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t("ai:tools.mcp.authType")}</FormLabel>
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
            {watchedAuthType !== "none" && (
              <FormField
                control={form.control}
                name="authConfig"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t("ai:tools.mcp.authConfig")}</FormLabel>
                    <FormControl>
                      <Textarea
                        placeholder={isEditing ? t("ai:tools.mcp.authConfigHint") : t("ai:tools.mcp.authConfigPlaceholder")}
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

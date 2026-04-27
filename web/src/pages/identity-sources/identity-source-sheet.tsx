import { useEffect } from "react"
import { useTranslation } from "react-i18next"
import { useForm, useWatch, type Resolver } from "react-hook-form"
import { z } from "zod"
import { zodResolver } from "@hookform/resolvers/zod"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { Loader2, Plug } from "lucide-react"
import { api } from "@/lib/api"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Checkbox } from "@/components/ui/checkbox"
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
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
  FormDescription,
} from "@/components/ui/form"

export interface IdentitySourceItem {
  id: number
  name: string
  type: string
  enabled: boolean
  domains: string
  forceSso: boolean
  defaultRoleId: number
  conflictStrategy: string
  config: Record<string, unknown>
  createdAt: string
  updatedAt: string
}

function createOidcConfigSchema(t: (key: string) => string) {
  return z.object({
    issuerUrl: z.string().min(1, t("validation.issuerUrlRequired")),
    clientId: z.string().min(1, t("validation.clientIdRequired")),
    clientSecret: z.string().optional().default(""),
    callbackUrl: z.string().optional().default(""),
    usePkce: z.boolean().default(true),
    scopes: z.string().optional().default("openid,profile,email"),
  })
}

function createLdapConfigSchema(t: (key: string) => string) {
  return z.object({
    serverUrl: z.string().min(1, t("validation.serverUrlRequired")),
    bindDn: z.string().min(1, t("validation.bindDnRequired")),
    bindPassword: z.string().optional().default(""),
    searchBase: z.string().min(1, t("validation.searchBaseRequired")),
    userFilter: z.string().optional().default("(uid={{username}})"),
    useTls: z.boolean().default(false),
    skipVerify: z.boolean().default(false),
  })
}

function createSchema(t: (key: string) => string) {
  return z.discriminatedUnion("type", [
    z.object({
      type: z.literal("oidc"),
      name: z.string().min(1, t("validation.nameRequired")),
      domains: z.string().min(1, t("validation.domainRequired")),
      forceSso: z.boolean().default(false),
      defaultRoleId: z.coerce.number().default(0),
      conflictStrategy: z.string().default("fail"),
      config: createOidcConfigSchema(t),
    }),
    z.object({
      type: z.literal("ldap"),
      name: z.string().min(1, t("validation.nameRequired")),
      domains: z.string().min(1, t("validation.domainRequired")),
      forceSso: z.boolean().default(false),
      defaultRoleId: z.coerce.number().default(0),
      conflictStrategy: z.string().default("fail"),
      config: createLdapConfigSchema(t),
    }),
  ])
}

type FormValues = z.infer<ReturnType<typeof createSchema>>

interface IdentitySourceSheetProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  source: IdentitySourceItem | null
}

const OIDC_DEFAULTS = {
  issuerUrl: "",
  clientId: "",
  clientSecret: "",
  callbackUrl: `${window.location.origin}/sso/callback`,
  usePkce: true,
  scopes: "openid,profile,email",
}

const LDAP_DEFAULTS = {
  serverUrl: "",
  bindDn: "",
  bindPassword: "",
  searchBase: "",
  userFilter: "(uid={{username}})",
  useTls: false,
  skipVerify: false,
}

export function IdentitySourceSheet({ open, onOpenChange, source }: IdentitySourceSheetProps) {
  const { t } = useTranslation(["identitySources", "common"])
  const queryClient = useQueryClient()
  const isEditing = source !== null

  const schema = createSchema(t)

  const form = useForm<FormValues>({
    resolver: zodResolver(schema) as Resolver<FormValues>,
    defaultValues: {
      type: "oidc",
      name: "",
      domains: "",
      forceSso: false,
      defaultRoleId: 0,
      conflictStrategy: "fail",
      config: OIDC_DEFAULTS,
    } as FormValues,
  })

  const selectedType = useWatch({ control: form.control, name: "type" })

  useEffect(() => {
    if (open) {
      if (source) {
        form.reset({
          type: source.type as "oidc" | "ldap",
          name: source.name,
          domains: source.domains,
          forceSso: source.forceSso,
          defaultRoleId: source.defaultRoleId,
          conflictStrategy: source.conflictStrategy,
          config: source.config as FormValues["config"],
        } as FormValues)
      } else {
        form.reset({
          type: "oidc",
          name: "",
          domains: "",
          forceSso: false,
          defaultRoleId: 0,
          conflictStrategy: "fail",
          config: OIDC_DEFAULTS,
        } as FormValues)
      }
    }
  }, [open, source, form])

  // Reset config defaults when type changes (only in create mode)
  useEffect(() => {
    if (!isEditing && open) {
      if (selectedType === "oidc") {
        form.setValue("config", OIDC_DEFAULTS)
      } else if (selectedType === "ldap") {
        form.setValue("config", LDAP_DEFAULTS)
      }
    }
  }, [selectedType, isEditing, open, form])

  const createMutation = useMutation({
    mutationFn: (values: FormValues) =>
      api.post("/api/v1/identity-sources", values),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["identity-sources"] })
      onOpenChange(false)
      toast.success(t("createSuccess"))
    },
    onError: (err) => toast.error(err.message),
  })

  const updateMutation = useMutation({
    mutationFn: (values: FormValues) =>
      api.put(`/api/v1/identity-sources/${source!.id}`, values),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["identity-sources"] })
      onOpenChange(false)
      toast.success(t("updateSuccess"))
    },
    onError: (err) => toast.error(err.message),
  })

  const testMutation = useMutation({
    mutationFn: async () => {
      if (!source) throw new Error(t("saveFirstBeforeTest"))
      const res = await api.post<{ success: boolean; message: string }>(
        `/api/v1/identity-sources/${source.id}/test`,
      )
      if (!res.success) throw new Error(res.message || t("testFailed"))
      return res
    },
    onSuccess: (res) => toast.success(res.message || t("testConnectionSuccess")),
    onError: (err) => toast.error(t("testConnectionFailedDetail", { message: err.message })),
  })

  function onSubmit(values: FormValues) {
    if (isEditing) {
      updateMutation.mutate(values)
    } else {
      createMutation.mutate(values)
    }
  }

  const isPending = createMutation.isPending || updateMutation.isPending

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className="sm:max-w-lg overflow-y-auto">
        <SheetHeader>
          <SheetTitle>{isEditing ? t("sheet.editTitle") : t("sheet.createTitle")}</SheetTitle>
          <SheetDescription className="sr-only">
            {isEditing ? t("sheet.editDescription") : t("sheet.createDescription")}
          </SheetDescription>
        </SheetHeader>
        <Form {...form}>
          <form onSubmit={form.handleSubmit(onSubmit)} className="flex flex-1 flex-col gap-4 px-4">
            {/* Basic fields */}
            <FormField
              control={form.control}
              name="name"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t("common:name")}</FormLabel>
                  <FormControl>
                    <Input placeholder={t("form.namePlaceholder")} {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name="type"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t("common:type")}</FormLabel>
                  {isEditing ? (
                    <div>
                      <Input value={field.value.toUpperCase()} disabled />
                      <p className="text-xs text-muted-foreground mt-1">{t("form.typeImmutable")}</p>
                    </div>
                  ) : (
                    <Select value={field.value} onValueChange={field.onChange}>
                      <FormControl>
                        <SelectTrigger>
                          <SelectValue placeholder={t("form.typePlaceholder")} />
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent>
                        <SelectItem value="oidc">OIDC</SelectItem>
                        <SelectItem value="ldap">LDAP</SelectItem>
                      </SelectContent>
                    </Select>
                  )}
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name="domains"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t("domain")}</FormLabel>
                  <FormControl>
                    <Input placeholder={t("form.domainPlaceholder")} {...field} />
                  </FormControl>
                  <FormDescription>{t("form.domainDescription")}</FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name="conflictStrategy"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t("form.conflictStrategy")}</FormLabel>
                  <Select value={field.value} onValueChange={field.onChange}>
                    <FormControl>
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                    </FormControl>
                    <SelectContent>
                      <SelectItem value="fail">{t("form.conflictFail")}</SelectItem>
                      <SelectItem value="link">{t("form.conflictLink")}</SelectItem>
                    </SelectContent>
                  </Select>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name="defaultRoleId"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t("form.defaultRoleId")}</FormLabel>
                  <FormControl>
                    <Input
                      type="number"
                      placeholder={t("form.defaultRoleIdPlaceholder")}
                      {...field}
                      onChange={(e) => field.onChange(Number(e.target.value))}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name="forceSso"
              render={({ field }) => (
                <FormItem className="flex items-center gap-2">
                  <FormControl>
                    <Checkbox
                      checked={field.value}
                      onCheckedChange={field.onChange}
                    />
                  </FormControl>
                  <FormLabel className="!mt-0">{t("form.forceSso")}</FormLabel>
                  <FormMessage />
                </FormItem>
              )}
            />

            {/* Dynamic config section */}
            <div className="space-y-3 rounded-lg border p-4">
              <p className="text-sm font-medium">
                {selectedType === "oidc" ? t("oidc.title") : t("ldap.title")}
              </p>

              {selectedType === "oidc" && <OidcConfigFields form={form} isEditing={isEditing} />}
              {selectedType === "ldap" && <LdapConfigFields form={form} isEditing={isEditing} />}
            </div>

            <SheetFooter>
              {isEditing && (
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  onClick={() => testMutation.mutate()}
                  disabled={testMutation.isPending}
                >
                  {testMutation.isPending ? (
                    <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />
                  ) : (
                    <Plug className="mr-1.5 h-3.5 w-3.5" />
                  )}
                  {t("testConnection")}
                </Button>
              )}
              <Button type="submit" size="sm" disabled={isPending}>
                {isPending ? t("common:saving") : t("common:save")}
              </Button>
            </SheetFooter>
          </form>
        </Form>
      </SheetContent>
    </Sheet>
  )
}

function OidcConfigFields({
  form,
  isEditing,
}: {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  form: any
  isEditing: boolean
}) {
  const { t } = useTranslation("identitySources")

  return (
    <>
      <FormField
        control={form.control}
        name="config.issuerUrl"
        render={({ field }) => (
          <FormItem>
            <FormLabel>{t("oidc.issuerUrl")}</FormLabel>
            <FormControl>
              <Input placeholder="https://accounts.google.com" {...field} value={field.value as string ?? ""} />
            </FormControl>
            <FormMessage />
          </FormItem>
        )}
      />
      <FormField
        control={form.control}
        name="config.clientId"
        render={({ field }) => (
          <FormItem>
            <FormLabel>{t("oidc.clientId")}</FormLabel>
            <FormControl>
              <Input placeholder="your-client-id" {...field} value={field.value as string ?? ""} />
            </FormControl>
            <FormMessage />
          </FormItem>
        )}
      />
      <FormField
        control={form.control}
        name="config.clientSecret"
        render={({ field }) => (
          <FormItem>
            <FormLabel>{t("oidc.clientSecret")}</FormLabel>
            <FormControl>
              <Input
                type="password"
                placeholder={isEditing ? t("oidc.clientSecretPlaceholderEdit") : t("oidc.clientSecretPlaceholderCreate")}
                value={field.value === "\u2022\u2022\u2022\u2022\u2022\u2022" ? "" : (field.value as string ?? "")}
                onChange={field.onChange}
                onBlur={field.onBlur}
                name={field.name}
                ref={field.ref}
              />
            </FormControl>
            <FormMessage />
          </FormItem>
        )}
      />
      <FormField
        control={form.control}
        name="config.callbackUrl"
        render={({ field }) => (
          <FormItem>
            <FormLabel>{t("oidc.callbackUrl")}</FormLabel>
            <FormControl>
              <Input
                readOnly
                className="bg-muted"
                {...field}
                value={field.value as string || `${window.location.origin}/sso/callback`}
              />
            </FormControl>
            <FormDescription>{t("oidc.callbackUrlDescription")}</FormDescription>
            <FormMessage />
          </FormItem>
        )}
      />
      <FormField
        control={form.control}
        name="config.usePkce"
        render={({ field }) => (
          <FormItem className="flex items-center gap-2">
            <FormControl>
              <Checkbox
                checked={field.value as boolean}
                onCheckedChange={field.onChange}
              />
            </FormControl>
            <FormLabel className="!mt-0">{t("oidc.usePkce")}</FormLabel>
          </FormItem>
        )}
      />
      <FormField
        control={form.control}
        name="config.scopes"
        render={({ field }) => (
          <FormItem>
            <FormLabel>{t("oidc.scopes")}</FormLabel>
            <FormControl>
              <Input placeholder="openid,profile,email" {...field} value={field.value as string ?? ""} />
            </FormControl>
            <FormDescription>{t("oidc.scopesDescription")}</FormDescription>
            <FormMessage />
          </FormItem>
        )}
      />
    </>
  )
}

function LdapConfigFields({
  form,
  isEditing,
}: {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  form: any
  isEditing: boolean
}) {
  const { t } = useTranslation("identitySources")

  return (
    <>
      <FormField
        control={form.control}
        name="config.serverUrl"
        render={({ field }) => (
          <FormItem>
            <FormLabel>{t("ldap.serverUrl")}</FormLabel>
            <FormControl>
              <Input placeholder="ldaps://ldap.corp.com:636" {...field} value={field.value as string ?? ""} />
            </FormControl>
            <FormMessage />
          </FormItem>
        )}
      />
      <FormField
        control={form.control}
        name="config.bindDn"
        render={({ field }) => (
          <FormItem>
            <FormLabel>{t("ldap.bindDn")}</FormLabel>
            <FormControl>
              <Input placeholder="cn=admin,dc=example,dc=com" {...field} value={field.value as string ?? ""} />
            </FormControl>
            <FormMessage />
          </FormItem>
        )}
      />
      <FormField
        control={form.control}
        name="config.bindPassword"
        render={({ field }) => (
          <FormItem>
            <FormLabel>{t("ldap.bindPassword")}</FormLabel>
            <FormControl>
              <Input
                type="password"
                placeholder={isEditing ? t("ldap.bindPasswordPlaceholderEdit") : t("ldap.bindPasswordPlaceholderCreate")}
                value={field.value === "\u2022\u2022\u2022\u2022\u2022\u2022" ? "" : (field.value as string ?? "")}
                onChange={field.onChange}
                onBlur={field.onBlur}
                name={field.name}
                ref={field.ref}
              />
            </FormControl>
            <FormMessage />
          </FormItem>
        )}
      />
      <FormField
        control={form.control}
        name="config.searchBase"
        render={({ field }) => (
          <FormItem>
            <FormLabel>{t("ldap.searchBase")}</FormLabel>
            <FormControl>
              <Input placeholder="ou=users,dc=example,dc=com" {...field} value={field.value as string ?? ""} />
            </FormControl>
            <FormMessage />
          </FormItem>
        )}
      />
      <FormField
        control={form.control}
        name="config.userFilter"
        render={({ field }) => (
          <FormItem>
            <FormLabel>{t("ldap.userFilter")}</FormLabel>
            <FormControl>
              <Input placeholder="(uid={{username}})" {...field} value={field.value as string ?? ""} />
            </FormControl>
            <FormMessage />
          </FormItem>
        )}
      />
      <FormField
        control={form.control}
        name="config.useTls"
        render={({ field }) => (
          <FormItem className="flex items-center gap-2">
            <FormControl>
              <Checkbox
                checked={field.value as boolean}
                onCheckedChange={field.onChange}
              />
            </FormControl>
            <FormLabel className="!mt-0">{t("ldap.useTls")}</FormLabel>
          </FormItem>
        )}
      />
      <FormField
        control={form.control}
        name="config.skipVerify"
        render={({ field }) => (
          <FormItem className="flex items-center gap-2">
            <FormControl>
              <Checkbox
                checked={field.value as boolean}
                onCheckedChange={field.onChange}
              />
            </FormControl>
            <FormLabel className="!mt-0">{t("ldap.skipVerify")}</FormLabel>
          </FormItem>
        )}
      />
    </>
  )
}

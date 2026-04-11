import { useEffect } from "react"
import { useTranslation } from "react-i18next"
import { useForm } from "react-hook-form"
import { z } from "zod"
import { zodResolver } from "@hookform/resolvers/zod"
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { toast } from "sonner"
import { api } from "@/lib/api"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Switch } from "@/components/ui/switch"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from "@/components/ui/form"

type FormValues = {
  passwordMinLength: number
  passwordRequireUpper: boolean
  passwordRequireLower: boolean
  passwordRequireNumber: boolean
  passwordRequireSpecial: boolean
  passwordExpiryDays: number
  loginMaxAttempts: number
  loginLockoutMinutes: number
  captchaProvider: "none" | "image"
  maxConcurrentSessions: number
  sessionTimeoutMinutes: number
  requireTwoFactor: boolean
  registrationOpen: boolean
  defaultRoleCode: string
}

type SecuritySettings = FormValues

const defaultValues: FormValues = {
  passwordMinLength: 8,
  passwordRequireUpper: false,
  passwordRequireLower: false,
  passwordRequireNumber: false,
  passwordRequireSpecial: false,
  passwordExpiryDays: 0,
  loginMaxAttempts: 5,
  loginLockoutMinutes: 30,
  captchaProvider: "none",
  maxConcurrentSessions: 5,
  sessionTimeoutMinutes: 10080,
  requireTwoFactor: false,
  registrationOpen: false,
  defaultRoleCode: "",
}

interface RoleOption {
  id: number
  name: string
  code: string
}

function NumberField({
  control,
  name,
  label,
  description,
  min = 0,
}: {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  control: any
  name: keyof FormValues
  label: string
  description?: string
  min?: number
}) {
  return (
    <FormField
      control={control}
      name={name}
      render={({ field }) => (
        <FormItem>
          <FormLabel>{label}</FormLabel>
          <FormControl>
            <Input
              type="number"
              min={min}
              className="max-w-[200px]"
              {...field}
              onChange={(e) => field.onChange(e.target.valueAsNumber)}
            />
          </FormControl>
          {description && <FormDescription>{description}</FormDescription>}
          <FormMessage />
        </FormItem>
      )}
    />
  )
}

function SwitchField({
  control,
  name,
  label,
  description,
}: {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  control: any
  name: keyof FormValues
  label: string
  description?: string
}) {
  return (
    <FormField
      control={control}
      name={name}
      render={({ field }) => (
        <FormItem className="flex items-center justify-between rounded-lg border p-3">
          <div className="space-y-0.5">
            <FormLabel className="text-sm">{label}</FormLabel>
            {description && (
              <FormDescription className="text-xs">{description}</FormDescription>
            )}
          </div>
          <FormControl>
            <Switch checked={field.value as boolean} onCheckedChange={field.onChange} />
          </FormControl>
        </FormItem>
      )}
    />
  )
}

export function SecurityCard() {
  const { t } = useTranslation(["settings", "common"])
  const queryClient = useQueryClient()

  const schema = z.object({
    // Password policy
    passwordMinLength: z.number().int().min(1, t("settings:security.validation.minLengthMin")),
    passwordRequireUpper: z.boolean(),
    passwordRequireLower: z.boolean(),
    passwordRequireNumber: z.boolean(),
    passwordRequireSpecial: z.boolean(),
    passwordExpiryDays: z.number().int().min(0, t("settings:security.validation.numberMin")),

    // Login security
    loginMaxAttempts: z.number().int().min(0, t("settings:security.validation.numberMin")),
    loginLockoutMinutes: z.number().int().min(0, t("settings:security.validation.numberMin")),
    captchaProvider: z.enum(["none", "image"]),

    // Session
    maxConcurrentSessions: z.number().int().min(0, t("settings:security.validation.numberMin")),
    sessionTimeoutMinutes: z.number().int().min(1, t("settings:security.validation.sessionTimeoutMin")),

    // Two-factor
    requireTwoFactor: z.boolean(),

    // Registration
    registrationOpen: z.boolean(),
    defaultRoleCode: z.string(),
  })

  const { data, isLoading } = useQuery({
    queryKey: ["settings", "security"],
    queryFn: () => api.get<SecuritySettings>("/api/v1/settings/security"),
  })

  const { data: roles } = useQuery({
    queryKey: ["roles-options"],
    queryFn: () =>
      api.get<{ items: RoleOption[] }>("/api/v1/roles").then((r) => r.items),
  })

  const form = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues,
  })

  useEffect(() => {
    if (data) {
      form.reset({ ...defaultValues, ...data })
    }
  }, [data, form])

  const mutation = useMutation({
    mutationFn: (values: FormValues) =>
      api.put("/api/v1/settings/security", values),
    onSuccess: (_data, values) => {
      queryClient.invalidateQueries({ queryKey: ["settings", "security"] })
      form.reset(values)
      toast.success(t("settings:security.saveSuccess"))
    },
    onError: () => toast.error(t("settings:security.saveFailed")),
  })

  if (isLoading) {
    return (
      <Card>
        <CardContent className="flex h-32 items-center justify-center text-muted-foreground">
          {t("common:loading")}
        </CardContent>
      </Card>
    )
  }

  return (
    <Form {...form}>
      <form
        onSubmit={form.handleSubmit((v) => mutation.mutate(v))}
        className="space-y-6"
      >
        {/* Password Policy */}
        <Card>
          <CardHeader>
            <CardTitle className="text-base">{t("settings:security.password.title")}</CardTitle>
            <CardDescription>{t("settings:security.password.description")}</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <NumberField
              control={form.control}
              name="passwordMinLength"
              label={t("settings:security.password.minLength")}
              min={1}
            />
            <div className="space-y-2">
              <SwitchField
                control={form.control}
                name="passwordRequireUpper"
                label={t("settings:security.password.requireUpper")}
              />
              <SwitchField
                control={form.control}
                name="passwordRequireLower"
                label={t("settings:security.password.requireLower")}
              />
              <SwitchField
                control={form.control}
                name="passwordRequireNumber"
                label={t("settings:security.password.requireNumber")}
              />
              <SwitchField
                control={form.control}
                name="passwordRequireSpecial"
                label={t("settings:security.password.requireSpecial")}
              />
            </div>
            <NumberField
              control={form.control}
              name="passwordExpiryDays"
              label={t("settings:security.password.expiryDays")}
              description={t("settings:security.password.expiryDaysDescription")}
            />
          </CardContent>
        </Card>

        {/* Login Security */}
        <Card>
          <CardHeader>
            <CardTitle className="text-base">{t("settings:security.login.title")}</CardTitle>
            <CardDescription>{t("settings:security.login.description")}</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="grid gap-4 sm:grid-cols-2">
              <NumberField
                control={form.control}
                name="loginMaxAttempts"
                label={t("settings:security.login.maxAttempts")}
                description={t("settings:security.login.maxAttemptsDescription")}
              />
              <NumberField
                control={form.control}
                name="loginLockoutMinutes"
                label={t("settings:security.login.lockoutMinutes")}
              />
            </div>
            <FormField
              control={form.control}
              name="captchaProvider"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t("settings:security.login.captchaProvider")}</FormLabel>
                  <Select
                    key={field.value ?? defaultValues.captchaProvider}
                    defaultValue={field.value ?? defaultValues.captchaProvider}
                    onValueChange={field.onChange}
                  >
                    <FormControl>
                      <SelectTrigger className="max-w-[200px]">
                        <SelectValue placeholder={t("settings:security.login.captchaPlaceholder")} />
                      </SelectTrigger>
                    </FormControl>
                    <SelectContent
                      position="popper"
                      side="bottom"
                      sideOffset={4}
                      className="bg-background shadow-md"
                    >
                      <SelectItem value="none">{t("settings:security.login.captchaNone")}</SelectItem>
                      <SelectItem value="image">{t("settings:security.login.captchaImage")}</SelectItem>
                    </SelectContent>
                  </Select>
                  <FormMessage />
                </FormItem>
              )}
            />
          </CardContent>
        </Card>

        {/* Session Management */}
        <Card>
          <CardHeader>
            <CardTitle className="text-base">{t("settings:security.session.title")}</CardTitle>
            <CardDescription>{t("settings:security.session.description")}</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="grid gap-4 sm:grid-cols-2">
              <NumberField
                control={form.control}
                name="maxConcurrentSessions"
                label={t("settings:security.session.maxConcurrent")}
                description={t("settings:security.session.maxConcurrentDescription")}
              />
              <NumberField
                control={form.control}
                name="sessionTimeoutMinutes"
                label={t("settings:security.session.timeoutMinutes")}
                min={1}
              />
            </div>
          </CardContent>
        </Card>

        {/* Two-Factor Authentication */}
        <Card>
          <CardHeader>
            <CardTitle className="text-base">{t("settings:security.twoFactor.title")}</CardTitle>
            <CardDescription>{t("settings:security.twoFactor.description")}</CardDescription>
          </CardHeader>
          <CardContent>
            <SwitchField
              control={form.control}
              name="requireTwoFactor"
              label={t("settings:security.twoFactor.requireAll")}
              description={t("settings:security.twoFactor.requireAllDescription")}
            />
          </CardContent>
        </Card>

        {/* Registration */}
        <Card>
          <CardHeader>
            <CardTitle className="text-base">{t("settings:security.registration.title")}</CardTitle>
            <CardDescription>{t("settings:security.registration.description")}</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <SwitchField
              control={form.control}
              name="registrationOpen"
              label={t("settings:security.registration.open")}
              description={t("settings:security.registration.openDescription")}
            />
            <FormField
              control={form.control}
              name="defaultRoleCode"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t("settings:security.registration.defaultRole")}</FormLabel>
                  <Select value={field.value} onValueChange={field.onChange}>
                    <FormControl>
                      <SelectTrigger className="max-w-[200px]">
                        <SelectValue placeholder={t("settings:security.registration.defaultRolePlaceholder")} />
                      </SelectTrigger>
                    </FormControl>
                    <SelectContent>
                      {roles?.map((r) => (
                        <SelectItem key={r.code} value={r.code}>
                          {r.name}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                  <FormDescription>
                    {t("settings:security.registration.defaultRoleDescription")}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          </CardContent>
        </Card>

        <Button
          type="submit"
          disabled={!form.formState.isDirty || mutation.isPending}
        >
          {mutation.isPending ? t("common:saving") : t("common:save")}
        </Button>
      </form>
    </Form>
  )
}

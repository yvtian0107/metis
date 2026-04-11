import { useEffect } from "react"
import { useTranslation } from "react-i18next"
import { useForm } from "react-hook-form"
import { z } from "zod"
import { zodResolver } from "@hookform/resolvers/zod"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { api } from "@/lib/api"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet"
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from "@/components/ui/form"

export interface AuthProvider {
  id: number
  providerKey: string
  displayName: string
  enabled: boolean
  clientId: string
  clientSecret: string
  scopes: string
  callbackUrl: string
  sortOrder: number
  createdAt: string
  updatedAt: string
}

type FormValues = z.infer<ReturnType<typeof createSchema>>

function createSchema(t: (key: string) => string) {
  return z.object({
    clientId: z.string().min(1, t("validation.clientIdRequired")),
    clientSecret: z.string(),
    scopes: z.string(),
    callbackUrl: z.string(),
  })
}

interface Props {
  open: boolean
  onOpenChange: (open: boolean) => void
  provider: AuthProvider | null
}

export function ProviderSheet({ open, onOpenChange, provider }: Props) {
  const { t } = useTranslation(["authProviders", "common"])
  const queryClient = useQueryClient()

  const schema = createSchema(t)

  const form = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: {
      clientId: "",
      clientSecret: "",
      scopes: "",
      callbackUrl: "",
    },
  })

  useEffect(() => {
    if (provider && open) {
      form.reset({
        clientId: provider.clientId,
        clientSecret: "",
        scopes: provider.scopes,
        callbackUrl: provider.callbackUrl,
      })
    }
  }, [provider, open, form])

  const mutation = useMutation({
    mutationFn: (values: FormValues) => {
      const body: Record<string, string> = {
        clientId: values.clientId,
        scopes: values.scopes,
        callbackUrl: values.callbackUrl,
      }
      if (values.clientSecret) {
        body.clientSecret = values.clientSecret
      }
      return api.put(
        `/api/v1/admin/auth-providers/${provider!.providerKey}`,
        body,
      )
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["auth-providers"] })
      onOpenChange(false)
    },
  })

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent>
        <SheetHeader>
          <SheetTitle>{t("sheet.editTitle", { name: provider?.displayName })}</SheetTitle>
          <SheetDescription>
            {t("sheet.editDescription", { name: provider?.displayName })}
          </SheetDescription>
        </SheetHeader>
        <Form {...form}>
          <form
            onSubmit={form.handleSubmit((v) => mutation.mutate(v))}
            className="space-y-4 px-4 pt-4"
          >
            <FormField
              control={form.control}
              name="clientId"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t("clientId")}</FormLabel>
                  <FormControl>
                    <Input placeholder={t("form.clientIdPlaceholder")} {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name="clientSecret"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t("clientSecret")}</FormLabel>
                  <FormControl>
                    <Input
                      type="password"
                      placeholder={
                        provider?.clientSecret
                          ? t("form.clientSecretPlaceholderConfigured")
                          : t("form.clientSecretPlaceholderEmpty")
                      }
                      {...field}
                    />
                  </FormControl>
                  <FormDescription>
                    {provider?.clientSecret
                      ? t("form.clientSecretConfigured")
                      : t("form.clientSecretNotConfigured")}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name="scopes"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>Scopes</FormLabel>
                  <FormControl>
                    <Input placeholder={t("form.scopesPlaceholder")} {...field} />
                  </FormControl>
                  <FormDescription>
                    {t("form.scopesDescription")}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name="callbackUrl"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t("callbackUrl")}</FormLabel>
                  <FormControl>
                    <Input
                      placeholder={t("form.callbackUrlPlaceholder")}
                      {...field}
                    />
                  </FormControl>
                  <FormDescription>
                    {t("form.callbackUrlDescription")}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
            <Button type="submit" disabled={mutation.isPending}>
              {mutation.isPending ? t("common:saving") : t("common:save")}
            </Button>
          </form>
        </Form>
      </SheetContent>
    </Sheet>
  )
}

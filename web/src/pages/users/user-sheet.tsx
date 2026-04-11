import { useEffect } from "react"
import { useTranslation } from "react-i18next"
import { useForm } from "react-hook-form"
import { z } from "zod"
import { zodResolver } from "@hookform/resolvers/zod"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { api, type PaginatedResponse } from "@/lib/api"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
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
} from "@/components/ui/form"
import type { User } from "@/stores/auth"

interface RoleOption {
  id: number
  name: string
  code: string
}

function createCreateSchema(t: (key: string) => string) {
  return z.object({
    username: z.string().min(1, t("users:validation.usernameRequired")).max(64),
    password: z.string().min(1, t("users:validation.passwordRequired")),
    email: z.string().email(t("users:validation.emailInvalid")).or(z.literal("")).optional(),
    phone: z.string().max(32).optional(),
    roleId: z.coerce.number().min(1, t("users:validation.roleRequired")),
  })
}

function createEditSchema(t: (key: string) => string) {
  return z.object({
    email: z.string().email(t("users:validation.emailInvalid")).or(z.literal("")).optional(),
    phone: z.string().max(32).optional(),
    roleId: z.coerce.number().min(1, t("users:validation.roleRequired")),
  })
}

type CreateValues = z.infer<ReturnType<typeof createCreateSchema>>
type EditValues = z.infer<ReturnType<typeof createEditSchema>>

interface UserSheetProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  user: User | null
}

export function UserSheet({ open, onOpenChange, user }: UserSheetProps) {
  const { t } = useTranslation(["users", "common"])
  const queryClient = useQueryClient()
  const isEditing = user !== null

  const { data: rolesData } = useQuery({
    queryKey: ["roles", "all"],
    queryFn: () =>
      api.get<PaginatedResponse<RoleOption>>("/api/v1/roles?page=1&pageSize=100"),
    enabled: open,
  })

  const roles = rolesData?.items ?? []

  const form = useForm<CreateValues>({
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    resolver: zodResolver((isEditing ? createEditSchema(t) : createCreateSchema(t)) as any),
    defaultValues: {
      username: "",
      password: "",
      email: "",
      phone: "",
      roleId: 0,
    },
  })

  useEffect(() => {
    if (open) {
      if (user) {
        form.reset({
          username: user.username,
          password: "",
          email: user.email || "",
          phone: user.phone || "",
          roleId: user.role?.id || 0,
        })
      } else {
        form.reset({
          username: "",
          password: "",
          email: "",
          phone: "",
          roleId: 0,
        })
      }
    }
  }, [open, user, form])

  const createMutation = useMutation({
    mutationFn: (values: CreateValues) =>
      api.post("/api/v1/users", values),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["users"] })
      onOpenChange(false)
    },
  })

  const updateMutation = useMutation({
    mutationFn: (values: EditValues) =>
      api.put(`/api/v1/users/${user!.id}`, values),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["users"] })
      onOpenChange(false)
    },
  })

  function onSubmit(values: CreateValues) {
    if (isEditing) {
      updateMutation.mutate({
        email: values.email,
        phone: values.phone,
        roleId: values.roleId,
      })
    } else {
      createMutation.mutate(values)
    }
  }

  const isPending = createMutation.isPending || updateMutation.isPending
  const error = createMutation.error || updateMutation.error

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className="sm:max-w-md">
        <SheetHeader>
          <SheetTitle>{isEditing ? t("users:editUser") : t("users:createUser")}</SheetTitle>
          <SheetDescription className="sr-only">
            {isEditing ? t("users:editDescription") : t("users:createDescription")}
          </SheetDescription>
        </SheetHeader>
        <Form {...form}>
          <form onSubmit={form.handleSubmit(onSubmit)} className="flex flex-1 flex-col gap-4 px-4">
            <FormField
              control={form.control}
              name="username"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t("users:username")}</FormLabel>
                  <FormControl>
                    <Input placeholder={t("users:usernamePlaceholder")} disabled={isEditing} {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            {!isEditing && (
              <FormField
                control={form.control}
                name="password"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t("users:password")}</FormLabel>
                    <FormControl>
                      <Input type="password" placeholder={t("users:passwordPlaceholder")} {...field} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            )}
            <FormField
              control={form.control}
              name="email"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t("users:email")}</FormLabel>
                  <FormControl>
                    <Input placeholder={t("users:emailPlaceholder")} {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name="phone"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t("users:phone")}</FormLabel>
                  <FormControl>
                    <Input placeholder={t("users:phonePlaceholder")} {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name="roleId"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t("users:role")}</FormLabel>
                  <Select
                    value={field.value ? String(field.value) : ""}
                    onValueChange={(v) => field.onChange(Number(v))}
                  >
                    <FormControl>
                      <SelectTrigger>
                        <SelectValue placeholder={t("users:selectRole")} />
                      </SelectTrigger>
                    </FormControl>
                    <SelectContent>
                      {roles.map((role) => (
                        <SelectItem key={role.id} value={String(role.id)}>
                          {role.name}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                  <FormMessage />
                </FormItem>
              )}
            />

            {error && (
              <p className="text-sm text-destructive">{error.message}</p>
            )}

            <SheetFooter>
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

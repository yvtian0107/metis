import { useEffect } from "react"
import { useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { z } from "zod"
import { zodResolver } from "@hookform/resolvers/zod"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { api } from "@/lib/api"
import { toast } from "sonner"
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

const ROOT_VALUE = "__root__"
const NONE_VALUE = "__none__"

export interface DepartmentItem {
  id: number
  name: string
  code: string
  parentId: number | null
  managerId: number | null
  managerName: string
  sort: number
  description: string
  isActive: boolean
  createdAt: string
  updatedAt: string
}

interface TreeNode extends DepartmentItem {
  children?: TreeNode[]
}

interface UserOption {
  id: number
  username: string
}

function useDepartmentSchema() {
  const { t } = useTranslation("org")
  return z.object({
    name: z.string().min(1, t("validation.nameRequired")),
    code: z.string().min(1, t("validation.codeRequired")),
    parentId: z.string().optional(),
    managerId: z.string().optional(),
    description: z.string().optional(),
  })
}

type FormValues = z.infer<ReturnType<typeof useDepartmentSchema>>

interface DepartmentSheetProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  department: DepartmentItem | null
}

export function DepartmentSheet({ open, onOpenChange, department }: DepartmentSheetProps) {
  const { t } = useTranslation(["org", "common"])
  const queryClient = useQueryClient()
  const isEditing = department !== null
  const schema = useDepartmentSchema()

  const form = useForm<FormValues>({
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    resolver: zodResolver(schema as any),
    defaultValues: { name: "", code: "", parentId: ROOT_VALUE, managerId: NONE_VALUE, description: "" },
  })

  const { data: treeData = [] } = useQuery({
    queryKey: ["departments", "tree"],
    queryFn: async () => {
      const res = await api.get<{ items: TreeNode[] }>("/api/v1/org/departments/tree")
      return res.items ?? []
    },
    enabled: open,
  })

  const { data: users = [] } = useQuery({
    queryKey: ["users", "all"],
    queryFn: async () => {
      const res = await api.get<{ items: UserOption[] }>("/api/v1/users", { page: 1, pageSize: 1000 })
      return res.items ?? []
    },
    enabled: open,
  })

  useEffect(() => {
    if (open) {
      if (department) {
        form.reset({
          name: department.name,
          code: department.code,
          parentId: department.parentId ? String(department.parentId) : ROOT_VALUE,
          managerId: department.managerId ? String(department.managerId) : NONE_VALUE,
          description: department.description,
        })
      } else {
        form.reset({ name: "", code: "", parentId: ROOT_VALUE, managerId: NONE_VALUE, description: "" })
      }
    }
  }, [open, department, form])

  const createMutation = useMutation({
    mutationFn: async (values: FormValues) => {
      await api.post("/api/v1/org/departments", {
        name: values.name,
        code: values.code,
        parentId: values.parentId && values.parentId !== ROOT_VALUE ? Number(values.parentId) : null,
        managerId: values.managerId && values.managerId !== NONE_VALUE ? Number(values.managerId) : null,
        description: values.description,
      })
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["departments"] })
      onOpenChange(false)
      toast.success(t("org:departments.createSuccess"))
    },
    onError: (err) => toast.error(err.message),
  })

  const updateMutation = useMutation({
    mutationFn: async (values: FormValues) => {
      await api.put(`/api/v1/org/departments/${department!.id}`, {
        name: values.name,
        code: values.code,
        parentId: values.parentId && values.parentId !== ROOT_VALUE ? Number(values.parentId) : null,
        managerId: values.managerId && values.managerId !== NONE_VALUE ? Number(values.managerId) : null,
        description: values.description,
      })
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["departments"] })
      onOpenChange(false)
      toast.success(t("org:departments.updateSuccess"))
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

  // Flatten tree options for Select, excluding current node and its descendants when editing
  const flatOptions: { value: string; label: string; depth: number }[] = []
  function flatten(nodes: TreeNode[] | undefined, depth: number) {
    if (!nodes) return
    for (const node of nodes) {
      if (isEditing && node.id === department!.id) continue
      flatOptions.push({ value: String(node.id), label: node.name, depth })
      flatten(node.children, depth + 1)
    }
  }
  flatten(treeData, 0)

  const isPending = createMutation.isPending || updateMutation.isPending

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className="sm:max-w-lg">
        <SheetHeader>
          <SheetTitle>
            {isEditing ? t("org:departments.edit") : t("org:departments.create")}
          </SheetTitle>
          <SheetDescription className="sr-only">
            {isEditing ? t("org:departments.edit") : t("org:departments.create")}
          </SheetDescription>
        </SheetHeader>
        <Form {...form}>
          <form onSubmit={form.handleSubmit(onSubmit)} className="flex flex-1 flex-col gap-5 px-4">
            <FormField
              control={form.control}
              name="name"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t("org:departments.name")}</FormLabel>
                  <FormControl>
                    <Input placeholder={t("org:departments.namePlaceholder")} {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name="code"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t("org:departments.code")}</FormLabel>
                  <FormControl>
                    <Input placeholder={t("org:departments.codePlaceholder")} {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name="parentId"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t("org:departments.parent")}</FormLabel>
                  <Select value={field.value} onValueChange={field.onChange}>
                    <FormControl>
                      <SelectTrigger>
                        <SelectValue placeholder={t("org:departments.topDepartment")} />
                      </SelectTrigger>
                    </FormControl>
                    <SelectContent>
                      <SelectItem value={ROOT_VALUE}>{t("org:departments.topDepartment")}</SelectItem>
                      {flatOptions.map((opt) => (
                        <SelectItem key={opt.value} value={opt.value}>
                          <span style={{ paddingLeft: opt.depth * 12 }}>{opt.label}</span>
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name="managerId"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t("org:departments.manager")}</FormLabel>
                  <Select value={field.value} onValueChange={field.onChange}>
                    <FormControl>
                      <SelectTrigger>
                        <SelectValue placeholder={t("org:departments.selectManager")} />
                      </SelectTrigger>
                    </FormControl>
                    <SelectContent>
                      <SelectItem value={NONE_VALUE}>{t("org:departments.noManager")}</SelectItem>
                      {users.map((user) => (
                        <SelectItem key={user.id} value={String(user.id)}>
                          {user.username}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name="description"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t("org:departments.description")}</FormLabel>
                  <FormControl>
                    <Textarea rows={3} {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
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

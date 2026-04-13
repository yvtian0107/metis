import { useState } from "react"
import { useTranslation } from "react-i18next"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { useForm } from "react-hook-form"
import { z } from "zod"
import { zodResolver } from "@hookform/resolvers/zod"
import { toast } from "sonner"
import { Plus, KeyRound, Trash2, Copy, Check, AlertTriangle } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Badge } from "@/components/ui/badge"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import {
  DataTableActions,
  DataTableActionsCell,
  DataTableActionsHead,
  DataTableCard,
  DataTableEmptyRow,
  DataTableLoadingRow,
} from "@/components/ui/data-table"
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
import { usePermission } from "@/hooks/use-permission"
import { formatDateTime } from "@/lib/utils"
import { observeApi, type TokenResponse } from "../../api"

const MAX_TOKENS = 10

// ── Create Token Sheet ────────────────────────────────────────────────────────

const createTokenSchema = z.object({
  name: z.string().min(1).max(100),
})

type CreateTokenForm = z.infer<typeof createTokenSchema>

type SheetPhase = "form" | "reveal"

function CreateTokenSheet({
  open,
  onOpenChange,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  const { t } = useTranslation(["observe", "common"])
  const queryClient = useQueryClient()
  const [phase, setPhase] = useState<SheetPhase>("form")
  const [rawToken, setRawToken] = useState("")
  const [copied, setCopied] = useState(false)
  const [closeConfirm, setCloseConfirm] = useState(false)

  const form = useForm<CreateTokenForm>({
    resolver: zodResolver(createTokenSchema),
    defaultValues: { name: "" },
  })

  function resetSheet() {
    form.reset({ name: "" })
    setPhase("form")
    setRawToken("")
    setCopied(false)
    setCloseConfirm(false)
  }

  const { mutate: createToken, isPending } = useMutation({
    mutationFn: (data: CreateTokenForm) => observeApi.createToken(data.name),
    onSuccess: (data) => {
      setRawToken(data.token)
      setPhase("reveal")
      queryClient.invalidateQueries({ queryKey: ["observe-tokens"] })
    },
    onError: (err: Error) => toast.error(err.message),
  })

  const handleCopy = () => {
    navigator.clipboard.writeText(rawToken)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
    toast.success(t("observe:tokens.secretCopied"))
  }

  const handleClose = () => {
    if (phase === "reveal" && !copied) {
      setCloseConfirm(true)
      return
    }
    doClose()
  }

  const doClose = () => {
    resetSheet()
    onOpenChange(false)
  }

  function handleOpenChange(nextOpen: boolean) {
    if (nextOpen) {
      resetSheet()
    } else {
      handleClose()
    }
  }

  return (
    <>
      <Sheet open={open} onOpenChange={handleOpenChange}>
        <SheetContent className="gap-0 p-0 sm:max-w-md">
          <SheetHeader className="border-b px-6 py-5">
            <SheetTitle>{t("observe:tokens.createTitle")}</SheetTitle>
            <SheetDescription className="sr-only">
              {t("observe:tokens.createTitle")}
            </SheetDescription>
          </SheetHeader>

          {phase === "form" ? (
            <Form {...form}>
              <form onSubmit={form.handleSubmit((data) => createToken(data))} className="flex min-h-0 flex-1 flex-col overflow-hidden">
                <div className="flex-1 space-y-5 overflow-auto px-6 py-6">
                  <FormField
                    control={form.control}
                    name="name"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t("observe:tokens.name")}</FormLabel>
                        <FormControl>
                          <Input
                            placeholder={t("observe:tokens.namePlaceholder")}
                            {...field}
                          />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                </div>
                <SheetFooter className="px-6 py-4">
                  <Button variant="outline" type="button" onClick={() => onOpenChange(false)}>
                    {t("common:cancel")}
                  </Button>
                  <Button type="submit" disabled={isPending}>
                    {isPending ? t("observe:tokens.generating") : t("observe:tokens.generate")}
                  </Button>
                </SheetFooter>
              </form>
            </Form>
          ) : (
            <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
              <div className="flex-1 space-y-5 overflow-auto px-6 py-6">
                <div className="flex items-start gap-2 rounded-lg border border-amber-200 bg-amber-50 dark:border-amber-800 dark:bg-amber-950/40 p-3 text-sm text-amber-800 dark:text-amber-300">
                  <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" />
                  <p>{t("observe:tokens.secretDesc")}</p>
                </div>

                <div className="space-y-1.5">
                  <label className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
                    {t("observe:tokens.secretTitle")}
                  </label>
                  <div className="flex items-center gap-2 rounded-lg border border-border bg-muted/40 px-3 py-2">
                    <code className="flex-1 break-all font-mono text-xs text-foreground">
                      {rawToken}
                    </code>
                    <button
                      onClick={handleCopy}
                      className="shrink-0 rounded p-1 text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
                    >
                      {copied ? <Check className="h-4 w-4" /> : <Copy className="h-4 w-4" />}
                    </button>
                  </div>
                </div>
              </div>
              <SheetFooter className="px-6 py-4">
                <Button onClick={doClose}>
                  {t("observe:tokens.closeAnyway")}
                </Button>
              </SheetFooter>
            </div>
          )}
        </SheetContent>
      </Sheet>

      <AlertDialog open={closeConfirm} onOpenChange={setCloseConfirm}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t("observe:tokens.secretTitle")}</AlertDialogTitle>
            <AlertDialogDescription>{t("observe:tokens.closeConfirm")}</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel onClick={() => setCloseConfirm(false)}>
              {t("observe:tokens.cancel")}
            </AlertDialogCancel>
            <AlertDialogAction onClick={doClose}>{t("observe:tokens.closeAnyway")}</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}

// ── Main Page ────────────────────────────────────────────────────────────────

export function Component() {
  const { t } = useTranslation(["observe", "common"])
  const queryClient = useQueryClient()
  const [createOpen, setCreateOpen] = useState(false)

  const canCreate = usePermission("observe:token:create")
  const canRevoke = usePermission("observe:token:revoke")

  const { data: tokens = [], isLoading } = useQuery({
    queryKey: ["observe-tokens"],
    queryFn: observeApi.listTokens,
  })

  const revokeMutation = useMutation({
    mutationFn: (id: number) => observeApi.revokeToken(id),
    onSuccess: () => {
      toast.success(t("observe:tokens.revokeSuccess"))
      queryClient.invalidateQueries({ queryKey: ["observe-tokens"] })
    },
    onError: (err: Error) => toast.error(err.message),
  })

  const atLimit = tokens.length >= MAX_TOKENS

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h2 className="text-lg font-semibold">{t("observe:tokens.title")}</h2>
        {canCreate && (
          <Button
            size="sm"
            disabled={atLimit}
            onClick={() => setCreateOpen(true)}
            title={atLimit ? t("observe:tokens.limitReached") : undefined}
          >
            <Plus className="mr-1.5 h-4 w-4" />
            {atLimit ? t("observe:tokens.limitReached") : t("observe:tokens.create")}
          </Button>
        )}
      </div>

      <DataTableCard>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="min-w-[160px]">{t("observe:tokens.name")}</TableHead>
              <TableHead>{t("observe:tokens.token")}</TableHead>
              <TableHead className="w-[100px]">{t("observe:tokens.scope")}</TableHead>
              <TableHead className="w-[160px]">{t("observe:tokens.lastUsed")}</TableHead>
              <TableHead className="w-[160px]">{t("observe:tokens.createdAt")}</TableHead>
              <DataTableActionsHead />
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading ? (
              <DataTableLoadingRow colSpan={6} />
            ) : tokens.length === 0 ? (
              <DataTableEmptyRow
                colSpan={6}
                icon={KeyRound}
                title={t("observe:tokens.empty")}
                description={t("observe:tokens.emptyHint")}
              />
            ) : (
              tokens.map((token) => (
                <TokenRow
                  key={token.id}
                  token={token}
                  canRevoke={canRevoke}
                  onRevoke={(id) => revokeMutation.mutate(id)}
                />
              ))
            )}
          </TableBody>
        </Table>
      </DataTableCard>

      <CreateTokenSheet open={createOpen} onOpenChange={setCreateOpen} />
    </div>
  )
}

// ── Token Row ────────────────────────────────────────────────────────────────

function TokenRow({
  token,
  canRevoke,
  onRevoke,
}: {
  token: TokenResponse
  canRevoke: boolean
  onRevoke: (id: number) => void
}) {
  const { t } = useTranslation(["observe", "common"])

  return (
    <TableRow>
      <TableCell className="font-medium">{token.name}</TableCell>
      <TableCell>
        <code className="font-mono text-xs text-muted-foreground break-all">{token.token}</code>
      </TableCell>
      <TableCell>
        <Badge variant="outline" className="text-xs">
          {t("observe:tokens.personal")}
        </Badge>
      </TableCell>
      <TableCell className="text-sm text-muted-foreground whitespace-nowrap">
        {token.lastUsedAt ? formatDateTime(token.lastUsedAt) : t("observe:tokens.neverUsed")}
      </TableCell>
      <TableCell className="text-sm text-muted-foreground whitespace-nowrap">
        {formatDateTime(token.createdAt)}
      </TableCell>
      <DataTableActionsCell>
        <DataTableActions>
          {canRevoke && (
            <AlertDialog>
              <AlertDialogTrigger asChild>
                <Button
                  variant="ghost"
                  size="sm"
                  className="px-2.5 text-destructive hover:text-destructive"
                >
                  <Trash2 className="mr-1 h-3.5 w-3.5" />
                  {t("observe:tokens.revoke")}
                </Button>
              </AlertDialogTrigger>
              <AlertDialogContent>
                <AlertDialogHeader>
                  <AlertDialogTitle>{t("observe:tokens.revokeTitle")}</AlertDialogTitle>
                  <AlertDialogDescription>
                    {t("observe:tokens.revokeDesc", { name: token.name })}
                  </AlertDialogDescription>
                </AlertDialogHeader>
                <AlertDialogFooter>
                  <AlertDialogCancel>{t("observe:tokens.cancel")}</AlertDialogCancel>
                  <AlertDialogAction
                    className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
                    onClick={() => onRevoke(token.id)}
                  >
                    {t("observe:tokens.revokeConfirm")}
                  </AlertDialogAction>
                </AlertDialogFooter>
              </AlertDialogContent>
            </AlertDialog>
          )}
        </DataTableActions>
      </DataTableActionsCell>
    </TableRow>
  )
}

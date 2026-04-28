import { useState } from "react"
import { useQuery, useQueryClient } from "@tanstack/react-query"
import { useTranslation } from "react-i18next"
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from "@/components/ui/table"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Textarea } from "@/components/ui/textarea"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { ChevronDown, ChevronRight, Pencil, Check, X } from "lucide-react"
import { toast } from "sonner"
import { usePermission } from "@/hooks/use-permission"
import { fetchTicketVariables, updateTicketVariable, type ProcessVariableItem } from "../api"

const TYPE_VARIANT: Record<string, "default" | "secondary" | "outline"> = {
  string: "secondary",
  number: "outline",
  boolean: "outline",
  json: "default",
  date: "secondary",
}

function ValueDisplay({ item }: { item: ProcessVariableItem }) {
  const [expanded, setExpanded] = useState(false)

  if (item.value === null || item.value === undefined) {
    return <span className="text-muted-foreground">—</span>
  }

  if (item.valueType === "boolean") {
    return (
      <Badge variant={item.value === true ? "default" : "secondary"} className="text-xs">
        {String(item.value)}
      </Badge>
    )
  }

  if (item.valueType === "json") {
    const formatted = JSON.stringify(item.value, null, 2)
    const isLong = formatted.length > 80
    return (
      <div>
        <pre className="whitespace-pre-wrap break-all rounded bg-muted/50 px-2 py-1 text-xs font-mono max-w-[300px]">
          {expanded || !isLong ? formatted : `${formatted.slice(0, 80)}…`}
        </pre>
        {isLong && (
          <button className="text-xs text-primary underline mt-0.5" onClick={() => setExpanded(!expanded)}>
            {expanded ? "collapse" : "expand"}
          </button>
        )}
      </div>
    )
  }

  return <span className="text-sm">{String(item.value)}</span>
}

function EditableRow({
  item,
  ticketId,
  onDone,
}: {
  item: ProcessVariableItem
  ticketId: number
  onDone: () => void
}) {
  const { t } = useTranslation("itsm")
  const [value, setValue] = useState(() => {
    if (item.valueType === "json") return JSON.stringify(item.value, null, 2)
    return String(item.value ?? "")
  })
  const [error, setError] = useState("")
  const [saving, setSaving] = useState(false)

  async function handleSave() {
    setError("")
    let parsed: unknown = value

    if (item.valueType === "json") {
      try {
        parsed = JSON.parse(value)
      } catch {
        setError(t("variables.invalidJson"))
        return
      }
    } else if (item.valueType === "number") {
      const n = Number(value)
      if (Number.isNaN(n)) {
        setError(t("variables.invalidNumber"))
        return
      }
      parsed = n
    } else if (item.valueType === "boolean") {
      parsed = value === "true"
    }

    setSaving(true)
    try {
      await updateTicketVariable(ticketId, item.key, { value: parsed })
      toast.success(t("variables.updated"))
      onDone()
    } catch {
      toast.error(t("variables.updateFailed"))
    } finally {
      setSaving(false)
    }
  }

  const isJson = item.valueType === "json"

  return (
    <div className="flex items-start gap-1">
      {isJson ? (
        <Textarea
          value={value}
          onChange={(e) => setValue(e.target.value)}
          className="font-mono text-xs min-h-[60px]"
        />
      ) : (
        <Input
          value={value}
          onChange={(e) => setValue(e.target.value)}
          className="h-7 text-sm"
        />
      )}
      <Button variant="ghost" size="icon" className="h-7 w-7 shrink-0" onClick={handleSave} disabled={saving}>
        <Check className="h-3.5 w-3.5" />
      </Button>
      <Button variant="ghost" size="icon" className="h-7 w-7 shrink-0" onClick={onDone}>
        <X className="h-3.5 w-3.5" />
      </Button>
      {error && <span className="text-xs text-destructive">{error}</span>}
    </div>
  )
}

export function VariablesPanel({
  ticketId,
  variant = "card",
}: {
  ticketId: number
  variant?: "card" | "flat"
}) {
  const { t } = useTranslation("itsm")
  const queryClient = useQueryClient()
  const canEdit = usePermission("itsm:variables:edit")
  const [editingKey, setEditingKey] = useState<string | null>(null)
  const [collapsedScopes, setCollapsedScopes] = useState<Set<string>>(new Set())

  const { data: variables = [] } = useQuery({
    queryKey: ["itsm-ticket-variables", ticketId],
    queryFn: () => fetchTicketVariables(ticketId),
    enabled: ticketId > 0,
  })

  // Group by scopeId
  const grouped = new Map<string, ProcessVariableItem[]>()
  for (const v of variables) {
    const list = grouped.get(v.scopeId) ?? []
    list.push(v)
    grouped.set(v.scopeId, list)
  }
  const scopes = Array.from(grouped.keys()).sort((a, b) => (a === "root" ? -1 : b === "root" ? 1 : a.localeCompare(b)))
  const hasMultipleScopes = scopes.length > 1

  function toggleScope(scope: string) {
    setCollapsedScopes((prev) => {
      const next = new Set(prev)
      if (next.has(scope)) next.delete(scope)
      else next.add(scope)
      return next
    })
  }

  function handleEditDone() {
    setEditingKey(null)
    queryClient.invalidateQueries({ queryKey: ["itsm-ticket-variables", ticketId] })
  }

  function renderTable(vars: ProcessVariableItem[]) {
    return (
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{t("variables.key")}</TableHead>
            <TableHead>{t("variables.value")}</TableHead>
            <TableHead>{t("variables.type")}</TableHead>
            <TableHead>{t("variables.source")}</TableHead>
            <TableHead>{t("variables.updatedAt")}</TableHead>
            {canEdit && <TableHead className="w-10" />}
          </TableRow>
        </TableHeader>
        <TableBody>
          {vars.map((v) => (
            <TableRow key={v.id}>
              <TableCell className="font-mono text-sm">{v.key}</TableCell>
              <TableCell className="max-w-[300px]">
                {editingKey === v.key ? (
                  <EditableRow item={v} ticketId={ticketId} onDone={handleEditDone} />
                ) : (
                  <ValueDisplay item={v} />
                )}
              </TableCell>
              <TableCell>
                <Badge variant={TYPE_VARIANT[v.valueType] ?? "secondary"}>
                  {v.valueType}
                </Badge>
              </TableCell>
              <TableCell className="text-xs text-muted-foreground">{v.source}</TableCell>
              <TableCell className="text-xs text-muted-foreground">
                {new Date(v.updatedAt).toLocaleString()}
              </TableCell>
              {canEdit && (
                <TableCell>
                  {editingKey !== v.key && (
                    <Button variant="ghost" size="icon" className="h-6 w-6" onClick={() => setEditingKey(v.key)}>
                      <Pencil className="h-3 w-3" />
                    </Button>
                  )}
                </TableCell>
              )}
            </TableRow>
          ))}
        </TableBody>
      </Table>
    )
  }

  const content = variables.length === 0 ? (
    <p className="text-sm text-muted-foreground">{t("variables.empty")}</p>
  ) : hasMultipleScopes ? (
    <div className="space-y-3">
      {scopes.map((scope) => {
        const vars = grouped.get(scope) ?? []
        const isCollapsed = collapsedScopes.has(scope)
        const isRoot = scope === "root"
        return (
          <div key={scope}>
            {!isRoot && (
              <button
                className="mb-1 flex items-center gap-1 text-sm font-medium text-muted-foreground"
                onClick={() => toggleScope(scope)}
              >
                {isCollapsed ? <ChevronRight className="h-3.5 w-3.5" /> : <ChevronDown className="h-3.5 w-3.5" />}
                {t("variables.scope")}: {scope}
                <Badge variant="outline" className="ml-1 text-xs">{vars.length}</Badge>
              </button>
            )}
            {(isRoot || !isCollapsed) && renderTable(vars)}
          </div>
        )
      })}
    </div>
  ) : (
    renderTable(variables)
  )

  if (variant === "flat") {
    return (
      <div className="space-y-3">
        <h4 className="text-sm font-semibold">{t("variables.title")}</h4>
        {content}
      </div>
    )
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">{t("variables.title")}</CardTitle>
      </CardHeader>
      <CardContent>{content}</CardContent>
    </Card>
  )
}

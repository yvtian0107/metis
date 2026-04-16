import { useState } from "react"
import { useTranslation } from "react-i18next"
import { Copy, Check, Search, Code, List } from "lucide-react"
import { Badge } from "@/components/ui/badge"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { Input } from "@/components/ui/input"
import { Button } from "@/components/ui/button"
import type { Span } from "../api"

interface SpanDetailPanelProps {
  span: Span | null
}

function CopyButton({ value }: { value: string }) {
  const [copied, setCopied] = useState(false)
  const handleCopy = () => {
    navigator.clipboard.writeText(value)
    setCopied(true)
    setTimeout(() => setCopied(false), 1500)
  }
  return (
    <button type="button" onClick={handleCopy} className="ml-1 inline-flex items-center text-muted-foreground hover:text-foreground">
      {copied ? <Check className="h-3 w-3 text-emerald-500" /> : <Copy className="h-3 w-3" />}
    </button>
  )
}

function AttributeTable({ data, search }: { data: Record<string, string> | null | undefined; search: string }) {
  const entries = Object.entries(data ?? {})
  const filtered = search
    ? entries.filter(([k, v]) => k.toLowerCase().includes(search.toLowerCase()) || v.toLowerCase().includes(search.toLowerCase()))
    : entries

  if (entries.length === 0) {
    return <p className="text-sm text-muted-foreground">No data</p>
  }

  return (
    <div className="space-y-1">
      {filtered.map(([key, value]) => (
        <div key={key} className="flex items-start gap-2 text-sm py-0.5">
          <span className="shrink-0 font-mono text-xs text-muted-foreground">{key}</span>
          <span className="break-all font-mono text-xs flex-1">{value}</span>
          <CopyButton value={value} />
        </div>
      ))}
      {filtered.length === 0 && search && (
        <p className="text-xs text-muted-foreground">No matching attributes</p>
      )}
    </div>
  )
}

function JsonTreeView({ data }: { data: Record<string, string> | null | undefined }) {
  const entries = data ?? {}

  // Lazy load react-json-view-lite
  const [JsonView, setJsonView] = useState<React.ComponentType<{ data: unknown; shouldExpandNode?: () => boolean }> | null>(null)

  if (Object.keys(entries).length === 0) {
    return <p className="text-sm text-muted-foreground">No data</p>
  }

  if (!JsonView) {
    import("react-json-view-lite").then((mod) => {
      setJsonView(() => mod.JsonView as React.ComponentType<{ data: unknown; shouldExpandNode?: () => boolean }>)
    })
    return <p className="text-xs text-muted-foreground">Loading...</p>
  }

  return (
    <div className="text-xs [&_.json-view-lite]:!bg-transparent">
      <JsonView data={entries} shouldExpandNode={() => true} />
    </div>
  )
}

export function SpanDetailPanel({ span }: SpanDetailPanelProps) {
  const { t } = useTranslation("apm")
  const [attrSearch, setAttrSearch] = useState("")
  const [viewMode, setViewMode] = useState<"table" | "json">("table")

  if (!span) {
    return (
      <div className="flex h-full items-center justify-center text-sm text-muted-foreground">
        {t("detail.selectSpan", "Select a span to view details")}
      </div>
    )
  }

  const durationMs = span.duration / 1e6
  const isError = span.statusCode === "STATUS_CODE_ERROR"

  return (
    <div className="h-full overflow-y-auto p-4 space-y-4">
      {/* Summary */}
      <div className="space-y-2 rounded-lg border p-3 text-sm">
        <div className="flex justify-between">
          <span className="text-muted-foreground">{t("detail.service")}</span>
          <span className="font-medium">{span.serviceName}</span>
        </div>
        <div className="flex justify-between">
          <span className="text-muted-foreground">{t("detail.operation")}</span>
          <span className="font-mono text-xs">{span.spanName}</span>
        </div>
        <div className="flex justify-between">
          <span className="text-muted-foreground">{t("detail.duration")}</span>
          <span className="font-mono text-xs">{durationMs.toFixed(2)} ms</span>
        </div>
        <div className="flex justify-between">
          <span className="text-muted-foreground">{t("detail.status")}</span>
          <Badge variant={isError ? "destructive" : "secondary"} className="text-xs">
            {span.statusCode.replace("STATUS_CODE_", "")}
          </Badge>
        </div>
        {span.statusMessage && (
          <div className="flex justify-between">
            <span className="text-muted-foreground">Message</span>
            <span className="text-xs text-destructive">{span.statusMessage}</span>
          </div>
        )}
        <div className="flex justify-between items-center">
          <span className="text-muted-foreground">{t("detail.spanId")}</span>
          <span className="font-mono text-xs flex items-center">
            {span.spanId}
            <CopyButton value={span.spanId} />
          </span>
        </div>
        {span.parentSpanId && (
          <div className="flex justify-between">
            <span className="text-muted-foreground">{t("detail.parentSpanId")}</span>
            <span className="font-mono text-xs">{span.parentSpanId}</span>
          </div>
        )}
      </div>

      {/* Tabs */}
      <Tabs defaultValue="attributes">
        <TabsList className="w-full">
          <TabsTrigger value="attributes" className="flex-1">{t("detail.attributes")}</TabsTrigger>
          <TabsTrigger value="resource" className="flex-1">{t("detail.resource")}</TabsTrigger>
          <TabsTrigger value="events" className="flex-1">
            {t("detail.events")} {span.events?.length ? `(${span.events.length})` : ""}
          </TabsTrigger>
        </TabsList>
        <TabsContent value="attributes" className="mt-3 space-y-2">
          <div className="flex items-center gap-2">
            <div className="relative flex-1">
              <Search className="absolute left-2 top-1/2 -translate-y-1/2 h-3 w-3 text-muted-foreground" />
              <Input
                placeholder="Search attributes..."
                value={attrSearch}
                onChange={(e) => setAttrSearch(e.target.value)}
                className="h-7 pl-7 text-xs"
              />
            </div>
            <Button
              variant="ghost"
              size="sm"
              className="h-7 w-7 p-0"
              onClick={() => setViewMode(viewMode === "table" ? "json" : "table")}
            >
              {viewMode === "table" ? <Code className="h-3 w-3" /> : <List className="h-3 w-3" />}
            </Button>
          </div>
          {viewMode === "table" ? (
            <AttributeTable data={span.spanAttributes} search={attrSearch} />
          ) : (
            <JsonTreeView data={span.spanAttributes} />
          )}
        </TabsContent>
        <TabsContent value="resource" className="mt-3">
          <AttributeTable data={span.resourceAttributes} search="" />
        </TabsContent>
        <TabsContent value="events" className="mt-3">
          {!span.events?.length ? (
            <p className="text-sm text-muted-foreground">No events</p>
          ) : (
            <div className="space-y-3">
              {span.events.map((evt, i) => (
                <div key={i} className="rounded-lg border p-2">
                  <div className="flex items-center justify-between">
                    <span className="font-medium text-sm">{evt.name}</span>
                    <span className="font-mono text-xs text-muted-foreground">
                      {new Date(evt.timestamp).toLocaleTimeString()}
                    </span>
                  </div>
                  {evt.attributes && Object.keys(evt.attributes).length > 0 && (
                    <div className="mt-1.5">
                      <AttributeTable data={evt.attributes} search="" />
                    </div>
                  )}
                </div>
              ))}
            </div>
          )}
        </TabsContent>
      </Tabs>
    </div>
  )
}

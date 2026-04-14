import { useTranslation } from "react-i18next"
import { Sheet, SheetContent, SheetHeader, SheetTitle } from "@/components/ui/sheet"
import { Badge } from "@/components/ui/badge"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import type { Span } from "../api"

interface SpanDetailSheetProps {
  span: Span | null
  open: boolean
  onOpenChange: (open: boolean) => void
}

function AttributeTable({ data }: { data: Record<string, string> | null | undefined }) {
  const entries = Object.entries(data ?? {})
  if (entries.length === 0) {
    return <p className="text-sm text-muted-foreground">No data</p>
  }
  return (
    <div className="space-y-1">
      {entries.map(([key, value]) => (
        <div key={key} className="flex gap-2 text-sm">
          <span className="shrink-0 font-mono text-xs text-muted-foreground">{key}</span>
          <span className="break-all font-mono text-xs">{value}</span>
        </div>
      ))}
    </div>
  )
}

export function SpanDetailSheet({ span, open, onOpenChange }: SpanDetailSheetProps) {
  const { t } = useTranslation("apm")

  if (!span) return null

  const durationMs = span.duration / 1e6
  const isError = span.statusCode === "STATUS_CODE_ERROR"

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className="w-[480px] sm:max-w-[480px] overflow-y-auto">
        <SheetHeader>
          <SheetTitle className="text-base">{t("detail.spanDetail")}</SheetTitle>
        </SheetHeader>

        <div className="mt-4 space-y-4">
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
            <div className="flex justify-between">
              <span className="text-muted-foreground">{t("detail.spanId")}</span>
              <span className="font-mono text-xs">{span.spanId}</span>
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
            <TabsContent value="attributes" className="mt-3">
              <AttributeTable data={span.spanAttributes} />
            </TabsContent>
            <TabsContent value="resource" className="mt-3">
              <AttributeTable data={span.resourceAttributes} />
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
                          <AttributeTable data={evt.attributes} />
                        </div>
                      )}
                    </div>
                  ))}
                </div>
              )}
            </TabsContent>
          </Tabs>
        </div>
      </SheetContent>
    </Sheet>
  )
}

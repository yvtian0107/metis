import { useState, useMemo } from "react"
import { useParams, Link } from "react-router"
import { useTranslation } from "react-i18next"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { ArrowLeft, RefreshCw, Network, TableProperties, FlaskConical } from "lucide-react"
import { api } from "@/lib/api"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs"
import { usePermission } from "@/hooks/use-permission"
import type { KnowledgeBaseDetail, NodeItem } from "./types"
import { CompileStatusBadge } from "./components/status-badges"
import { SourcesTab } from "./components/sources-tab"
import { KnowledgeGraphView } from "./components/knowledge-graph-view"
import { RecallPanel } from "./components/recall-panel"
import { NodeTableView } from "./components/node-table-view"
import { CompileLogsTab } from "./components/compile-logs-tab"
import { useCompileProgress } from "./hooks/use-compile-progress"

// ─── Knowledge Graph Tab (with Recall Panel) ─────────────────────────────────

function KnowledgeGraphTab({ kbId, compileMethod }: { kbId: number; compileMethod: string }) {
  const { t } = useTranslation("ai")
  const showGraphToggle = compileMethod === "knowledge_graph"
  const [view, setView] = useState<"graph" | "table">(showGraphToggle ? "graph" : "table")
  const [recallOpen, setRecallOpen] = useState(false)
  const [recallSearchQuery, setRecallSearchQuery] = useState("")

  const { data: recallData, isLoading: recallLoading } = useQuery({
    queryKey: ["ai-kb-recall", kbId, recallSearchQuery],
    queryFn: () => api.get<{ nodes: NodeItem[]; edges: unknown[] }>(
      `/api/v1/ai/knowledge-bases/${kbId}/search?q=${encodeURIComponent(recallSearchQuery)}&limit=20`,
    ),
    enabled: recallSearchQuery.length > 0 && recallOpen,
  })

  const highlightedNodeIds = useMemo(() => {
    if (!recallOpen || !recallData?.nodes) return new Set<string>()
    return new Set(recallData.nodes.map((r) => r.id))
  }, [recallOpen, recallData])

  function toggleRecall() {
    if (recallOpen) {
      setRecallSearchQuery("")
    }
    setRecallOpen(prev => !prev)
  }

  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-1">
          {showGraphToggle ? (
            <>
              <Button
                variant={view === "graph" ? "default" : "ghost"}
                size="sm"
                onClick={() => setView("graph")}
              >
                <Network className="mr-1.5 h-3.5 w-3.5" />
                {t("knowledge.graph.viewGraph")}
              </Button>
              <Button
                variant={view === "table" ? "default" : "ghost"}
                size="sm"
                onClick={() => setView("table")}
              >
                <TableProperties className="mr-1.5 h-3.5 w-3.5" />
                {t("knowledge.graph.viewTable")}
              </Button>
            </>
          ) : (
            <Button variant="default" size="sm" disabled>
              <TableProperties className="mr-1.5 h-3.5 w-3.5" />
              {t("knowledge.graph.viewTable")}
            </Button>
          )}
        </div>
        <Button
          variant={recallOpen ? "default" : "outline"}
          size="sm"
          onClick={toggleRecall}
        >
          <FlaskConical className="mr-1.5 h-3.5 w-3.5" />
          {t("knowledge.recall.title")}
        </Button>
      </div>
      <div className="flex gap-3">
        <div className="flex-1 min-w-0">
          {view === "graph" && showGraphToggle ? (
            <KnowledgeGraphView kbId={kbId} highlightedNodeIds={highlightedNodeIds} />
          ) : (
            <NodeTableView kbId={kbId} />
          )}
        </div>
        {recallOpen && (
          <RecallPanel
            kbId={kbId}
            results={recallData?.nodes ?? []}
            isLoading={recallLoading}
            hasSearched={recallSearchQuery.length > 0}
            onSearch={setRecallSearchQuery}
          />
        )}
      </div>
    </div>
  )
}

// ─── Main Page ────────────────────────────────────────────────────────────────

export function Component() {
  const { id } = useParams<{ id: string }>()
  const { t } = useTranslation(["ai", "common"])
  const queryClient = useQueryClient()
  const kbId = Number(id)

  const canCreate = usePermission("ai:knowledge:create")
  const canCompile = usePermission("ai:knowledge:compile")

  const { data: kb, isLoading } = useQuery({
    queryKey: ["ai-kb-detail", kbId],
    queryFn: () => api.get<KnowledgeBaseDetail>(`/api/v1/ai/knowledge-bases/${kbId}`),
    enabled: !Number.isNaN(kbId),
  })

  const { progress } = useCompileProgress({
    kbId,
    compileStatus: kb?.compileStatus ?? "idle",
    enabled: !Number.isNaN(kbId),
  })

  const compileMutation = useMutation({
    mutationFn: () => api.post(`/api/v1/ai/knowledge-bases/${kbId}/compile`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["ai-kb-detail", kbId] })
      toast.success(t("ai:knowledge.compileStarted"))
    },
    onError: (err) => toast.error(err.message),
  })

  const recompileMutation = useMutation({
    mutationFn: () => api.post(`/api/v1/ai/knowledge-bases/${kbId}/recompile`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["ai-kb-detail", kbId] })
      toast.success(t("ai:knowledge.compileStarted"))
    },
    onError: (err) => toast.error(err.message),
  })

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-48 text-sm text-muted-foreground">
        {t("common:loading")}
      </div>
    )
  }

  if (!kb) {
    return (
      <div className="flex items-center justify-center h-48 text-sm text-muted-foreground">
        {t("ai:knowledge.notFound")}
      </div>
    )
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-3">
        <Button variant="ghost" size="sm" asChild>
          <Link to="/ai/knowledge">
            <ArrowLeft className="h-4 w-4" />
          </Link>
        </Button>
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2">
            <h2 className="text-lg font-semibold truncate">{kb.name}</h2>
            <CompileStatusBadge status={kb.compileStatus} />
          </div>
          {kb.description && (
            <p className="text-sm text-muted-foreground truncate">{kb.description}</p>
          )}
        </div>
        <div className="flex items-center gap-2 shrink-0">
          <div className="flex items-center gap-3 text-sm text-muted-foreground mr-2">
            <span>{t("ai:knowledge.sourceCount")}: {kb.sourceCount}</span>
            <span>{t("ai:knowledge.nodeCount")}: {kb.nodeCount}</span>
            <span>{t("ai:knowledge.edgeCount")}: {kb.edgeCount}</span>
          </div>
          {canCompile && (
            <Button
              size="sm"
              variant="outline"
              disabled={compileMutation.isPending || recompileMutation.isPending || kb.compileStatus === "compiling"}
              onClick={() => {
                if (kb.compileStatus === "completed") {
                  recompileMutation.mutate()
                } else {
                  compileMutation.mutate()
                }
              }}
            >
              <RefreshCw className="mr-1.5 h-4 w-4" />
              {kb.compileStatus === "completed"
                ? t("ai:knowledge.recompile")
                : t("ai:knowledge.compile")}
            </Button>
          )}
        </div>
      </div>

      <Tabs defaultValue="sources">
        <TabsList>
          <TabsTrigger value="sources">{t("ai:knowledge.tabs.sources")}</TabsTrigger>
          <TabsTrigger value="graph">{t("ai:knowledge.tabs.graph")}</TabsTrigger>
          <TabsTrigger value="logs">{t("ai:knowledge.tabs.logs")}</TabsTrigger>
        </TabsList>
        <TabsContent value="sources" className="mt-4">
          <SourcesTab kbId={kbId} canCreate={canCreate} progress={progress} />
        </TabsContent>
        <TabsContent value="graph" className="mt-4">
          <KnowledgeGraphTab kbId={kbId} compileMethod={kb.compileMethod} />
        </TabsContent>
        <TabsContent value="logs" className="mt-4">
          <CompileLogsTab kbId={kbId} />
        </TabsContent>
      </Tabs>
    </div>
  )
}

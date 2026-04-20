import { useParams, useNavigate, Link } from "react-router"
import { useTranslation } from "react-i18next"
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { ArrowLeft, Bot, BrainCircuit, Code2, Pencil, Trash2, MessageSquare, Loader2 } from "lucide-react"
import { agentApi, sessionApi, api, type AgentWithBindings, type AgentSession, type PaginatedResponse } from "@/lib/api"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from "@/components/ui/table"
import {
  AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent,
  AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle, AlertDialogTrigger,
} from "@/components/ui/alert-dialog"
import { formatDateTime } from "@/lib/utils"

const TYPE_ICON: Record<string, typeof Bot> = {
  assistant: BrainCircuit,
  coding: Code2,
}

const SESSION_STATUS_VARIANT: Record<string, "default" | "secondary" | "outline" | "destructive"> = {
  running: "default",
  completed: "secondary",
  cancelled: "outline",
  error: "destructive",
}

interface NamedItem {
  id: number
  name: string
  displayName?: string
}

function useBindingNames(ids: number[], queryKey: string[], endpoint: string) {
  const { t } = useTranslation(["ai"])
  const { data: items = [] } = useQuery({
    queryKey,
    queryFn: () =>
      api.get<PaginatedResponse<NamedItem>>(endpoint).then((r) => r?.items ?? []),
    enabled: ids.length > 0,
  })
  return ids.map((id) => {
    const item = items.find((i) => i.id === id)
    if (!item) return `#${id}`
    return t(`ai:tools.toolDefs.${item.name}.name`, { defaultValue: item.displayName || item.name })
  })
}

function BindingBadges({ ids, queryKey, endpoint }: { ids: number[]; queryKey: string[]; endpoint: string }) {
  const names = useBindingNames(ids, queryKey, endpoint)

  if (ids.length === 0) return <span className="text-sm text-muted-foreground">-</span>

  return (
    <div className="flex flex-wrap gap-1.5">
      {names.map((name, i) => (
        <Badge key={ids[i]} variant="outline">{name}</Badge>
      ))}
    </div>
  )
}

function AgentConfiguration({ agent }: { agent: AgentWithBindings }) {
  const { t } = useTranslation(["ai"])

  return (
    <div className="space-y-6">
      {/* Basic Info */}
      <Card>
        <CardHeader className="pb-4">
          <CardTitle className="text-base">{t("ai:agents.sections.basic")}</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="grid grid-cols-2 gap-4 sm:grid-cols-3">
            <div>
              <span className="text-sm text-muted-foreground">{t("ai:agents.type")}</span>
              <p className="font-medium">{t(`ai:agents.agentTypes.${agent.type}`)}</p>
            </div>
            <div>
              <span className="text-sm text-muted-foreground">{t("ai:agents.visibility")}</span>
              <p className="font-medium">{t(`ai:agents.visibilityOptions.${agent.visibility}`)}</p>
            </div>
            {agent.description && (
              <div className="col-span-2 sm:col-span-3">
                <span className="text-sm text-muted-foreground">{t("ai:agents.description")}</span>
                <p className="text-sm mt-1">{agent.description}</p>
              </div>
            )}
          </div>
        </CardContent>
      </Card>

      {/* Model Config (assistant) */}
      {agent.type === "assistant" && (
        <Card>
          <CardHeader className="pb-4">
            <CardTitle className="text-base">{t("ai:agents.sections.modelConfig")}</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="grid grid-cols-2 gap-4 sm:grid-cols-3">
              <div>
                <span className="text-sm text-muted-foreground">{t("ai:agents.strategy")}</span>
                <p className="font-medium">{agent.strategy ? t(`ai:agents.strategies.${agent.strategy}`) : "-"}</p>
              </div>
              <div>
                <span className="text-sm text-muted-foreground">{t("ai:agents.temperature")}</span>
                <p className="font-medium">{agent.temperature}</p>
              </div>
              <div>
                <span className="text-sm text-muted-foreground">{t("ai:agents.maxTokens")}</span>
                <p className="font-medium">{agent.maxTokens}</p>
              </div>
              <div>
                <span className="text-sm text-muted-foreground">{t("ai:agents.maxTurns")}</span>
                <p className="font-medium">{agent.maxTurns}</p>
              </div>
            </div>
            {agent.systemPrompt && (
              <div className="mt-4">
                <span className="text-sm text-muted-foreground">{t("ai:agents.systemPrompt")}</span>
                <pre className="mt-1 text-sm whitespace-pre-wrap bg-muted/50 rounded-md p-3 max-h-64 overflow-auto">{agent.systemPrompt}</pre>
              </div>
            )}
          </CardContent>
        </Card>
      )}

      {/* Runtime Config (coding) */}
      {agent.type === "coding" && (
        <Card>
          <CardHeader className="pb-4">
            <CardTitle className="text-base">{t("ai:agents.sections.execution")}</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="grid grid-cols-2 gap-4 sm:grid-cols-3">
              <div>
                <span className="text-sm text-muted-foreground">{t("ai:agents.runtime")}</span>
                <p className="font-medium">{agent.runtime ? t(`ai:agents.runtimes.${agent.runtime}`) : "-"}</p>
              </div>
              <div>
                <span className="text-sm text-muted-foreground">{t("ai:agents.execMode")}</span>
                <p className="font-medium">{agent.execMode ? t(`ai:agents.execModes.${agent.execMode}`) : "-"}</p>
              </div>
              {agent.workspace && (
                <div className="col-span-2 sm:col-span-3">
                  <span className="text-sm text-muted-foreground">{t("ai:agents.workspace")}</span>
                  <p className="font-mono text-sm">{agent.workspace}</p>
                </div>
              )}
            </div>
          </CardContent>
        </Card>
      )}

      {/* Bindings */}
      <Card>
        <CardHeader className="pb-4">
          <CardTitle className="text-base">{t("ai:agents.bindings")}</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div>
            <h4 className="text-sm font-medium mb-2">{t("ai:agents.tools")}</h4>
            <BindingBadges ids={agent.toolIds} queryKey={["ai-agent-detail-tools"]} endpoint="/api/v1/ai/tools?pageSize=100" />
          </div>
          <div>
            <h4 className="text-sm font-medium mb-2">{t("ai:agents.mcpServers")}</h4>
            <BindingBadges ids={agent.mcpServerIds} queryKey={["ai-binding-mcp-servers"]} endpoint="/api/v1/ai/mcp-servers?pageSize=100" />
          </div>
          <div>
            <h4 className="text-sm font-medium mb-2">{t("ai:agents.skills")}</h4>
            <BindingBadges ids={agent.skillIds} queryKey={["ai-binding-skills"]} endpoint="/api/v1/ai/skills?pageSize=100" />
          </div>
          <div>
            <h4 className="text-sm font-medium mb-2">{t("ai:agents.knowledgeBases")}</h4>
            <BindingBadges ids={agent.knowledgeBaseIds} queryKey={["ai-binding-knowledge-bases"]} endpoint="/api/v1/ai/knowledge-bases?pageSize=100" />
          </div>
        </CardContent>
      </Card>

      {/* Instructions */}
      {agent.instructions && (
        <Card>
          <CardHeader className="pb-4">
            <CardTitle className="text-base">{t("ai:agents.sections.instructions")}</CardTitle>
          </CardHeader>
          <CardContent>
            <pre className="text-sm whitespace-pre-wrap bg-muted/50 rounded-md p-3 max-h-40 overflow-auto">{agent.instructions}</pre>
          </CardContent>
        </Card>
      )}
    </div>
  )
}

function AgentSessions({ agentId }: { agentId: number }) {
  const { t } = useTranslation(["ai", "common"])
  const queryClient = useQueryClient()

  const { data, isLoading } = useQuery({
    queryKey: ["ai-agent-sessions", agentId],
    queryFn: () => sessionApi.list({ agentId, pageSize: 50 }),
  })
  const sessions = data?.items ?? []

  const deleteMutation = useMutation({
    mutationFn: (sid: number) => sessionApi.delete(sid),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["ai-agent-sessions"] })
      toast.success(t("ai:chat.sessionDeleted"))
    },
    onError: (err) => toast.error(err.message),
  })

  if (isLoading) return <p className="text-sm text-muted-foreground py-4">{t("common:loading")}</p>

  if (sessions.length === 0) {
    return <p className="text-sm text-muted-foreground py-4">{t("ai:chat.noSessions")}</p>
  }

  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>ID</TableHead>
          <TableHead>{t("ai:chat.title")}</TableHead>
          <TableHead>{t("ai:agents.status")}</TableHead>
          <TableHead>{t("common:createdAt")}</TableHead>
          <TableHead className="w-[80px]" />
        </TableRow>
      </TableHeader>
      <TableBody>
        {sessions.map((s: AgentSession) => (
          <TableRow key={s.id}>
            <TableCell className="font-mono text-xs">{s.id}</TableCell>
            <TableCell>{s.title || "-"}</TableCell>
            <TableCell>
              <Badge variant={SESSION_STATUS_VARIANT[s.status] ?? "secondary"}>
                {t(`ai:chat.sessionStatus.${s.status}`)}
              </Badge>
            </TableCell>
            <TableCell className="text-sm text-muted-foreground whitespace-nowrap">
              {formatDateTime(s.createdAt)}
            </TableCell>
            <TableCell>
              <AlertDialog>
                <AlertDialogTrigger asChild>
                  <Button variant="ghost" size="sm" className="px-2 text-destructive hover:text-destructive">
                    <Trash2 className="h-3.5 w-3.5" />
                  </Button>
                </AlertDialogTrigger>
                <AlertDialogContent>
                  <AlertDialogHeader>
                    <AlertDialogTitle>{t("ai:chat.deleteSession")}</AlertDialogTitle>
                    <AlertDialogDescription>{t("ai:chat.deleteSessionDesc")}</AlertDialogDescription>
                  </AlertDialogHeader>
                  <AlertDialogFooter>
                    <AlertDialogCancel>{t("common:cancel")}</AlertDialogCancel>
                    <AlertDialogAction onClick={() => deleteMutation.mutate(s.id)} disabled={deleteMutation.isPending}>
                      {t("common:delete")}
                    </AlertDialogAction>
                  </AlertDialogFooter>
                </AlertDialogContent>
              </AlertDialog>
            </TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  )
}

export function Component() {
  const { id } = useParams<{ id: string }>()
  const { t } = useTranslation(["ai", "common"])
  const navigate = useNavigate()
  const queryClient = useQueryClient()

  const { data: agent, isLoading } = useQuery({
    queryKey: ["ai-agent", id],
    queryFn: () => agentApi.get(Number(id)),
    enabled: !!id,
  })

  const deleteMutation = useMutation({
    mutationFn: () => agentApi.delete(Number(id)),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["ai-agents"] })
      toast.success(t("ai:agents.deleteSuccess"))
      navigate("/ai/agents")
    },
    onError: (err) => toast.error(err.message),
  })

  const createSessionMutation = useMutation({
    mutationFn: () => sessionApi.create(Number(id)),
    onSuccess: (session) => navigate(`/ai/chat/${session.id}`),
    onError: (err) => toast.error(err.message),
  })

  if (isLoading || !agent) {
    return <div className="py-8 text-center text-muted-foreground">{t("common:loading")}</div>
  }

  const TypeIcon = TYPE_ICON[agent.type] ?? Bot

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-3">
        <Link to="/ai/agents">
          <Button variant="ghost" size="icon" className="h-8 w-8">
            <ArrowLeft className="h-4 w-4" />
          </Button>
        </Link>
        <TypeIcon className="h-5 w-5 text-muted-foreground" />
        <div className="flex-1">
          <h2 className="text-lg font-semibold">{agent.name}</h2>
          <div className="flex items-center gap-2 mt-0.5">
            <Badge variant="outline">{t(`ai:agents.agentTypes.${agent.type}`)}</Badge>
            <Badge variant={agent.isActive ? "default" : "secondary"}>
              {agent.isActive ? t("ai:statusLabels.active") : t("ai:statusLabels.inactive")}
            </Badge>
          </div>
        </div>
        <div className="flex items-center gap-2">
          <Button
            variant="outline"
            size="sm"
            disabled={!agent.isActive || createSessionMutation.isPending}
            onClick={() => createSessionMutation.mutate()}
          >
            {createSessionMutation.isPending ? (
              <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />
            ) : (
              <MessageSquare className="mr-1.5 h-3.5 w-3.5" />
            )}
            {t("ai:chat.startChat")}
          </Button>
          <Button variant="outline" size="sm" onClick={() => navigate(`/ai/agents/${id}/edit`)}>
            <Pencil className="mr-1.5 h-3.5 w-3.5" />
            {t("common:edit")}
          </Button>
          <AlertDialog>
            <AlertDialogTrigger asChild>
              <Button variant="outline" size="sm" className="text-destructive hover:text-destructive">
                <Trash2 className="mr-1.5 h-3.5 w-3.5" />
                {t("common:delete")}
              </Button>
            </AlertDialogTrigger>
            <AlertDialogContent>
              <AlertDialogHeader>
                <AlertDialogTitle>{t("ai:agents.deleteTitle")}</AlertDialogTitle>
                <AlertDialogDescription>{t("ai:agents.deleteDesc", { name: agent.name })}</AlertDialogDescription>
              </AlertDialogHeader>
              <AlertDialogFooter>
                <AlertDialogCancel>{t("common:cancel")}</AlertDialogCancel>
                <AlertDialogAction onClick={() => deleteMutation.mutate()} disabled={deleteMutation.isPending}>
                  {t("ai:agents.confirmDelete")}
                </AlertDialogAction>
              </AlertDialogFooter>
            </AlertDialogContent>
          </AlertDialog>
        </div>
      </div>

      <Tabs defaultValue="config">
        <TabsList>
          <TabsTrigger value="config">{t("ai:agents.tabs.config")}</TabsTrigger>
          <TabsTrigger value="sessions">{t("ai:agents.tabs.sessions")}</TabsTrigger>
        </TabsList>
        <TabsContent value="config" className="pt-4">
          <AgentConfiguration agent={agent} />
        </TabsContent>
        <TabsContent value="sessions" className="pt-4">
          <AgentSessions agentId={agent.id} />
        </TabsContent>
      </Tabs>
    </div>
  )
}

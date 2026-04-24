"use client";

import { useEffect, useRef, useState } from "react"
import { useNavigate, useSearchParams } from "react-router"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { Bot, Loader2, MessageSquare, PanelLeft, PanelLeftClose } from "lucide-react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { sessionApi } from "@/lib/api"
import { ChatHeader, SessionSidebar } from "@/components/chat-workspace"

const SIDEBAR_COLLAPSED_KEY = "ai-chat-sidebar-collapsed"

export function Component() {
  const { t } = useTranslation(["ai"])
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [searchParams] = useSearchParams()
  const startedRef = useRef(false)
  const [sidebarCollapsed, setSidebarCollapsed] = useState(() => {
    const saved = localStorage.getItem(SIDEBAR_COLLAPSED_KEY)
    return saved ? saved === "true" : false
  })

  const agentIdParam = searchParams.get("agentId")
  const agentId = agentIdParam ? Number(agentIdParam) : undefined
  const autostart = searchParams.get("autostart") === "1"

  const createMutation = useMutation({
    mutationFn: (id: number) => sessionApi.create(id),
    onSuccess: (session) => {
      queryClient.invalidateQueries({ queryKey: ["ai-sessions"] })
      navigate(`/ai/chat/${session.id}`, { replace: true })
    },
    onError: (err) => toast.error(err.message),
  })

  useEffect(() => {
    if (!autostart || !agentId || Number.isNaN(agentId) || startedRef.current) return
    startedRef.current = true
    createMutation.mutate(agentId)
  }, [agentId, autostart, createMutation])

  const toggleSidebar = () => {
    setSidebarCollapsed((prev) => {
      const next = !prev
      localStorage.setItem(SIDEBAR_COLLAPSED_KEY, String(next))
      return next
    })
  }

  return (
    <div className="flex h-full overflow-hidden">
      {agentId && <SessionSidebar agentId={agentId} collapsed={sidebarCollapsed} />}

      <div className="flex min-w-0 flex-1 flex-col bg-background">
        <ChatHeader
          identity={{
            title: t("ai:chat.newChat"),
            icon: <Bot className="size-4" />,
          }}
          leading={
            agentId ? (
              <Button variant="ghost" size="sm" className="h-8 w-8 p-0" onClick={toggleSidebar}>
                {sidebarCollapsed ? <PanelLeft className="h-4 w-4" /> : <PanelLeftClose className="h-4 w-4" />}
              </Button>
            ) : null
          }
        />

        <div className="flex flex-1 items-center justify-center px-4 pb-24">
          <div className="flex max-w-md flex-col items-center text-center">
            <div className="mb-4 flex h-14 w-14 items-center justify-center rounded-2xl border border-border/55 bg-background/70">
              {createMutation.isPending ? (
                <Loader2 className="h-7 w-7 animate-spin text-primary" />
              ) : (
                <Bot className="h-7 w-7 text-primary" />
              )}
            </div>
            <h2 className="mb-2 text-xl font-semibold">
              {createMutation.isPending ? t("ai:chat.creatingSession") : t("ai:chat.emptyShellTitle")}
            </h2>
            <p className="text-sm leading-6 text-muted-foreground">
              {agentId ? t("ai:chat.emptyShellHint") : t("ai:chat.selectAgentHint")}
            </p>
            {!agentId && (
              <Button className="mt-6" onClick={() => navigate("/ai/assistant-agents")}>
                <MessageSquare className="mr-1.5 h-4 w-4" />
                {t("ai:chat.selectAgent")}
              </Button>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}

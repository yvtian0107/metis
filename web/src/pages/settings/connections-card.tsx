import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { useTranslation } from "react-i18next"
import { api } from "@/lib/api"
import { Button } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"

interface Connection {
  id: number
  provider: string
  externalName: string
  externalEmail: string
  avatarUrl: string
  createdAt: string
}

interface ProviderInfo {
  providerKey: string
  displayName: string
  sortOrder: number
}

const providerLabels: Record<string, string> = {
  github: "GitHub",
  google: "Google",
}

const providerIcons: Record<string, string> = {
  github:
    "M12 2C6.477 2 2 6.484 2 12.017c0 4.425 2.865 8.18 6.839 9.504.5.092.682-.217.682-.483 0-.237-.008-.868-.013-1.703-2.782.605-3.369-1.343-3.369-1.343-.454-1.158-1.11-1.466-1.11-1.466-.908-.62.069-.608.069-.608 1.003.07 1.531 1.032 1.531 1.032.892 1.53 2.341 1.088 2.91.832.092-.647.35-1.088.636-1.338-2.22-.253-4.555-1.113-4.555-4.951 0-1.093.39-1.988 1.029-2.688-.103-.253-.446-1.272.098-2.65 0 0 .84-.27 2.75 1.026A9.564 9.564 0 0112 6.844c.85.004 1.705.115 2.504.337 1.909-1.296 2.747-1.027 2.747-1.027.546 1.379.202 2.398.1 2.651.64.7 1.028 1.595 1.028 2.688 0 3.848-2.339 4.695-4.566 4.943.359.309.678.92.678 1.855 0 1.338-.012 2.419-.012 2.747 0 .268.18.58.688.482A10.019 10.019 0 0022 12.017C22 6.484 17.522 2 12 2z",
  google:
    "M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92a5.06 5.06 0 01-2.2 3.32v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.1zM12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23zM5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62zM12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z",
}

function ProviderIcon({ provider }: { provider: string }) {
  const path = providerIcons[provider]
  if (!path) return null
  return (
    <svg className="h-4 w-4" viewBox="0 0 24 24" fill="currentColor">
      <path d={path} />
    </svg>
  )
}

export function ConnectionsCard() {
  const { t } = useTranslation(["settings", "common"])
  const queryClient = useQueryClient()

  const { data: connections = [], isLoading } = useQuery({
    queryKey: ["connections"],
    queryFn: () => api.get<Connection[]>("/api/v1/auth/connections"),
  })

  const { data: providers = [] } = useQuery({
    queryKey: ["auth-providers"],
    queryFn: () => api.get<ProviderInfo[]>("/auth/providers"),
  })

  const unbindMutation = useMutation({
    mutationFn: (provider: string) =>
      api.delete(`/api/v1/auth/connections/${provider}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["connections"] })
    },
  })

  async function handleBind(providerKey: string) {
    try {
      const data = await api.post<{ authURL: string; state: string }>(
        `/api/v1/auth/connections/${providerKey}`,
      )
      sessionStorage.setItem("oauth_provider", providerKey)
      sessionStorage.setItem("oauth_bind", "1")
      window.location.assign(data.authURL)
    } catch {
      // ignore
    }
  }

  if (isLoading) {
    return (
      <Card>
        <CardContent className="flex h-32 items-center justify-center text-muted-foreground">
          {t("common:loading")}
        </CardContent>
      </Card>
    )
  }

  const boundProviders = new Set(connections.map((c) => c.provider))
  const unboundProviders = providers.filter(
    (p) => !boundProviders.has(p.providerKey),
  )

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("settings:connections.title")}</CardTitle>
        <CardDescription>{t("settings:connections.description")}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        {connections.map((conn) => (
          <div
            key={conn.id}
            className="flex items-center justify-between rounded-lg border p-3"
          >
            <div className="flex items-center gap-3">
              <ProviderIcon provider={conn.provider} />
              <div>
                <p className="text-sm font-medium">
                  {providerLabels[conn.provider] ?? conn.provider}
                </p>
                <p className="text-xs text-muted-foreground">
                  {conn.externalName}
                  {conn.externalEmail ? ` (${conn.externalEmail})` : ""}
                </p>
              </div>
            </div>
            <Button
              variant="outline"
              size="sm"
              onClick={() => unbindMutation.mutate(conn.provider)}
              disabled={unbindMutation.isPending}
            >
              {t("settings:connections.unbind")}
            </Button>
          </div>
        ))}

        {unboundProviders.map((p) => (
          <div
            key={p.providerKey}
            className="flex items-center justify-between rounded-lg border border-dashed p-3"
          >
            <div className="flex items-center gap-3">
              <ProviderIcon provider={p.providerKey} />
              <div>
                <p className="text-sm font-medium">{p.displayName}</p>
                <p className="text-xs text-muted-foreground">{t("settings:connections.unbound")}</p>
              </div>
            </div>
            <Button
              variant="outline"
              size="sm"
              onClick={() => handleBind(p.providerKey)}
            >
              {t("settings:connections.bind")}
            </Button>
          </div>
        ))}

        {connections.length === 0 && unboundProviders.length === 0 && (
          <p className="text-sm text-muted-foreground">{t("settings:connections.noProviders")}</p>
        )}
      </CardContent>
    </Card>
  )
}

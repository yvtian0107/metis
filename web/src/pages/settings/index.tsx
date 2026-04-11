import { useTranslation } from "react-i18next"
import { useQuery } from "@tanstack/react-query"
import { api, type SiteInfo } from "@/lib/api"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { addKernelNamespace } from "@/i18n"
import zhCNSettings from "@/i18n/locales/zh-CN/settings.json"
import enSettings from "@/i18n/locales/en/settings.json"
import { SiteNameCard } from "./site-name-card"
import { LogoCard } from "./logo-card"
import { SecurityCard } from "./security-card"
import { SchedulerCard } from "./scheduler-card"
import { ConnectionsCard } from "./connections-card"

addKernelNamespace("settings", zhCNSettings, enSettings)

export function Component() {
  const { t } = useTranslation(["settings", "common"])
  const { data, isLoading } = useQuery({
    queryKey: ["site-info"],
    queryFn: () => api.get<SiteInfo>("/api/v1/site-info"),
  })

  if (isLoading) {
    return (
      <div className="flex h-64 items-center justify-center text-muted-foreground">
        {t("common:loading")}
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <h2 className="text-lg font-semibold">{t("settings:pageTitle")}</h2>
      <Tabs defaultValue="site">
        <TabsList>
          <TabsTrigger value="site">{t("settings:tabs.site")}</TabsTrigger>
          <TabsTrigger value="security">{t("settings:tabs.security")}</TabsTrigger>
          <TabsTrigger value="scheduler">{t("settings:tabs.scheduler")}</TabsTrigger>
          <TabsTrigger value="connections">{t("settings:tabs.connections")}</TabsTrigger>
        </TabsList>
        <TabsContent value="site" className="space-y-6 mt-4">
          <SiteNameCard appName={data?.appName ?? "Metis"} />
          <LogoCard hasLogo={data?.hasLogo ?? false} />
        </TabsContent>
        <TabsContent value="security" className="mt-4">
          <SecurityCard />
        </TabsContent>
        <TabsContent value="scheduler" className="mt-4">
          <SchedulerCard />
        </TabsContent>
        <TabsContent value="connections" className="mt-4">
          <ConnectionsCard />
        </TabsContent>
      </Tabs>
    </div>
  )
}

import { useTranslation } from "react-i18next"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { addKernelNamespace } from "@/i18n"
import zhCN from "@/i18n/locales/zh-CN/audit.json"
import en from "@/i18n/locales/en/audit.json"
import { AuthTab } from "./auth-tab"
import { OperationTab } from "./operation-tab"

addKernelNamespace("audit", zhCN, en)

export function Component() {
  const { t } = useTranslation("audit")

  return (
    <div className="space-y-4">
      <h2 className="text-lg font-semibold">{t("title")}</h2>

      <Tabs defaultValue="auth">
        <TabsList>
          <TabsTrigger value="auth">{t("tabs.auth")}</TabsTrigger>
          <TabsTrigger value="operation">{t("tabs.operation")}</TabsTrigger>
        </TabsList>
        <TabsContent value="auth">
          <AuthTab />
        </TabsContent>
        <TabsContent value="operation">
          <OperationTab />
        </TabsContent>
      </Tabs>
    </div>
  )
}

import { useTranslation } from "react-i18next"
import { MCPServersTab } from "./components/mcp-servers-tab"

export function Component() {
  const { t } = useTranslation("ai")

  return (
    <div className="space-y-4">
      <h2 className="text-lg font-semibold">{t("tools.navigation.mcp")}</h2>
      <MCPServersTab />
    </div>
  )
}

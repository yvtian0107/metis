import { useTranslation } from "react-i18next"
import { BuiltinToolsTab } from "./components/builtin-tools-tab"

export function Component() {
  const { t } = useTranslation("ai")

  return (
    <div className="workspace-page">
      <div className="workspace-page-header">
        <div className="min-w-0">
          <h2 className="workspace-page-title">{t("tools.navigation.builtin")}</h2>
          <p className="workspace-page-description">{t("tools.builtin.pageDesc")}</p>
        </div>
      </div>
      <BuiltinToolsTab />
    </div>
  )
}

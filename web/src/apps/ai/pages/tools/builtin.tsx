import { useTranslation } from "react-i18next"
import { BuiltinToolsTab } from "./components/builtin-tools-tab"

export function Component() {
  const { t } = useTranslation("ai")

  return (
    <div className="space-y-4">
      <h2 className="text-lg font-semibold">{t("tools.navigation.builtin")}</h2>
      <BuiltinToolsTab />
    </div>
  )
}

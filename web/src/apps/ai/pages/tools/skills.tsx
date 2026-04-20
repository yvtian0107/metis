import { useTranslation } from "react-i18next"
import { SkillsTab } from "./components/skills-tab"

export function Component() {
  const { t } = useTranslation("ai")

  return (
    <div className="space-y-4">
      <h2 className="text-lg font-semibold">{t("tools.navigation.skills")}</h2>
      <SkillsTab />
    </div>
  )
}

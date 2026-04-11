import { FileQuestion } from "lucide-react"
import { Link } from "react-router"
import { useTranslation } from "react-i18next"
import { Button } from "@/components/ui/button"

export default function NotFoundPage() {
  const { t } = useTranslation()
  return (
    <div className="flex min-h-screen items-center justify-center">
      <div className="flex flex-col items-center gap-4 text-muted-foreground">
        <FileQuestion className="h-16 w-16" />
        <h1 className="text-4xl font-bold text-foreground">404</h1>
        <p className="text-sm">{t("notFound")}</p>
        <Button asChild variant="outline">
          <Link to="/">{t("backToHome")}</Link>
        </Button>
      </div>
    </div>
  )
}

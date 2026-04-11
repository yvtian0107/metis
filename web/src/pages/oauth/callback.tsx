import { useEffect, useState } from "react"
import { useNavigate, useSearchParams } from "react-router"
import { useTranslation } from "react-i18next"
import { useAuthStore } from "@/stores/auth"
import { api } from "@/lib/api"

export function Component() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const oauthLogin = useAuthStore((s) => s.oauthLogin)
  const [error, setError] = useState("")

  useEffect(() => {
    const code = searchParams.get("code")
    const state = searchParams.get("state")

    if (!code || !state) {
      navigate("/login", { replace: true })
      return
    }

    const provider = sessionStorage.getItem("oauth_provider") || "github"
    sessionStorage.removeItem("oauth_provider")

    api
      .post<{
        accessToken: string
        refreshToken: string
        expiresIn: number
        permissions: string[]
      }>("/auth/oauth/callback", { provider, code, state })
      .then((data) => {
        oauthLogin(data).then(() => navigate("/", { replace: true }))
      })
      .catch((err) => {
        const msg = err instanceof Error ? err.message : "OAuth login failed"
        setError(msg)
        setTimeout(() => navigate("/login", { replace: true, state: { error: msg } }), 3000)
      })
  }, [searchParams, navigate, oauthLogin])

  if (error) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-background">
        <div className="text-center space-y-2">
          <p className="text-sm text-destructive">{error}</p>
          <p className="text-xs text-muted-foreground">{t("redirectingToLogin")}</p>
        </div>
      </div>
    )
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-background">
      <p className="text-sm text-muted-foreground">{t("loggingIn")}</p>
    </div>
  )
}

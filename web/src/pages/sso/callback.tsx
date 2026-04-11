import { useEffect, useState } from "react"
import { useNavigate, useSearchParams, Link } from "react-router"
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

    async function run() {
      try {
        const data = await api.post<{
          accessToken: string
          refreshToken: string
          expiresIn: number
          permissions: string[]
        }>("/api/v1/auth/sso/callback", { code, state })

        await oauthLogin(data)
        navigate("/", { replace: true })
      } catch (err) {
        const msg = err instanceof Error ? err.message : t("ssoLoginFailed")
        setError(msg)
      }
    }

    run()
  }, [searchParams, navigate, oauthLogin, t])

  if (error) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-background">
        <div className="text-center space-y-3">
          <p className="text-sm text-destructive">{error}</p>
          <Link
            to="/login"
            className="text-sm text-primary underline underline-offset-4 hover:text-primary/80"
          >
            {t("backToLogin")}
          </Link>
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

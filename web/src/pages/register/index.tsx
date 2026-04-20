import { useState } from "react"
import { useNavigate, Navigate, Link } from "react-router"
import { useTranslation } from "react-i18next"

import { AuthShell } from "@/components/auth/auth-shell"
import { AuthBrandLockup } from "@/components/auth/brand-lockup"
import { useAuthStore } from "@/stores/auth"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"

export function Component() {
  const navigate = useNavigate()
  const user = useAuthStore((s) => s.user)
  const oauthLogin = useAuthStore((s) => s.oauthLogin)
  const { t } = useTranslation("auth")

  const [username, setUsername] = useState("")
  const [password, setPassword] = useState("")
  const [confirmPassword, setConfirmPassword] = useState("")
  const [email, setEmail] = useState("")
  const [error, setError] = useState("")
  const [loading, setLoading] = useState(false)

  if (user) {
    return <Navigate to="/" replace />
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setError("")

    if (password !== confirmPassword) {
      setError(t("register.passwordMismatch"))
      return
    }

    setLoading(true)
    try {
      const res = await fetch("/api/v1/auth/register", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ username, password, email }),
      })

      const body = await res.json()
      if (!res.ok || body.code !== 0) {
        throw new Error(body.message || t("register.registerFailed"))
      }

      // Auto-login after registration
      await oauthLogin(body.data)
      navigate("/", { replace: true })
    } catch (err) {
      setError(err instanceof Error ? err.message : t("register.registerFailed"))
    } finally {
      setLoading(false)
    }
  }

  return (
    <AuthShell>
      <div className="w-full max-w-[32rem] lg:max-w-[28rem]">
        <div className="auth-panel-glass rounded-[1.5rem] px-5 py-5 sm:rounded-[1.75rem] sm:px-6 sm:py-6 lg:px-8 lg:py-8">
          <div className="mb-8 space-y-5">
            <AuthBrandLockup appName="Metis" compact />
            <div className="space-y-2">
              <h1 className="text-[1.625rem] font-semibold tracking-[-0.04em] text-foreground sm:text-[1.875rem] lg:text-[2rem]">
                {t("register.title")}
              </h1>
              <p className="text-sm leading-6 text-muted-foreground">{t("register.subtitle")}</p>
            </div>
          </div>

          <form onSubmit={handleSubmit} className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="username" className="text-foreground/78">{t("register.username")}</Label>
              <Input
                id="username"
                name="username"
                autoComplete="username"
                spellCheck={false}
                placeholder={t("register.usernamePlaceholder")}
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                required
                autoFocus
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="email" className="text-foreground/78">{t("register.email")}</Label>
              <Input
                id="email"
                name="email"
                type="email"
                autoComplete="email"
                placeholder={t("register.emailPlaceholder")}
                value={email}
                onChange={(e) => setEmail(e.target.value)}
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="password" className="text-foreground/78">{t("register.password")}</Label>
              <Input
                id="password"
                name="password"
                type="password"
                autoComplete="new-password"
                placeholder={t("register.passwordPlaceholder")}
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                required
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="confirmPassword" className="text-foreground/78">{t("register.confirmPassword")}</Label>
              <Input
                id="confirmPassword"
                name="confirmPassword"
                type="password"
                autoComplete="new-password"
                placeholder={t("register.confirmPasswordPlaceholder")}
                value={confirmPassword}
                onChange={(e) => setConfirmPassword(e.target.value)}
                required
              />
            </div>

            {error ? <p className="rounded-2xl border border-destructive/18 bg-destructive/6 px-3.5 py-3 text-sm text-destructive">{error}</p> : null}

            <Button type="submit" className="h-11 w-full rounded-xl" disabled={loading}>
              {loading ? t("register.submitting") : t("register.submit")}
            </Button>
          </form>

          <p className="mt-6 text-center text-sm text-muted-foreground">
            {t("register.hasAccount")}{" "}
            <Link to="/login" className="font-medium text-foreground transition hover:text-primary">
              {t("register.login")}
            </Link>
          </p>
        </div>
      </div>
    </AuthShell>
  )
}

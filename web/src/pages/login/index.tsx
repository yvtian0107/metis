import { useState, useEffect, useCallback } from "react"
import { useTranslation } from "react-i18next"
import { useQuery } from "@tanstack/react-query"
import { useNavigate, Navigate, Link } from "react-router"

import { AuthShell } from "@/components/auth/auth-shell"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { api, type SiteInfo } from "@/lib/api"
import { useAuthStore, TwoFactorRequiredError, AccountLockedError } from "@/stores/auth"

interface AuthProviderInfo {
  providerKey: string
  displayName: string
  sortOrder: number
}

interface CaptchaData {
  enabled: boolean
  id?: string
  image?: string
}

const providerIcons: Record<string, string> = {
  github: "M12 2C6.477 2 2 6.484 2 12.017c0 4.425 2.865 8.18 6.839 9.504.5.092.682-.217.682-.483 0-.237-.008-.868-.013-1.703-2.782.605-3.369-1.343-3.369-1.343-.454-1.158-1.11-1.466-1.11-1.466-.908-.62.069-.608.069-.608 1.003.07 1.531 1.032 1.531 1.032.892 1.53 2.341 1.088 2.91.832.092-.647.35-1.088.636-1.338-2.22-.253-4.555-1.113-4.555-4.951 0-1.093.39-1.988 1.029-2.688-.103-.253-.446-1.272.098-2.65 0 0 .84-.27 2.75 1.026A9.564 9.564 0 0112 6.844c.85.004 1.705.115 2.504.337 1.909-1.296 2.747-1.027 2.747-1.027.546 1.379.202 2.398.1 2.651.64.7 1.028 1.595 1.028 2.688 0 3.848-2.339 4.695-4.566 4.943.359.309.678.92.678 1.855 0 1.338-.012 2.419-.012 2.747 0 .268.18.58.688.482A10.019 10.019 0 0022 12.017C22 6.484 17.522 2 12 2z",
  google: "M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92a5.06 5.06 0 01-2.2 3.32v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.1zM12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23zM5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62zM12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z",
}

function ProviderIcon({ provider }: { provider: string }) {
  const path = providerIcons[provider]
  if (!path) return null
  return (
    <svg className="mr-2 h-4 w-4" viewBox="0 0 24 24" fill="currentColor">
      <path d={path} />
    </svg>
  )
}

export default function LoginPage() {
  const { t } = useTranslation("auth")
  const navigate = useNavigate()
  const login = useAuthStore((s) => s.login)
  const user = useAuthStore((s) => s.user)

  const { data: siteInfo } = useQuery({
    queryKey: ["site-info"],
    queryFn: () => api.get<SiteInfo>("/api/v1/site-info"),
    staleTime: 60_000,
  })

  const [username, setUsername] = useState("")
  const [password, setPassword] = useState("")
  const [captchaAnswer, setCaptchaAnswer] = useState("")
  const [captcha, setCaptcha] = useState<CaptchaData | null>(null)
  const [error, setError] = useState("")
  const [loading, setLoading] = useState(false)
  const [providers, setProviders] = useState<AuthProviderInfo[]>([])
  const [registrationOpen, setRegistrationOpen] = useState(false)

  const loadCaptcha = useCallback(async () => {
    try {
      const data = await api.get<CaptchaData>("/api/v1/captcha")
      setCaptcha(data)
      setCaptchaAnswer("")
    } catch {
      setCaptcha(null)
    }
  }, [])

  useEffect(() => {
    api.get<AuthProviderInfo[]>("/api/v1/auth/providers").then(setProviders).catch(() => {})
    api.get<{ registrationOpen: boolean }>("/api/v1/auth/registration-status")
      .then((data) => setRegistrationOpen(data.registrationOpen))
      .catch(() => {})
    loadCaptcha()
  }, [loadCaptcha])

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setError("")
    setLoading(true)

    try {
      await login(username, password, captcha?.id, captchaAnswer)
      navigate("/", { replace: true })
    } catch (err) {
      if (err instanceof TwoFactorRequiredError) {
        navigate("/2fa", { state: { twoFactorToken: err.twoFactorToken } })
        return
      }
      if (err instanceof AccountLockedError) {
        setError(t("login.accountLocked"))
      } else {
        setError(err instanceof Error ? err.message : t("login.loginFailed"))
      }
      loadCaptcha()
    } finally {
      setLoading(false)
    }
  }

  async function handleOAuth(providerKey: string) {
    setError("")
    try {
      const data = await api.get<{ authURL: string; state: string }>(`/api/v1/auth/oauth/${providerKey}`)
      sessionStorage.setItem("oauth_provider", providerKey)
      window.location.assign(data.authURL)
    } catch (err) {
      setError(err instanceof Error ? err.message : t("login.oauthFailed"))
    }
  }

  const appName = siteInfo?.appName ?? "Metis"

  if (user) {
    return <Navigate to="/" replace />
  }

  return (
    <AuthShell>
      <div className="w-full max-w-[25rem]">
        <div className="auth-panel-glass rounded-3xl px-8 py-8 sm:px-10 sm:py-9">
          {/* Heading */}
          <div className="mb-6 text-center">
            <h2 className="text-lg font-semibold tracking-[-0.02em] text-slate-900">
              {t("login.title", { appName })}
            </h2>
            <p className="mt-1 text-[13px] text-slate-400">
              {t("login.subtitle")}
            </p>
          </div>

          {/* Form — flat, no wrapper card */}
          <form onSubmit={handleSubmit} className="space-y-4">
            <div className="space-y-3">
              <div>
                <Label htmlFor="username" className="mb-1.5 block text-[13px] font-medium text-slate-500">
                  {t("login.username")}
                </Label>
                <Input
                  id="username"
                  placeholder="username"
                  value={username}
                  onChange={(e) => setUsername(e.target.value)}
                  required
                  autoFocus
                  className="auth-input"
                />
              </div>

              <div>
                <Label htmlFor="password" className="mb-1.5 block text-[13px] font-medium text-slate-500">
                  {t("login.password")}
                </Label>
                <Input
                  id="password"
                  type="password"
                  placeholder="••••••••"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  required
                  className="auth-input"
                />
              </div>

              {captcha?.enabled && (
                <div>
                  <Label htmlFor="captcha" className="mb-1.5 block text-[13px] font-medium text-slate-500">
                    {t("login.captcha")}
                  </Label>
                  <div className="flex items-center gap-2">
                    <Input
                      id="captcha"
                      placeholder={t("login.captchaPlaceholder")}
                      value={captchaAnswer}
                      onChange={(e) => setCaptchaAnswer(e.target.value)}
                      className="auth-input flex-1"
                      required
                    />
                    {captcha.image && (
                      <img
                        src={captcha.image}
                        alt="captcha"
                        className="h-10 w-24 cursor-pointer rounded-xl border border-slate-200/60 bg-white object-cover"
                        onClick={loadCaptcha}
                        title={t("login.captchaRefresh")}
                      />
                    )}
                  </div>
                </div>
              )}
            </div>

            {error && (
              <div className="rounded-xl bg-red-50 px-3.5 py-2.5 text-[13px] leading-snug text-red-600">
                {error}
              </div>
            )}

            <div className="space-y-4 pt-1">
              <Button
                type="submit"
                className="h-[2.625rem] w-full rounded-xl border-0 bg-slate-900 text-sm font-medium tracking-[-0.01em] text-white shadow-[0_1px_2px_rgba(0,0,0,0.05),inset_0_1px_0_rgba(255,255,255,0.06)] hover:bg-slate-800 active:scale-[0.985]"
                disabled={loading}
              >
                {loading ? t("login.loggingIn") : t("login.submit")}
              </Button>

              {providers.length > 0 && (
                <>
                  <div className="relative">
                    <div className="absolute inset-0 flex items-center">
                      <span className="w-full border-t border-slate-200/60" />
                    </div>
                    <div className="relative flex justify-center">
                      <span className="bg-white/60 px-2.5 text-[11px] font-medium tracking-wide text-slate-300 uppercase">
                        {t("login.or")}
                      </span>
                    </div>
                  </div>

                  <div className="grid gap-2">
                    {providers.map((provider) => (
                      <Button
                        key={provider.providerKey}
                        variant="outline"
                        className="h-10 rounded-xl border-slate-200/70 bg-white text-[13px] font-medium text-slate-600 shadow-none hover:border-slate-300 hover:bg-slate-50 hover:text-slate-900"
                        onClick={() => handleOAuth(provider.providerKey)}
                      >
                        <ProviderIcon provider={provider.providerKey} />
                        {provider.displayName}
                      </Button>
                    ))}
                  </div>
                </>
              )}
            </div>
          </form>

          {/* Footer */}
          <div className="mt-6 text-center text-[12.5px] text-slate-400">
            {registrationOpen ? (
              <p>
                {t("login.noAccount")}{" "}
                <Link to="/register" className="font-medium text-slate-600 transition hover:text-slate-900">
                  {t("login.createAccount")}
                </Link>
              </p>
            ) : (
              <p>{t("login.contactAdmin")}</p>
            )}
          </div>
        </div>
      </div>
      {siteInfo?.version && (
        <div className="fixed bottom-3 right-4 text-[11px] text-slate-300">
          {siteInfo.version}
        </div>
      )}
    </AuthShell>
  )
}

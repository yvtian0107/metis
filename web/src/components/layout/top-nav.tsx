import { useState, useEffect } from "react"
import { useNavigate } from "react-router"
import { useQuery } from "@tanstack/react-query"
import { useTranslation } from "react-i18next"
import { PanelLeft, LogOut, KeyRound, ShieldCheck, Globe, Clock, ChevronDown, Check } from "lucide-react"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuSub,
  DropdownMenuSubTrigger,
  DropdownMenuSubContent,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { useUiStore } from "@/stores/ui"
import { useAuthStore, type User } from "@/stores/auth"
import { api, type SiteInfo } from "@/lib/api"
import { supportedLocales, changeLocale } from "@/i18n"
import { ChangePasswordDialog } from "@/components/change-password-dialog"
import { TwoFactorSetupDialog } from "@/components/two-factor-setup-dialog"
import { NotificationBell } from "@/components/notification-bell"
import { cn } from "@/lib/utils"

const COMMON_TIMEZONES = [
  "UTC",
  "America/New_York", "America/Chicago", "America/Denver", "America/Los_Angeles",
  "America/Sao_Paulo", "America/Toronto",
  "Europe/London", "Europe/Berlin", "Europe/Paris", "Europe/Moscow",
  "Asia/Shanghai", "Asia/Tokyo", "Asia/Seoul", "Asia/Singapore",
  "Asia/Hong_Kong", "Asia/Taipei", "Asia/Dubai", "Asia/Kolkata",
  "Australia/Sydney", "Pacific/Auckland",
]

export function TopNav() {
  const { t } = useTranslation("layout")
  const toggleSidebar = useUiStore((s) => s.toggleSidebar)
  const user = useAuthStore((s) => s.user)
  const setUser = useAuthStore((s) => s.setUser)
  const logout = useAuthStore((s) => s.logout)
  const navigate = useNavigate()

  const { data: siteInfo } = useQuery({
    queryKey: ["site-info"],
    queryFn: () => api.get<SiteInfo>("/api/v1/site-info"),
    staleTime: 60_000,
  })

  const [pwdDialogOpen, setPwdDialogOpen] = useState(false)
  const [tfaDialogOpen, setTfaDialogOpen] = useState(false)

  const { i18n } = useTranslation()
  const currentTimezone = user?.timezone || Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC"

  // Listen for password-expired events from api.ts 409 interceptor
  useEffect(() => {
    function handleExpired(e: Event) {
      const msg = (e as CustomEvent).detail?.message || t("passwordExpired")
      toast.warning(msg)
      setPwdDialogOpen(true)
    }
    window.addEventListener("password-expired", handleExpired)
    return () => window.removeEventListener("password-expired", handleExpired)
  }, [t])

  async function handleLocaleChange(locale: string) {
    changeLocale(locale)
    if (user) {
      try {
        const updated = await api.put<User>("/api/v1/auth/profile", { locale })
        setUser(updated)
      } catch {
        // UI already switched
      }
    }
  }

  async function handleTimezoneChange(timezone: string) {
    if (user) {
      try {
        const updated = await api.put<User>("/api/v1/auth/profile", { timezone })
        setUser(updated)
        toast.success(t("pref.saved"))
      } catch {
        // ignore
      }
    }
  }

  async function handleLogout() {
    await logout()
    navigate("/login", { replace: true })
  }

  return (
    <>
      <header
        className={cn(
          "fixed inset-x-0 top-0 z-30 flex h-14 items-center gap-3 border-b border-border/50 px-4",
          "bg-white/60 backdrop-blur-2xl",
        )}
      >
        <Button
          variant="ghost"
          size="icon"
          className="h-8 w-8 text-muted-foreground"
          onClick={toggleSidebar}
        >
          <PanelLeft className="h-4 w-4" />
        </Button>

        <div className="flex items-center gap-2">
          {siteInfo?.hasLogo && (
            <img
              src="/api/v1/site-info/logo"
              alt="Logo"
              width={28}
              height={28}
              className="h-7 w-7 rounded object-contain"
            />
          )}
          <span className="workspace-chrome-brand">
            {siteInfo?.appName ?? "Metis"}
          </span>
        </div>

        <div className="ml-auto flex items-center gap-2">
          <NotificationBell />
          {user && (
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <button className="workspace-chrome-trigger flex items-center gap-1.5 rounded-full border border-transparent px-2.5 py-1.5 text-muted-foreground transition-colors hover:border-border/70 hover:bg-white/80 hover:text-foreground">
                  <span>{user.username}</span>
                  <ChevronDown className="h-3.5 w-3.5" />
                </button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end" className="w-52">
                <DropdownMenuItem onClick={() => setPwdDialogOpen(true)}>
                  <KeyRound className="mr-2 h-4 w-4" />
                  {t("changePassword")}
                </DropdownMenuItem>
                <DropdownMenuItem onClick={() => setTfaDialogOpen(true)}>
                  <ShieldCheck className="mr-2 h-4 w-4" />
                  {t("twoFactor")}
                </DropdownMenuItem>
                <DropdownMenuSeparator />

                {/* Language sub-menu */}
                <DropdownMenuSub>
                  <DropdownMenuSubTrigger>
                    <Globe className="mr-2 h-4 w-4" />
                    {t("pref.language")}
                  </DropdownMenuSubTrigger>
                  <DropdownMenuSubContent>
                    {supportedLocales.map((loc) => (
                      <DropdownMenuItem
                        key={loc.code}
                        onClick={() => handleLocaleChange(loc.code)}
                      >
                        {i18n.language === loc.code && <Check className="mr-2 h-4 w-4" />}
                        {i18n.language !== loc.code && <span className="mr-2 w-4" />}
                        {loc.name}
                      </DropdownMenuItem>
                    ))}
                  </DropdownMenuSubContent>
                </DropdownMenuSub>

                {/* Timezone sub-menu */}
                <DropdownMenuSub>
                  <DropdownMenuSubTrigger>
                    <Clock className="mr-2 h-4 w-4" />
                    {t("pref.timezone")}
                  </DropdownMenuSubTrigger>
                  <DropdownMenuSubContent className="max-h-64 overflow-y-auto">
                    {COMMON_TIMEZONES.map((tz) => (
                      <DropdownMenuItem
                        key={tz}
                        onClick={() => handleTimezoneChange(tz)}
                      >
                        {currentTimezone === tz && <Check className="mr-2 h-4 w-4 shrink-0" />}
                        {currentTimezone !== tz && <span className="mr-2 w-4 shrink-0" />}
                        <span className="truncate">{tz}</span>
                      </DropdownMenuItem>
                    ))}
                  </DropdownMenuSubContent>
                </DropdownMenuSub>

                <DropdownMenuSeparator />
                <DropdownMenuItem onClick={handleLogout} className="text-destructive focus:text-destructive">
                  <LogOut className="mr-2 h-4 w-4" />
                  {t("logout")}
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          )}
        </div>
      </header>

      <ChangePasswordDialog open={pwdDialogOpen} onOpenChange={setPwdDialogOpen} />
      <TwoFactorSetupDialog
        open={tfaDialogOpen}
        onOpenChange={setTfaDialogOpen}
        enabled={user?.twoFactorEnabled ?? false}
      />
    </>
  )
}

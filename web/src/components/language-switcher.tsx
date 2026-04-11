import { useTranslation } from "react-i18next"
import { Globe } from "lucide-react"
import { supportedLocales, changeLocale } from "@/i18n"
import { useAuthStore, type User } from "@/stores/auth"
import { api } from "@/lib/api"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"

export function LanguageSwitcher({ className }: { className?: string }) {
  const { i18n } = useTranslation()
  const user = useAuthStore((s) => s.user)
  const setUser = useAuthStore((s) => s.setUser)
  const current = supportedLocales.find((l) => l.code === i18n.language)

  async function handleChange(locale: string) {
    changeLocale(locale)
    // Persist to user profile if logged in
    if (user) {
      try {
        const updated = await api.put<User>("/api/v1/auth/profile", { locale })
        setUser(updated)
      } catch {
        // UI already switched, ignore persist failure
      }
    }
  }

  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        className={`inline-flex items-center gap-1.5 rounded-lg px-2.5 py-1.5 text-[13px] font-medium text-slate-500 transition hover:text-slate-700 hover:bg-slate-100/50 ${className ?? ""}`}
      >
        <Globe className="h-3.5 w-3.5" />
        {current?.name}
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        {supportedLocales.map((loc) => (
          <DropdownMenuItem
            key={loc.code}
            onClick={() => handleChange(loc.code)}
            className={i18n.language === loc.code ? "font-medium" : ""}
          >
            {loc.name}
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

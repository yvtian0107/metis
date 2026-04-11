import { clsx, type ClassValue } from "clsx"
import { twMerge } from "tailwind-merge"
import { useAuthStore } from "@/stores/auth"

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

// Cached site info from last fetch
let _siteLocale = ""
let _siteTimezone = ""

export function setSiteLocaleTimezone(locale: string, timezone: string) {
  _siteLocale = locale
  _siteTimezone = timezone
}

/**
 * Resolve effective locale: user.locale → system.locale → browser → zh-CN
 */
export function resolveLocale(): string {
  const user = useAuthStore.getState().user
  if (user?.locale) return user.locale
  if (_siteLocale) return _siteLocale
  return navigator.language || "zh-CN"
}

/**
 * Resolve effective timezone: user.timezone → system.timezone → browser → UTC
 */
export function resolveTimezone(): string {
  const user = useAuthStore.getState().user
  if (user?.timezone) return user.timezone
  if (_siteTimezone) return _siteTimezone
  try {
    return Intl.DateTimeFormat().resolvedOptions().timeZone
  } catch {
    return "UTC"
  }
}

/**
 * Format a date/time value using resolved locale and timezone.
 * Optionally override locale/timezone for specific use cases.
 */
export function formatDateTime(
  value: string | number | Date,
  options?: { locale?: string; timezone?: string; dateOnly?: boolean },
) {
  const locale = options?.locale || resolveLocale()
  const timeZone = options?.timezone || resolveTimezone()

  const formatOptions: Intl.DateTimeFormatOptions = {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    timeZone,
    ...(options?.dateOnly
      ? {}
      : {
          hour: "2-digit",
          minute: "2-digit",
          hour12: false,
        }),
  }

  return new Intl.DateTimeFormat(locale, formatOptions).format(new Date(value))
}

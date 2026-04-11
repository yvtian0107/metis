import i18n from "i18next"
import { initReactI18next } from "react-i18next"

import zhCNCommon from "./locales/zh-CN/common.json"
import enCommon from "./locales/en/common.json"
import zhCNErrors from "./locales/zh-CN/errors.json"
import enErrors from "./locales/en/errors.json"
import zhCNInstall from "./locales/zh-CN/install.json"
import enInstall from "./locales/en/install.json"
import zhCNAuth from "./locales/zh-CN/auth.json"
import enAuth from "./locales/en/auth.json"
import zhCNLayout from "./locales/zh-CN/layout.json"
import enLayout from "./locales/en/layout.json"
import zhCNSessions from "./locales/zh-CN/sessions.json"
import enSessions from "./locales/en/sessions.json"
import zhCNUsers from "./locales/zh-CN/users.json"
import enUsers from "./locales/en/users.json"
import zhCNRoles from "./locales/zh-CN/roles.json"
import enRoles from "./locales/en/roles.json"
import zhCNMenus from "./locales/zh-CN/menus.json"
import enMenus from "./locales/en/menus.json"
import zhCNSettings from "./locales/zh-CN/settings.json"
import enSettings from "./locales/en/settings.json"
import zhCNTasks from "./locales/zh-CN/tasks.json"
import enTasks from "./locales/en/tasks.json"
import zhCNAudit from "./locales/zh-CN/audit.json"
import enAudit from "./locales/en/audit.json"
import zhCNAnnouncements from "./locales/zh-CN/announcements.json"
import enAnnouncements from "./locales/en/announcements.json"
import zhCNChannels from "./locales/zh-CN/channels.json"
import enChannels from "./locales/en/channels.json"
import zhCNAuthProviders from "./locales/zh-CN/authProviders.json"
import enAuthProviders from "./locales/en/authProviders.json"
import zhCNIdentitySources from "./locales/zh-CN/identitySources.json"
import enIdentitySources from "./locales/en/identitySources.json"

export const supportedLocales = [
  { code: "zh-CN", name: "简体中文" },
  { code: "en", name: "English" },
] as const

export type SupportedLocale = (typeof supportedLocales)[number]["code"]

const LOCALE_KEY = "metis_locale"

/**
 * Resolve locale using priority: user preference → system default → browser → zh-CN.
 * The user/system values are injected later via setLocaleFromUser/setLocaleFromSystem.
 */
function resolveInitialLocale(): string {
  // 1. Check localStorage (set by language switcher or after login)
  const stored = localStorage.getItem(LOCALE_KEY)
  if (stored && supportedLocales.some((l) => l.code === stored)) {
    return stored
  }

  // 2. Browser language
  const browserLang = navigator.language
  if (supportedLocales.some((l) => l.code === browserLang)) {
    return browserLang
  }
  // Try prefix match (e.g., "zh" → "zh-CN")
  const prefix = browserLang.split("-")[0]
  const match = supportedLocales.find((l) => l.code.startsWith(prefix))
  if (match) {
    return match.code
  }

  // 3. Fallback
  return "zh-CN"
}

i18n.use(initReactI18next).init({
  lng: resolveInitialLocale(),
  fallbackLng: "zh-CN",
  supportedLngs: supportedLocales.map((l) => l.code),
  ns: ["common", "errors", "install", "auth", "layout", "sessions", "users", "roles", "menus", "settings", "tasks", "audit", "announcements", "channels", "authProviders", "identitySources"],
  defaultNS: "common",
  resources: {
    "zh-CN": {
      common: zhCNCommon, errors: zhCNErrors, install: zhCNInstall, auth: zhCNAuth,
      layout: zhCNLayout, sessions: zhCNSessions, users: zhCNUsers, roles: zhCNRoles,
      menus: zhCNMenus, settings: zhCNSettings, tasks: zhCNTasks, audit: zhCNAudit,
      announcements: zhCNAnnouncements, channels: zhCNChannels,
      authProviders: zhCNAuthProviders, identitySources: zhCNIdentitySources,
    },
    en: {
      common: enCommon, errors: enErrors, install: enInstall, auth: enAuth,
      layout: enLayout, sessions: enSessions, users: enUsers, roles: enRoles,
      menus: enMenus, settings: enSettings, tasks: enTasks, audit: enAudit,
      announcements: enAnnouncements, channels: enChannels,
      authProviders: enAuthProviders, identitySources: enIdentitySources,
    },
  },
  interpolation: {
    escapeValue: false, // React already handles XSS
  },
})

/**
 * Register translations for an App module namespace.
 * Call this in each App's module.ts to register its translations.
 */
export function registerTranslations(
  ns: string,
  resources: Record<string, object>,
) {
  for (const [lang, bundle] of Object.entries(resources)) {
    i18n.addResourceBundle(lang, ns, bundle, true, true)
  }
}

/**
 * Add a namespace bundle for kernel pages (used in i18n/locales imports).
 */
export function addKernelNamespace(
  ns: string,
  zhCN: object,
  en: object,
) {
  i18n.addResourceBundle("zh-CN", ns, zhCN, true, true)
  i18n.addResourceBundle("en", ns, en, true, true)
}

/**
 * Change the active locale and persist to localStorage.
 */
export function changeLocale(locale: string) {
  localStorage.setItem(LOCALE_KEY, locale)
  i18n.changeLanguage(locale)
}

/**
 * Get the current persisted locale key from localStorage.
 */
export function getPersistedLocale(): string | null {
  return localStorage.getItem(LOCALE_KEY)
}

export default i18n

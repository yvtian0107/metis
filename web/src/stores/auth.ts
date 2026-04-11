import { create } from "zustand"
import { useMenuStore } from "./menu"

export interface RoleInfo {
  id: number
  name: string
  code: string
}

export interface User {
  id: number
  username: string
  email: string
  phone: string
  avatar: string
  locale: string
  timezone: string
  role: RoleInfo
  isActive: boolean
  twoFactorEnabled: boolean
  createdAt: string
  updatedAt: string
}

interface TokenPair {
  accessToken: string
  refreshToken: string
  expiresIn: number
  permissions: string[]
  needsTwoFactor?: boolean
  twoFactorToken?: string
  requireTwoFactorSetup?: boolean
}

// Thrown when the server responds with 202 (2FA required)
export class TwoFactorRequiredError extends Error {
  twoFactorToken: string
  constructor(token: string) {
    super("2FA required")
    this.name = "TwoFactorRequiredError"
    this.twoFactorToken = token
  }
}

// Thrown when the account is locked (423)
export class AccountLockedError extends Error {
  constructor(message: string) {
    super(message)
    this.name = "AccountLockedError"
  }
}

interface AuthState {
  accessToken: string | null
  refreshToken: string | null
  user: User | null
  initialized: boolean
  requireTwoFactorSetup: boolean

  setTokens: (pair: TokenPair) => void
  setUser: (user: User) => void
  clear: () => void
  init: () => Promise<void>
  login: (username: string, password: string, captchaId?: string, captchaAnswer?: string) => Promise<void>
  oauthLogin: (pair: TokenPair) => Promise<void>
  logout: () => Promise<void>
  refresh: () => Promise<boolean>
}

const TOKEN_KEY = "metis_access_token"
const REFRESH_KEY = "metis_refresh_token"

export const useAuthStore = create<AuthState>((set, get) => ({
  accessToken: null,
  refreshToken: null,
  user: null,
  initialized: false,
  requireTwoFactorSetup: false,

  setTokens: (pair) => {
    localStorage.setItem(TOKEN_KEY, pair.accessToken)
    localStorage.setItem(REFRESH_KEY, pair.refreshToken)
    set({ accessToken: pair.accessToken, refreshToken: pair.refreshToken })
  },

  setUser: (user) => set({ user }),

  clear: () => {
    localStorage.removeItem(TOKEN_KEY)
    localStorage.removeItem(REFRESH_KEY)
    set({ accessToken: null, refreshToken: null, user: null, requireTwoFactorSetup: false })
    useMenuStore.getState().clear()
  },

  init: async () => {
    const accessToken = localStorage.getItem(TOKEN_KEY)
    const refreshToken = localStorage.getItem(REFRESH_KEY)

    if (!accessToken || !refreshToken) {
      set({ initialized: true })
      return
    }

    set({ accessToken, refreshToken })

    // Try to fetch current user
    try {
      const res = await fetch("/api/v1/auth/me", {
        headers: { Authorization: `Bearer ${accessToken}` },
      })

      if (res.ok) {
        const body = await res.json()
        set({ user: body.data.user, initialized: true })
        // Load menu tree
        await useMenuStore.getState().init()
        return
      }

      // Access token expired, try refresh
      if (res.status === 401) {
        const refreshed = await get().refresh()
        if (refreshed) {
          // Retry with new token
          const newToken = get().accessToken
          const retryRes = await fetch("/api/v1/auth/me", {
            headers: { Authorization: `Bearer ${newToken}` },
          })
          if (retryRes.ok) {
            const body = await retryRes.json()
            set({ user: body.data.user, initialized: true })
            await useMenuStore.getState().init()
            return
          }
        }
      }
    } catch {
      // Network error, clear tokens
    }

    get().clear()
    set({ initialized: true })
  },

  login: async (username, password, captchaId?, captchaAnswer?) => {
    const res = await fetch("/api/v1/auth/login", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ username, password, captchaId, captchaAnswer }),
    })

    const body = await res.json()

    // 2FA required
    if (res.status === 202 && body.needsTwoFactor) {
      throw new TwoFactorRequiredError(body.twoFactorToken)
    }

    // Account locked
    if (res.status === 423) {
      throw new AccountLockedError(body.message || "account locked")
    }

    if (!res.ok || body.code !== 0) {
      throw new Error(body.message || "Login failed")
    }

    get().setTokens(body.data)

    // Store requireTwoFactorSetup flag if server mandates 2FA setup
    if (body.data.requireTwoFactorSetup) {
      set({ requireTwoFactorSetup: true })
    }

    // Fetch user profile and menu tree in parallel
    const meRes = await fetch("/api/v1/auth/me", {
      headers: { Authorization: `Bearer ${body.data.accessToken}` },
    })
    if (meRes.ok) {
      const meBody = await meRes.json()
      set({ user: meBody.data.user })
    }

    await useMenuStore.getState().init()
  },

  oauthLogin: async (pair) => {
    get().setTokens(pair)

    const meRes = await fetch("/api/v1/auth/me", {
      headers: { Authorization: `Bearer ${pair.accessToken}` },
    })
    if (meRes.ok) {
      const meBody = await meRes.json()
      set({ user: meBody.data.user })
    }

    await useMenuStore.getState().init()
  },

  logout: async () => {
    const { refreshToken, accessToken } = get()
    if (refreshToken && accessToken) {
      try {
        await fetch("/api/v1/auth/logout", {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
            Authorization: `Bearer ${accessToken}`,
          },
          body: JSON.stringify({ refreshToken }),
        })
      } catch {
        // Ignore logout errors
      }
    }
    get().clear()
  },

  refresh: async () => {
    const { refreshToken } = get()
    if (!refreshToken) return false

    try {
      const res = await fetch("/api/v1/auth/refresh", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ refreshToken }),
      })

      if (!res.ok) return false

      const body = await res.json()
      if (body.code !== 0) return false

      get().setTokens(body.data)
      return true
    } catch {
      return false
    }
  },
}))

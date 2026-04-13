import { api } from "@/lib/api"

export interface TokenResponse {
  id: number
  name: string
  token: string
  scope: string
  lastUsedAt: string | null
  createdAt: string
}

export interface CreateTokenResponse extends TokenResponse {}

export const observeApi = {
  listTokens: () => api.get<TokenResponse[]>("/api/v1/observe/tokens"),

  createToken: (name: string) =>
    api.post<CreateTokenResponse>("/api/v1/observe/tokens", { name }),

  revokeToken: (id: number) =>
    api.delete<null>(`/api/v1/observe/tokens/${id}`),

  getSettings: () =>
    api.get<{ otelEndpoint: string }>("/api/v1/observe/settings"),
}

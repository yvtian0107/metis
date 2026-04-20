export interface ProviderBrand {
  stripe: string
  avatarBg: string
  avatarText: string
  label: string
}

const PROVIDER_BRAND_MAP: Record<string, ProviderBrand> = {
  openai: {
    stripe: "bg-emerald-500",
    avatarBg: "bg-emerald-50 text-emerald-700",
    avatarText: "OA",
    label: "OpenAI",
  },
  anthropic: {
    stripe: "bg-amber-500",
    avatarBg: "bg-amber-50 text-amber-700",
    avatarText: "AP",
    label: "Anthropic",
  },
  ollama: {
    stripe: "bg-sky-500",
    avatarBg: "bg-sky-50 text-sky-700",
    avatarText: "OL",
    label: "Ollama",
  },
}

const FALLBACK_BRAND: ProviderBrand = {
  stripe: "bg-primary",
  avatarBg: "bg-primary/10 text-primary",
  avatarText: "??",
  label: "Unknown",
}

export function getProviderBrand(type: string): ProviderBrand {
  return PROVIDER_BRAND_MAP[type] ?? FALLBACK_BRAND
}

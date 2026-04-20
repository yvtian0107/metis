import openaiLogo from "@lobehub/icons-static-svg/icons/openai.svg"
import anthropicLogo from "@lobehub/icons-static-svg/icons/anthropic.svg"
import ollamaLogo from "@lobehub/icons-static-svg/icons/ollama.svg"

const PROVIDER_LOGOS: Record<string, string> = {
  openai: openaiLogo,
  anthropic: anthropicLogo,
  ollama: ollamaLogo,
}

export function ProviderLogo({ type, label, className }: { type: string; label: string; className?: string }) {
  const src = PROVIDER_LOGOS[type]

  if (!src) {
    return <span className={className}>{label.slice(0, 2).toUpperCase()}</span>
  }

  return <img src={src} alt={label} className={className} />
}

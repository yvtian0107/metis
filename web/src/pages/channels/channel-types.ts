export interface ConfigField {
  key: string
  labelKey: string
  type: "string" | "number" | "boolean"
  required?: boolean
  default?: unknown
  sensitive?: boolean
  placeholderKey?: string
}

export interface ChannelTypeDef {
  labelKey: string
  icon: string
  configSchema: ConfigField[]
}

export const CHANNEL_TYPES: Record<string, ChannelTypeDef> = {
  email: {
    labelKey: "channelType.email",
    icon: "Mail",
    configSchema: [
      { key: "host", labelKey: "channelType.smtpHost", type: "string", required: true, placeholderKey: "smtp.example.com" },
      { key: "port", labelKey: "channelType.port", type: "number", required: true, default: 465 },
      { key: "secure", labelKey: "channelType.sslTls", type: "boolean", default: true },
      { key: "username", labelKey: "channelType.username", type: "string", required: true, placeholderKey: "user@example.com" },
      { key: "password", labelKey: "channelType.password", type: "string", required: true, sensitive: true },
      { key: "from", labelKey: "channelType.from", type: "string", required: true, placeholderKey: "channelType.fromPlaceholder" },
    ],
  },
}

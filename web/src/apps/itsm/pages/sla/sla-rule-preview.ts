import type {
  EscalationRuleItem,
  NotificationChannelOption,
  PriorityItem,
  SLATemplateItem,
} from "../../api"

interface EscalationParticipant {
  type: string
  value?: string
  id?: string | number
  name?: string
  position_code?: string
  department_code?: string
}

interface EscalationTargetConfig {
  recipients?: EscalationParticipant[]
  channelId?: number
  subjectTemplate?: string
  bodyTemplate?: string
  assigneeCandidates?: EscalationParticipant[]
  priorityId?: number
}

type SLARuleRiskCode =
  | "inactive"
  | "duplicate_level"
  | "notify_missing_recipients"
  | "notify_missing_channel"
  | "reassign_missing_candidates"
  | "priority_missing"
  | "priority_inactive"

type SLARuleRiskTone = "neutral" | "warning" | "danger"

const triggerOrder: Record<string, number> = {
  response_timeout: 0,
  resolution_timeout: 1,
}

interface SLARulePreviewRow {
  rule: EscalationRuleItem
  sla: SLATemplateItem
  targetSummary: string
  riskCodes: SLARuleRiskCode[]
  riskTone: SLARuleRiskTone
}

interface BuildSLARulePreviewRowsInput {
  slas: SLATemplateItem[]
  rulesBySlaId: Record<number, EscalationRuleItem[]>
  priorities: PriorityItem[]
  channels: NotificationChannelOption[]
}

function readTargetConfig(value: unknown): EscalationTargetConfig {
  if (!value || typeof value !== "object") return {}
  return value as EscalationTargetConfig
}

function participantLabel(p: EscalationParticipant) {
  if (p.type === "requester_manager") return p.name ?? "提交人上级"
  if (p.type === "requester") return p.name ?? "提交人"
  if (p.type === "position_department") {
    return [p.department_code, p.position_code].filter(Boolean).join(" / ") || "岗位+部门"
  }
  return p.name ?? p.value ?? p.type
}

function formatParticipants(items: EscalationParticipant[] | undefined) {
  if (!items || items.length === 0) return "未配置"
  return items.map(participantLabel).join("、")
}

function ruleTargetSummary(
  rule: EscalationRuleItem,
  priorities: PriorityItem[],
  channels: NotificationChannelOption[],
) {
  const cfg = readTargetConfig(rule.targetConfig)
  if (rule.actionType === "notify") {
    const channel = channels.find((item) => item.id === cfg.channelId)
    return `${formatParticipants(cfg.recipients)} / ${channel?.name ?? (cfg.channelId ? `#${cfg.channelId}` : "未配置")}`
  }
  if (rule.actionType === "reassign") return formatParticipants(cfg.assigneeCandidates)
  if (rule.actionType === "escalate_priority") {
    const priority = priorities.find((item) => item.id === cfg.priorityId)
    return priority ? `${priority.name} / ${priority.code}` : cfg.priorityId ? `#${cfg.priorityId}` : "未配置"
  }
  return "未配置"
}

function riskTone(codes: SLARuleRiskCode[]): SLARuleRiskTone {
  if (codes.length === 0) return "neutral"
  if (codes.includes("inactive")) return codes.length === 1 ? "warning" : "danger"
  return "danger"
}

function buildSLARulePreviewRows({
  slas,
  rulesBySlaId,
  priorities,
  channels,
}: BuildSLARulePreviewRowsInput): SLARulePreviewRow[] {
  const duplicateKeys = new Set<string>()
  const seenKeys = new Set<string>()

  for (const sla of slas) {
    for (const rule of rulesBySlaId[sla.id] ?? []) {
      const key = `${sla.id}:${rule.triggerType}:${rule.level}`
      if (seenKeys.has(key)) duplicateKeys.add(key)
      seenKeys.add(key)
    }
  }

  return slas.flatMap((sla) => {
    const rules = (rulesBySlaId[sla.id] ?? []).slice().sort((a, b) => (
      (triggerOrder[a.triggerType] ?? 99) - (triggerOrder[b.triggerType] ?? 99)
      || a.level - b.level
      || a.waitMinutes - b.waitMinutes
    ))
    return rules.map((rule) => {
      const cfg = readTargetConfig(rule.targetConfig)
      const codes: SLARuleRiskCode[] = []
      const duplicateKey = `${sla.id}:${rule.triggerType}:${rule.level}`

      if (!rule.isActive) codes.push("inactive")
      if (duplicateKeys.has(duplicateKey)) codes.push("duplicate_level")

      if (rule.actionType === "notify") {
        if (!cfg.recipients || cfg.recipients.length === 0) codes.push("notify_missing_recipients")
        if (!cfg.channelId || !channels.some((item) => item.id === cfg.channelId)) codes.push("notify_missing_channel")
      } else if (rule.actionType === "reassign") {
        if (!cfg.assigneeCandidates || cfg.assigneeCandidates.length === 0) codes.push("reassign_missing_candidates")
      } else if (rule.actionType === "escalate_priority") {
        const priority = priorities.find((item) => item.id === cfg.priorityId)
        if (!priority) {
          codes.push("priority_missing")
        } else if (!priority.isActive) {
          codes.push("priority_inactive")
        }
      }

      return {
        rule,
        sla,
        targetSummary: ruleTargetSummary(rule, priorities, channels),
        riskCodes: codes,
        riskTone: riskTone(codes),
      }
    })
  })
}

export {
  buildSLARulePreviewRows,
  readTargetConfig,
  ruleTargetSummary,
}
export type {
  EscalationParticipant,
  EscalationTargetConfig,
  SLARulePreviewRow,
  SLARuleRiskCode,
  SLARuleRiskTone,
}

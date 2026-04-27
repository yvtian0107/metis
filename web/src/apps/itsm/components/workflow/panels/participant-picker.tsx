import { type ReactNode, useState } from "react"
import { useTranslation } from "react-i18next"
import { useQuery } from "@tanstack/react-query"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Plus, Trash2, User, Building2, Briefcase, UserCheck, Users } from "lucide-react"
import { api } from "@/lib/api"
import { cn } from "@/lib/utils"
import type { Participant } from "../types"

interface ParticipantPickerProps {
  participants: Participant[]
  onChange: (participants: Participant[]) => void
}

interface UserItem {
  id: number
  username: string
  email: string
  avatar: string
}

interface PositionItem {
  id: number
  name: string
  code: string
}

interface DeptTreeNode {
  id: number
  name: string
  code: string
  children?: DeptTreeNode[]
}

const PARTICIPANT_TYPES = [
  { value: "user", icon: User, label: "workflow.participant.user" },
  { value: "position", icon: Briefcase, label: "workflow.participant.position" },
  { value: "department", icon: Building2, label: "workflow.participant.department" },
  { value: "position_department", icon: Users, label: "workflow.participant.positionDepartment" },
  { value: "requester_manager", icon: UserCheck, label: "workflow.participant.requesterManager" },
] as const

function formatParticipantLabel(p: Participant): string {
  if (p.type === "position_department") {
    const parts = [p.department_code, p.position_code].filter(Boolean)
    if (parts.length > 0) return parts.join(" / ")
  }
  return p.name ?? p.value ?? p.type
}

function ParticipantTypePill({ children }: { children: ReactNode }) {
  return (
    <span className="inline-flex h-5 shrink-0 items-center rounded-full border border-border/60 bg-background/45 px-2 text-[10px] font-medium text-muted-foreground">
      {children}
    </span>
  )
}

function PickerList({ children, className }: { children: ReactNode; className?: string }) {
  return (
    <div className={cn("workspace-picker-scrollbar max-h-40 overflow-y-auto rounded-md border border-border/55 bg-background/35 p-1", className)}>
      {children}
    </div>
  )
}

function PickerOption({
  children,
  depth = 0,
  onClick,
}: {
  children: ReactNode
  depth?: number
  onClick: () => void
}) {
  return (
    <button
      type="button"
      className="flex min-h-8 w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-[13px] leading-none text-foreground/82 transition-colors hover:bg-accent/45 hover:text-foreground focus-visible:bg-accent/55 focus-visible:outline-none"
      style={{ paddingLeft: `${depth * 12 + 8}px` }}
      onClick={onClick}
    >
      {children}
    </button>
  )
}

export function ParticipantPicker({ participants, onChange }: ParticipantPickerProps) {
  const { t } = useTranslation(["itsm", "common"])
  const [addingType, setAddingType] = useState<string>("")
  const [userKeyword, setUserKeyword] = useState("")
  const [pdDeptCode, setPdDeptCode] = useState("")

  const { data: users } = useQuery({
    queryKey: ["users-search", userKeyword],
    queryFn: () => api.get<{ items: UserItem[] }>(`/api/v1/users?page=1&pageSize=20&keyword=${encodeURIComponent(userKeyword)}`).then((r) => r.items),
    enabled: addingType === "user" && userKeyword.length > 0,
    staleTime: 30_000,
  })

  const { data: positions } = useQuery({
    queryKey: ["org-positions"],
    queryFn: () => api.get<{ items: PositionItem[] }>("/api/v1/org/positions?pageSize=0").then((r) => r.items),
    enabled: addingType === "position" || addingType === "position_department",
    staleTime: 60_000,
  })

  const { data: departments } = useQuery({
    queryKey: ["org-departments-tree"],
    queryFn: () => api.get<{ items: DeptTreeNode[] }>("/api/v1/org/departments/tree").then((r) => r.items),
    enabled: addingType === "department" || addingType === "position_department",
    staleTime: 60_000,
  })

  function addParticipant(p: Participant) {
    onChange([...participants, p])
    setAddingType("")
    setUserKeyword("")
    setPdDeptCode("")
  }

  function removeParticipant(index: number) {
    onChange(participants.filter((_, i) => i !== index))
  }

  function handleTypeSelect(type: string) {
    if (type === "requester_manager") {
      addParticipant({ type: "requester_manager", name: t("workflow.participant.requesterManager") })
      return
    }
    if (type === "position_department") {
      setAddingType("position_department")
      return
    }
    setAddingType(type)
  }

  const flatDepts = departments ? flattenDeptTree(departments) : []
  const participantTypeLabel = (type: string) => {
    const matched = PARTICIPANT_TYPES.find((item) => item.value === type)
    return matched ? t(`itsm:${matched.label}`) : type
  }

  return (
    <div className="space-y-2.5">
      <Label className="text-xs font-medium text-foreground/72">{t("workflow.prop.participants")}</Label>

      {participants.length > 0 && (
        <div className="space-y-1.5">
          {participants.map((p, i) => (
            <div key={i} className="flex min-h-8 items-center justify-between gap-2 rounded-md border border-border/55 bg-background/35 px-2.5 py-1.5">
              <div className="flex min-w-0 items-center gap-1.5">
                <ParticipantTypePill>{participantTypeLabel(p.type)}</ParticipantTypePill>
                <span className="truncate text-xs font-medium text-foreground/78">{formatParticipantLabel(p)}</span>
              </div>
              <Button type="button" variant="ghost" size="icon-xs" className="text-muted-foreground hover:text-foreground" onClick={() => removeParticipant(i)}>
                <Trash2 className="h-3 w-3" />
              </Button>
            </div>
          ))}
        </div>
      )}

      {!addingType ? (
        <Select onValueChange={handleTypeSelect}>
          <SelectTrigger className="h-8 w-full bg-background/35 text-xs">
            <div className="flex items-center gap-1.5">
              <Plus className="h-3.5 w-3.5 text-muted-foreground" />
              <SelectValue placeholder={t("workflow.participant.add")} />
            </div>
          </SelectTrigger>
          <SelectContent>
            {PARTICIPANT_TYPES.map((pt) => (
              <SelectItem key={pt.value} value={pt.value}>
                <div className="flex items-center gap-1.5">
                  <pt.icon size={12} />
                  <span>{t(pt.label)}</span>
                </div>
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      ) : addingType === "user" ? (
        <div className="space-y-2">
          <Input
            value={userKeyword}
            onChange={(e) => setUserKeyword(e.target.value)}
            placeholder={t("workflow.participant.searchUser")}
            className="h-8 bg-background/35 text-xs"
            autoFocus
          />
          {users && users.length > 0 && (
            <PickerList>
              {users.map((u) => (
                <PickerOption
                  key={u.id}
                  onClick={() => addParticipant({ type: "user", id: u.id, name: u.username, value: String(u.id) })}
                >
                  <User className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
                  <span className="min-w-0 truncate font-medium">{u.username}</span>
                  <span className="min-w-0 truncate text-muted-foreground">{u.email}</span>
                </PickerOption>
              ))}
            </PickerList>
          )}
          <Button type="button" variant="ghost" size="xs" className="text-muted-foreground" onClick={() => { setAddingType(""); setUserKeyword("") }}>
            {t("common:cancel")}
          </Button>
        </div>
      ) : addingType === "position" ? (
        <div className="space-y-2">
          <PickerList>
            {(positions ?? []).map((p) => (
              <PickerOption
                key={p.id}
                onClick={() => addParticipant({ type: "position", id: p.id, name: p.name, value: String(p.id) })}
              >
                <Briefcase className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
                <span className="min-w-0 truncate font-medium">{p.name}</span>
                <span className="shrink-0 text-muted-foreground">{p.code}</span>
              </PickerOption>
            ))}
          </PickerList>
          <Button type="button" variant="ghost" size="xs" className="text-muted-foreground" onClick={() => setAddingType("")}>
            {t("common:cancel")}
          </Button>
        </div>
      ) : addingType === "department" ? (
        <div className="space-y-2">
          <PickerList>
            {flatDepts.map((d) => (
              <PickerOption
                key={d.id}
                depth={d.depth ?? 0}
                onClick={() => addParticipant({ type: "department", id: d.id, name: d.name, value: String(d.id) })}
              >
                <Building2 className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
                <span className="min-w-0 truncate font-medium">{d.name}</span>
                <span className="shrink-0 text-muted-foreground">{d.code}</span>
              </PickerOption>
            ))}
          </PickerList>
          <Button type="button" variant="ghost" size="xs" className="text-muted-foreground" onClick={() => setAddingType("")}>
            {t("common:cancel")}
          </Button>
        </div>
      ) : addingType === "position_department" ? (
        <div className="space-y-2">
          {!pdDeptCode ? (
            <>
              <Label className="text-[11px] font-medium text-muted-foreground">{t("workflow.participant.department")}</Label>
              <PickerList>
                {flatDepts.map((d) => (
                  <PickerOption
                    key={d.id}
                    depth={d.depth ?? 0}
                    onClick={() => setPdDeptCode(d.code)}
                  >
                    <Building2 className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
                    <span className="min-w-0 truncate font-medium">{d.name}</span>
                    <span className="shrink-0 text-muted-foreground">{d.code}</span>
                  </PickerOption>
                ))}
              </PickerList>
            </>
          ) : (
            <>
              <Label className="text-[11px] font-medium text-muted-foreground">{t("workflow.participant.position")}</Label>
              <PickerList>
                {(positions ?? []).map((p) => (
                  <PickerOption
                    key={p.id}
                    onClick={() => addParticipant({ type: "position_department", department_code: pdDeptCode, position_code: p.code })}
                  >
                    <Briefcase className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
                    <span className="min-w-0 truncate font-medium">{p.name}</span>
                    <span className="shrink-0 text-muted-foreground">{p.code}</span>
                  </PickerOption>
                ))}
              </PickerList>
            </>
          )}
          <Button type="button" variant="ghost" size="xs" className="text-muted-foreground" onClick={() => { setAddingType(""); setPdDeptCode("") }}>
            {t("common:cancel")}
          </Button>
        </div>
      ) : null}
    </div>
  )
}

function flattenDeptTree(nodes: DeptTreeNode[], depth = 0): Array<DeptTreeNode & { depth: number }> {
  const result: Array<DeptTreeNode & { depth: number }> = []
  for (const n of nodes) {
    result.push({ ...n, depth })
    if (n.children) {
      result.push(...flattenDeptTree(n.children, depth + 1))
    }
  }
  return result
}

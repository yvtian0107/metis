import { type ActivityItem, type TicketItem } from "../../../api"

const ACTIVE_STATUSES = new Set(["submitted", "waiting_human", "approved_decisioning", "rejected_decisioning", "decisioning", "executing_action"])
const HUMAN_ACTIVITY_TYPES = new Set(["approve", "form", "process"])
const ACTIVE_ACTIVITY_STATUSES = new Set(["pending", "in_progress"])

export interface TicketActionContextInput {
  ticket: TicketItem | null | undefined
  activities: ActivityItem[]
  currentUserId: number
  canAssignPermission: boolean
  canCancelPermission: boolean
}

export interface TicketActionContext {
  isActive: boolean
  isDecisioning: boolean
  activeHumanActivities: ActivityItem[]
  actionableActivities: ActivityItem[]
  displayHumanActivity: ActivityItem | undefined
  selectedActionableActivity: ActivityItem | undefined
  canProcess: boolean
  canAssign: boolean
  canCancel: boolean
  canWithdraw: boolean
}

function isActiveHumanActivity(activity: ActivityItem) {
  return ACTIVE_ACTIVITY_STATUSES.has(activity.status) && HUMAN_ACTIVITY_TYPES.has(activity.activityType)
}

export function buildTicketActionContext({
  ticket,
  activities,
  currentUserId,
  canAssignPermission,
  canCancelPermission,
}: TicketActionContextInput): TicketActionContext {
  const isActive = ticket ? ACTIVE_STATUSES.has(ticket.status) : false
  const isDecisioning = ticket?.engineType === "smart" && ticket.smartState === "ai_reasoning"
  const activeHumanActivities = activities.filter(isActiveHumanActivity)
  const actionableActivities = activeHumanActivities.filter((activity) => activity.canAct)
  const currentHumanActivity = ticket?.currentActivityId
    ? activeHumanActivities.find((activity) => activity.id === ticket.currentActivityId)
    : undefined
  const currentActionableActivity = ticket?.currentActivityId
    ? actionableActivities.find((activity) => activity.id === ticket.currentActivityId)
    : undefined
  const selectedActionableActivity = currentActionableActivity ?? actionableActivities[0]
  const displayHumanActivity = currentHumanActivity ?? selectedActionableActivity ?? activeHumanActivities[0]

  return {
    isActive,
    isDecisioning,
    activeHumanActivities,
    actionableActivities,
    displayHumanActivity,
    selectedActionableActivity,
    canProcess: Boolean(isActive && !isDecisioning && selectedActionableActivity?.canAct),
    canAssign: Boolean(canAssignPermission && isActive && !isDecisioning),
    canCancel: Boolean(canCancelPermission && isActive && !isDecisioning),
    canWithdraw: Boolean(
      ticket
      && isActive
      && !isDecisioning
      && ticket.status === "submitted"
      && currentUserId > 0
      && ticket.requesterId === currentUserId,
    ),
  }
}

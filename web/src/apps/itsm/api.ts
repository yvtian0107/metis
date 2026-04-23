import { api } from "@/lib/api"

// ─── Catalog ────────────────────────────────────────────

export interface CatalogItem {
  id: number
  parentId: number | null
  name: string
  code: string
  description: string
  icon: string
  sortOrder: number
  isActive: boolean
  children?: CatalogItem[]
  createdAt: string
  updatedAt: string
}

export function fetchCatalogTree() {
  return api.get<CatalogItem[]>("/api/v1/itsm/catalogs/tree").then((r) => r ?? [])
}

export function createCatalog(data: {
  name: string
  code: string
  parentId?: number | null
  description?: string
  icon?: string
  sortOrder?: number
}) {
  return api.post<CatalogItem>("/api/v1/itsm/catalogs", data)
}

export function updateCatalog(
  id: number,
  data: Partial<{
    name: string
    code: string
    parentId: number | null
    description: string
    icon: string
    sortOrder: number
    isActive: boolean
  }>,
) {
  return api.put<CatalogItem>(`/api/v1/itsm/catalogs/${id}`, data)
}

export function deleteCatalog(id: number) {
  return api.delete(`/api/v1/itsm/catalogs/${id}`)
}

// ─── Service Definition ─────────────────────────────────

export interface ServiceDefItem {
  id: number
  name: string
  code: string
  description: string
  catalogId: number
  engineType: string
  slaId: number | null
  intakeFormSchema: unknown
  workflowJson: unknown
  collaborationSpec: string
  agentId: number | null
  agentConfig: SmartAgentConfig | null
  publishHealthCheck: ServiceHealthCheck | null
  isActive: boolean
  sortOrder: number
  createdAt: string
  updatedAt: string
}

export interface SmartAgentConfig {
  confidence_threshold: number
  decision_timeout_seconds: number
  fallback_strategy: string
}

export interface ServiceDefListParams {
  keyword?: string
  catalogId?: number
  isActive?: boolean
  page?: number
  pageSize?: number
}

export function fetchServiceDefs(params: ServiceDefListParams) {
  const p = new URLSearchParams()
  if (params.keyword) p.set("keyword", params.keyword)
  if (params.catalogId) p.set("catalogId", String(params.catalogId))
  if (params.isActive !== undefined) p.set("isActive", String(params.isActive))
  p.set("page", String(params.page ?? 1))
  p.set("pageSize", String(params.pageSize ?? 20))
  return api.get<{ items: ServiceDefItem[]; total: number }>(
    `/api/v1/itsm/services?${p}`,
  )
}

export function fetchServiceDef(id: number) {
  return api.get<ServiceDefItem>(`/api/v1/itsm/services/${id}`)
}

export function createServiceDef(data: {
  name: string
  code: string
  catalogId: number
  engineType?: string
  description?: string
  slaId?: number | null
  intakeFormSchema?: unknown
  sortOrder?: number
}) {
  return api.post<ServiceDefItem>("/api/v1/itsm/services", data)
}

export function updateServiceDef(id: number, data: Partial<ServiceDefItem>) {
  return api.put<ServiceDefItem>(`/api/v1/itsm/services/${id}`, data)
}

export function deleteServiceDef(id: number) {
  return api.delete(`/api/v1/itsm/services/${id}`)
}

// ─── Service Action ─────────────────────────────────────

export interface ServiceActionItem {
  id: number
  serviceId: number
  name: string
  code: string
  actionType: string
  configJson: unknown
  createdAt: string
  updatedAt: string
}

export function fetchServiceActions(serviceId: number) {
  return api.get<ServiceActionItem[]>(
    `/api/v1/itsm/services/${serviceId}/actions`,
  ).then((r) => r ?? [])
}

export function createServiceAction(
  serviceId: number,
  data: { name: string; code: string; actionType: string; configJson?: unknown },
) {
  return api.post<ServiceActionItem>(
    `/api/v1/itsm/services/${serviceId}/actions`,
    data,
  )
}

export function updateServiceAction(
  serviceId: number,
  actionId: number,
  data: Partial<ServiceActionItem>,
) {
  return api.put<ServiceActionItem>(
    `/api/v1/itsm/services/${serviceId}/actions/${actionId}`,
    data,
  )
}

export function deleteServiceAction(serviceId: number, actionId: number) {
  return api.delete(
    `/api/v1/itsm/services/${serviceId}/actions/${actionId}`,
  )
}

// ─── Priority ───────────────────────────────────────────

export interface PriorityItem {
  id: number
  name: string
  code: string
  value: number
  color: string
  description: string
  isActive: boolean
  createdAt: string
  updatedAt: string
}

export function fetchPriorities() {
  return api.get<PriorityItem[]>("/api/v1/itsm/priorities").then((r) => r ?? [])
}

export function createPriority(data: {
  name: string
  code: string
  value: number
  color: string
  description?: string
}) {
  return api.post<PriorityItem>("/api/v1/itsm/priorities", data)
}

export function updatePriority(id: number, data: Partial<PriorityItem>) {
  return api.put<PriorityItem>(`/api/v1/itsm/priorities/${id}`, data)
}

export function deletePriority(id: number) {
  return api.delete(`/api/v1/itsm/priorities/${id}`)
}

// ─── SLA Template ───────────────────────────────────────

export interface SLATemplateItem {
  id: number
  name: string
  code: string
  description: string
  responseMinutes: number
  resolutionMinutes: number
  isActive: boolean
  createdAt: string
  updatedAt: string
}

export function fetchSLATemplates() {
  return api.get<SLATemplateItem[]>("/api/v1/itsm/sla").then((r) => r ?? [])
}

export function createSLATemplate(data: {
  name: string
  code: string
  description?: string
  responseMinutes: number
  resolutionMinutes: number
}) {
  return api.post<SLATemplateItem>("/api/v1/itsm/sla", data)
}

export function updateSLATemplate(id: number, data: Partial<SLATemplateItem>) {
  return api.put<SLATemplateItem>(`/api/v1/itsm/sla/${id}`, data)
}

export function deleteSLATemplate(id: number) {
  return api.delete(`/api/v1/itsm/sla/${id}`)
}

// ─── Escalation Rule ────────────────────────────────────

export interface EscalationRuleItem {
  id: number
  slaId: number
  triggerType: string
  level: number
  waitMinutes: number
  actionType: string
  targetConfig: unknown
  isActive: boolean
  createdAt: string
  updatedAt: string
}

export function fetchEscalationRules(slaId: number) {
  return api.get<EscalationRuleItem[]>(`/api/v1/itsm/sla/${slaId}/escalations`).then((r) => r ?? [])
}

export function createEscalationRule(
  slaId: number,
  data: {
    triggerType: string
    level: number
    waitMinutes: number
    actionType: string
    targetConfig?: unknown
  },
) {
  return api.post<EscalationRuleItem>(
    `/api/v1/itsm/sla/${slaId}/escalations`,
    data,
  )
}

export function updateEscalationRule(
  slaId: number,
  ruleId: number,
  data: Partial<EscalationRuleItem>,
) {
  return api.put<EscalationRuleItem>(
    `/api/v1/itsm/sla/${slaId}/escalations/${ruleId}`,
    data,
  )
}

export function deleteEscalationRule(slaId: number, ruleId: number) {
  return api.delete(`/api/v1/itsm/sla/${slaId}/escalations/${ruleId}`)
}

// ─── Ticket ─────────────────────────────────────────────

export interface TicketItem {
  id: number
  code: string
  title: string
  description: string
  serviceId: number
  serviceName: string
  engineType: string
  status: string
  priorityId: number
  priorityName: string
  priorityColor: string
  requesterId: number
  requesterName: string
  assigneeId: number | null
  assigneeName: string
  currentActivityId: number | null
  source: string
  agentSessionId: number | null
  aiFailureCount: number
  formData: unknown
  workflowJson: unknown
  slaStatus: string
  slaResponseDeadline: string | null
  slaResolutionDeadline: string | null
  finishedAt: string | null
  smartState?: "terminal" | "ai_disabled" | "waiting_ai_confirmation" | "action_running" | "waiting_human" | "ai_reasoning" | "ai_decided" | string
  currentOwnerType?: string
  currentOwnerName?: string
  nextStepSummary?: string
  canAct?: boolean
  canOverride?: boolean
  createdAt: string
  updatedAt: string
}

export interface TicketListParams {
  keyword?: string
  status?: string
  priorityId?: number
  serviceId?: number
  assigneeId?: number
  requesterId?: number
  page?: number
  pageSize?: number
}

export function fetchTickets(params: TicketListParams) {
  const p = new URLSearchParams()
  if (params.keyword) p.set("keyword", params.keyword)
  if (params.status) p.set("status", params.status)
  if (params.priorityId) p.set("priorityId", String(params.priorityId))
  if (params.serviceId) p.set("serviceId", String(params.serviceId))
  if (params.assigneeId) p.set("assigneeId", String(params.assigneeId))
  if (params.requesterId) p.set("requesterId", String(params.requesterId))
  p.set("page", String(params.page ?? 1))
  p.set("pageSize", String(params.pageSize ?? 20))
  return api.get<{ items: TicketItem[]; total: number }>(
    `/api/v1/itsm/tickets?${p}`,
  )
}

export function fetchTicket(id: number) {
  return api.get<TicketItem>(`/api/v1/itsm/tickets/${id}`)
}

export function assignTicket(id: number, assigneeId: number) {
  return api.put<TicketItem>(`/api/v1/itsm/tickets/${id}/assign`, {
    assigneeId,
  })
}

export function cancelTicket(id: number, reason: string) {
  return api.put<TicketItem>(`/api/v1/itsm/tickets/${id}/cancel`, { reason })
}

export function withdrawTicket(id: number, reason: string) {
  return api.put<TicketItem>(`/api/v1/itsm/tickets/${id}/withdraw`, { reason })
}

export function fetchMyTickets(params: {
  keyword?: string
  status?: string
  startDate?: string
  endDate?: string
  page?: number
  pageSize?: number
}) {
  const p = new URLSearchParams()
  if (params.keyword) p.set("keyword", params.keyword)
  if (params.status) p.set("status", params.status)
  if (params.startDate) p.set("startDate", params.startDate)
  if (params.endDate) p.set("endDate", params.endDate)
  p.set("page", String(params.page ?? 1))
  p.set("pageSize", String(params.pageSize ?? 20))
  return api.get<{ items: TicketItem[]; total: number }>(
    `/api/v1/itsm/tickets/mine?${p}`,
  )
}

// ─── Service Desk ──────────────────────────────────────

export interface ServiceDeskState {
  stage: string
  candidate_service_ids?: number[]
  top_match_service_id?: number
  confirmed_service_id?: number
  confirmation_required: boolean
  loaded_service_id?: number
  draft_summary?: string
  draft_form_data?: Record<string, unknown>
  request_text?: string
  prefill_form_data?: Record<string, unknown>
  draft_version: number
  confirmed_draft_version: number
  fields_hash?: string
}

export interface ServiceDeskSessionState {
  state: ServiceDeskState
  nextExpectedAction: string
}

export interface AgenticUISurface<TPayload = unknown> {
  surfaceId: string
  surfaceType: string
  payload: TPayload
}

export interface ITSMDraftFormSurfacePayload {
  status: "loading" | "ready" | "submitted"
  serviceId?: number
  title?: string
  summary?: string
  schema?: unknown
  values?: Record<string, unknown>
  draftVersion?: number
  submitAction?: {
    method?: string
    kind?: string
  }
  ticketId?: number
  ticketCode?: string
  message?: string
}

export type ITSMDraftFormSurface = AgenticUISurface<ITSMDraftFormSurfacePayload>

export interface SubmitDraftRequest {
  draftVersion: number
  summary: string
  formData: Record<string, unknown>
}

export interface SubmitDraftResponse {
  ok: boolean
  ticketId?: number
  ticketCode?: string
  status?: string
  message?: string
  failureReason?: string
  nodeLabel?: string
  guidance?: string
  warnings?: Array<{ type: string; field: string; message: string }>
  missingRequiredFields?: Array<{ key: string; label: string; type: string; required: boolean }>
  state?: ServiceDeskState
  surface?: AgenticUISurface
}

export function fetchServiceDeskSessionState(sessionId: number) {
  return api.get<ServiceDeskSessionState>(
    `/api/v1/itsm/service-desk/sessions/${sessionId}/state`,
  )
}

export function submitServiceDeskDraft(sessionId: number, data: SubmitDraftRequest) {
  return api.post<SubmitDraftResponse>(
    `/api/v1/itsm/service-desk/sessions/${sessionId}/draft/submit`,
    data,
  )
}

// ─── Timeline ───────────────────────────────────────────

export interface TimelineItem {
  id: number
  ticketId: number
  eventType: string
  message: string
  content: string
  operatorId: number
  operatorName: string
  details: unknown
  metadata: unknown
  reasoning: string
  createdAt: string
}

export function fetchTicketTimeline(ticketId: number) {
  return api.get<TimelineItem[]>(`/api/v1/itsm/tickets/${ticketId}/timeline`).then((r) => r ?? [])
}

// ─── Ticket Activities (Classic Engine) ─────────────────

export interface ActivityItem {
  id: number
  ticketId: number
  name: string
  activityType: string
  status: string
  nodeId: string
  executionMode: string
  sequenceOrder: number
  formSchema: unknown
  formData: unknown
  transitionOutcome: string
  aiDecision: string | null
  aiReasoning: string | null
  aiConfidence: number | null
  evidence?: unknown[]
  toolCalls?: unknown[]
  knowledgeHits?: unknown[]
  actionExecutions?: unknown[]
  riskFlags?: unknown[]
  overriddenBy: number | null
  canAct: boolean
  startedAt: string | null
  finishedAt: string | null
  createdAt: string
}

export function fetchTicketActivities(ticketId: number) {
  return api.get<ActivityItem[]>(`/api/v1/itsm/tickets/${ticketId}/activities`).then((r) => r ?? [])
}

export function progressTicket(ticketId: number, data: { activityId: number; outcome: "approved" | "rejected"; opinion: string; result?: unknown }) {
  return api.post<TicketItem>(`/api/v1/itsm/tickets/${ticketId}/progress`, data)
}

export function signalTicket(ticketId: number, data: { activityId: number; outcome: string; data?: unknown }) {
  return api.post<TicketItem>(`/api/v1/itsm/tickets/${ticketId}/signal`, data)
}

// ─── Users (kernel API) ────────────────────────────────

export interface SimpleUser {
  id: number
  username: string
  email: string
  avatar: string
}

export function fetchUsers(keyword?: string) {
  const p = new URLSearchParams({ page: "1", pageSize: "50" })
  if (keyword) p.set("keyword", keyword)
  return api.get<{ items: SimpleUser[] }>(`/api/v1/users?${p}`).then((r) => r.items)
}

// ─── Smart Engine Override APIs ────────────────────────

export function overrideJump(ticketId: number, data: { activityType: string; assigneeId?: number; reason: string }) {
  return api.post(`/api/v1/itsm/tickets/${ticketId}/override/jump`, data)
}

export function overrideReassign(ticketId: number, data: { activityId: number; newAssigneeId: number; reason: string }) {
  return api.post(`/api/v1/itsm/tickets/${ticketId}/override/reassign`, data)
}

export function retryAI(ticketId: number, reason?: string) {
  return api.post(`/api/v1/itsm/tickets/${ticketId}/override/retry-ai`, { reason })
}

// ─── AI App APIs (for smart engine config) ─────────────

export interface AgentItem {
  id: number
  name: string
  description: string
  type: string
  visibility: string
  isActive: boolean
  strategy: string
  temperature: number
  maxTurns: number
  modelId: number
}

export function fetchAgents() {
  return api.get<{ items: AgentItem[] }>("/api/v1/ai/agents?page=1&pageSize=100").then((r) => r?.items ?? [])
}

// ─── Service Knowledge Documents ────────────────────────

export interface KnowledgeDocItem {
  id: number
  serviceId: number
  fileName: string
  fileSize: number
  fileType: string
  parseStatus: string
  parseError?: string
  createdAt: string
}

export function fetchKnowledgeDocs(serviceId: number) {
  return api.get<KnowledgeDocItem[]>(`/api/v1/itsm/services/${serviceId}/knowledge-documents`).then((r) => r ?? [])
}

export function uploadKnowledgeDoc(serviceId: number, file: File) {
  const form = new FormData()
  form.append("file", file)
  return api.upload<KnowledgeDocItem>(`/api/v1/itsm/services/${serviceId}/knowledge-documents`, form)
}

export function deleteKnowledgeDoc(serviceId: number, docId: number) {
  return api.delete(`/api/v1/itsm/services/${serviceId}/knowledge-documents/${docId}`)
}

// ─── Engine Config ──────────────────────────────────────

export interface EngineAgentConfig {
  modelId: number
  providerId: number
  providerName: string
  modelName: string
  temperature: number
}

export interface StaffingAgentSelector {
  agentId: number
  agentName: string
}

export interface StaffingPathBuilderConfig extends EngineAgentConfig {
  maxRetries: number
  timeoutSeconds: number
}

export interface StaffingServiceMatcherConfig extends EngineAgentConfig {
  maxTokens: number
  timeoutSeconds: number
}

export interface EngineHealthItem {
  key: string
  label: string
  status: "pass" | "warn" | "fail"
  message: string
}

export interface SmartStaffingConfig {
  posts: {
    intake: StaffingAgentSelector
    decision: StaffingAgentSelector & { mode: string }
    slaAssurance: StaffingAgentSelector
  }
  health: {
    items: EngineHealthItem[]
  }
}

export interface EngineSettingsConfig {
  runtime: {
    serviceMatcher: StaffingServiceMatcherConfig
    pathBuilder: StaffingPathBuilderConfig
    guard: {
      auditLevel: string
      fallbackAssignee: number
    }
  }
  health: {
    items: EngineHealthItem[]
  }
}

export interface SmartStaffingConfigUpdate {
  posts: {
    intake: { agentId: number }
    decision: { agentId: number; mode: string }
    slaAssurance: { agentId: number }
  }
}

export interface EngineSettingsConfigUpdate {
  runtime: {
    serviceMatcher: { modelId: number; temperature: number; maxTokens: number; timeoutSeconds: number }
    pathBuilder: { modelId: number; temperature: number; maxRetries: number; timeoutSeconds: number }
    guard: { auditLevel: string; fallbackAssignee: number }
  }
}

export function fetchSmartStaffingConfig() {
  return api.get<SmartStaffingConfig>("/api/v1/itsm/smart-staffing/config")
}

export function updateSmartStaffingConfig(data: SmartStaffingConfigUpdate) {
  return api.put("/api/v1/itsm/smart-staffing/config", data)
}

export function fetchEngineSettingsConfig() {
  return api.get<EngineSettingsConfig>("/api/v1/itsm/engine-settings/config")
}

export function updateEngineSettingsConfig(data: EngineSettingsConfigUpdate) {
  return api.put("/api/v1/itsm/engine-settings/config", data)
}

// ─── AI Provider / Model APIs (for smart staffing runtime) ───────

export interface ProviderItem {
  id: number
  name: string
  type: string
  status: string
}

export interface ModelItem {
  id: number
  modelId: string
  displayName: string
  providerId: number
  type: string
  status: string
}

export function fetchProviders() {
  return api.get<{ items: ProviderItem[] }>("/api/v1/ai/providers?pageSize=100").then((r) => r?.items ?? [])
}

export function fetchModels(providerId: number) {
  return api.get<{ items: ModelItem[] }>(`/api/v1/ai/models?providerId=${providerId}&type=llm&pageSize=100`).then((r) => r?.items ?? [])
}

// ─── Workflow Generate ──────────────────────────────────

export interface WorkflowGenerateResponse {
  workflowJson: unknown
  retries: number
  errors?: { nodeId?: string; edgeId?: string; message: string }[]
  service?: ServiceDefItem
  healthCheck?: ServiceHealthCheck
}

export function generateWorkflow(data: { serviceId: number; collaborationSpec: string }) {
  return api.post<WorkflowGenerateResponse>("/api/v1/itsm/workflows/generate", data)
}

// ─── Service Health ─────────────────────────────────────

export interface ServiceHealthItem {
  key: string
  label: string
  status: "pass" | "warn" | "fail"
  message: string
}

export interface ServiceHealthCheck {
  serviceId: number
  status: "pass" | "warn" | "fail"
  items: ServiceHealthItem[]
  checkedAt?: string
}

export function fetchServiceHealth(serviceId: number) {
  return api.get<ServiceHealthCheck>(`/api/v1/itsm/services/${serviceId}/health`)
}

// ─── Process Variables ──────────────────────────────────

export interface ProcessVariableItem {
  id: number
  ticketId: number
  scopeId: string
  key: string
  value: unknown
  valueType: string
  source: string
  createdAt: string
  updatedAt: string
}

export function fetchTicketVariables(ticketId: number) {
  return api.get<ProcessVariableItem[]>(`/api/v1/itsm/tickets/${ticketId}/variables`).then((r) => r ?? [])
}

// --- Execution Tokens ---

export interface TokenItem {
  id: number
  ticketId: number
  parentTokenId: number | null
  nodeId: string
  status: string
  tokenType: string
  scopeId: string
  createdAt: string
  updatedAt: string
}

export function fetchTicketTokens(ticketId: number) {
  return api.get<TokenItem[]>(`/api/v1/itsm/tickets/${ticketId}/tokens`).then((r) => r ?? [])
}

export function updateTicketVariable(ticketId: number, key: string, data: { value: unknown; valueType?: string }) {
  return api.put<ProcessVariableItem>(`/api/v1/itsm/tickets/${ticketId}/variables/${encodeURIComponent(key)}`, data)
}

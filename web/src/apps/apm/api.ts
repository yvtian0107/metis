import { api } from "@/lib/api"

export interface TraceSummary {
  traceId: string
  serviceName: string
  rootOperation: string
  durationMs: number
  statusCode: string
  timestamp: string
  spanCount: number
  hasError: boolean
}

export interface Span {
  traceId: string
  spanId: string
  parentSpanId: string
  serviceName: string
  spanName: string
  spanKind: string
  startTime: string
  duration: number // nanoseconds
  statusCode: string
  statusMessage: string
  spanAttributes: Record<string, string>
  resourceAttributes: Record<string, string>
  events: SpanEvent[]
}

export interface SpanEvent {
  timestamp: string
  name: string
  attributes: Record<string, string>
}

export interface TraceListResponse {
  items: TraceSummary[]
  total: number
  page: number
}

export interface TraceDetailResponse {
  spans: Span[]
}

export interface TraceFilters {
  start: string
  end: string
  service?: string
  operation?: string
  status?: string
  duration_min?: number
  duration_max?: number
  page?: number
  page_size?: number
}

export function fetchTraces(filters: TraceFilters) {
  const params = new URLSearchParams()
  params.set("start", filters.start)
  params.set("end", filters.end)
  if (filters.service) params.set("service", filters.service)
  if (filters.operation) params.set("operation", filters.operation)
  if (filters.status) params.set("status", filters.status)
  if (filters.duration_min != null) params.set("duration_min", String(filters.duration_min))
  if (filters.duration_max != null) params.set("duration_max", String(filters.duration_max))
  params.set("page", String(filters.page ?? 1))
  params.set("page_size", String(filters.page_size ?? 20))
  return api.get<TraceListResponse>(`/api/v1/apm/traces?${params}`)
}

export function fetchTrace(traceId: string) {
  return api.get<TraceDetailResponse>(`/api/v1/apm/traces/${traceId}`)
}

// --- Service Catalog APIs ---

export interface ServiceOverview {
  serviceName: string
  requestCount: number
  avgDurationMs: number
  p50Ms: number
  p95Ms: number
  p99Ms: number
  errorRate: number
  firstSeen: string
  lastSeen: string
}

export interface SparklinePoint {
  timestamp: string
  requestCount: number
  errorRate: number
  avgDurationMs: number
}

export interface ServiceListResponse {
  services: ServiceOverview[]
  sparklines: Record<string, SparklinePoint[]>
}

export interface ServiceDetail {
  serviceName: string
  requestCount: number
  avgDurationMs: number
  p50Ms: number
  p95Ms: number
  p99Ms: number
  errorRate: number
  operations: OperationStats[]
}

export interface OperationStats {
  spanName: string
  requestCount: number
  avgDurationMs: number
  p50Ms: number
  p95Ms: number
  p99Ms: number
  errorRate: number
}

export interface TimeseriesPoint {
  timestamp: string
  requestCount: number
  avgDurationMs: number
  p50Ms: number
  p95Ms: number
  p99Ms: number
  errorRate: number
}

export interface TimeseriesResponse {
  points: TimeseriesPoint[]
  interval: number
}

export function fetchServices(start: string, end: string) {
  const params = new URLSearchParams({ start, end })
  return api.get<ServiceListResponse>(`/api/v1/apm/services?${params}`)
}

export function fetchServiceDetail(name: string, start: string, end: string) {
  const params = new URLSearchParams({ start, end })
  return api.get<ServiceDetail>(`/api/v1/apm/services/${encodeURIComponent(name)}?${params}`)
}

export function fetchTimeseries(params: { start: string; end: string; service?: string; operation?: string; interval?: number }) {
  const sp = new URLSearchParams({ start: params.start, end: params.end })
  if (params.service) sp.set("service", params.service)
  if (params.operation) sp.set("operation", params.operation)
  if (params.interval) sp.set("interval", String(params.interval))
  return api.get<TimeseriesResponse>(`/api/v1/apm/timeseries?${sp}`)
}

// --- Topology APIs ---

export interface TopologyNode {
  serviceName: string
  requestCount: number
  errorRate: number
}

export interface TopologyEdge {
  caller: string
  callee: string
  callCount: number
  avgDurationMs: number
  p95DurationMs: number
  errorRate: number
}

export interface TopologyGraph {
  nodes: TopologyNode[]
  edges: TopologyEdge[]
}

export function fetchTopology(start: string, end: string) {
  const params = new URLSearchParams({ start, end })
  return api.get<TopologyGraph>(`/api/v1/apm/topology?${params}`)
}

// --- Trace Logs APIs ---

export interface TraceLog {
  timestamp: string
  severityText: string
  severityNumber: number
  body: string
  serviceName: string
  traceId: string
  spanId: string
  logAttributes: Record<string, string>
  resourceAttributes: Record<string, string>
}

export interface TraceLogsResponse {
  logs: TraceLog[]
  logsAvailable: boolean
}

export function fetchTraceLogs(traceId: string) {
  return api.get<TraceLogsResponse>(`/api/v1/apm/traces/${traceId}/logs`)
}

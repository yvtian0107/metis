/** Unified API response from Go backend: R{code, message, data} */
interface ApiResponse<T> {
  code: number;
  message: string;
  data: T;
}

export interface PaginatedResponse<T> {
  items: T[];
  total: number;
  page: number;
  pageSize: number;
}

export interface SiteInfo {
  appName: string;
  hasLogo: boolean;
  locale: string;
  timezone: string;
  version: string;
  gitCommit: string;
  buildTime: string;
}

export class ApiError extends Error {
  status: number;
  code: number;

  constructor(message: string, status: number, code: number) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
    this.code = code;
  }
}

// Token refresh state for concurrent 401 handling
let isRefreshing = false;
let refreshQueue: Array<{
  resolve: (token: string) => void;
  reject: (err: Error) => void;
}> = [];

import { TOKEN_KEY, REFRESH_KEY } from './constants';

function getAccessToken(): string | null {
  return localStorage.getItem(TOKEN_KEY);
}

function getRefreshToken(): string | null {
  return localStorage.getItem(REFRESH_KEY);
}

function setTokens(accessToken: string, refreshToken: string) {
  localStorage.setItem(TOKEN_KEY, accessToken);
  localStorage.setItem(REFRESH_KEY, refreshToken);
}

function clearTokens() {
  localStorage.removeItem(TOKEN_KEY);
  localStorage.removeItem(REFRESH_KEY);
}

async function tryRefresh(): Promise<string> {
  const refreshToken = getRefreshToken();
  if (!refreshToken) throw new Error('No refresh token');

  const res = await fetch('/api/v1/auth/refresh', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ refreshToken }),
  });

  if (!res.ok) {
    clearTokens();
    window.location.href = '/login';
    throw new Error('Refresh failed');
  }

  const body = (await res.json()) as ApiResponse<{
    accessToken: string;
    refreshToken: string;
  }>;
  if (body.code !== 0) {
    clearTokens();
    window.location.href = '/login';
    throw new Error('Refresh failed');
  }

  setTokens(body.data.accessToken, body.data.refreshToken);
  return body.data.accessToken;
}

async function handleRefresh(): Promise<string> {
  if (isRefreshing) {
    // Queue this request and wait for the ongoing refresh
    return new Promise((resolve, reject) => {
      refreshQueue.push({ resolve, reject });
    });
  }

  isRefreshing = true;
  try {
    const newToken = await tryRefresh();
    // Resolve all queued requests
    refreshQueue.forEach((q) => q.resolve(newToken));
    refreshQueue = [];
    return newToken;
  } catch (err) {
    refreshQueue.forEach((q) => q.reject(err as Error));
    refreshQueue = [];
    throw err;
  } finally {
    isRefreshing = false;
  }
}

function buildHeaders(init?: RequestInit): Record<string, string> {
  const token = getAccessToken();
  const headers: Record<string, string> = {
    ...(init?.headers as Record<string, string>),
  };

  const hasContentType = Object.keys(headers).some(
    (k) => k.toLowerCase() === 'content-type',
  );
  if (init?.body !== undefined && !hasContentType && !(init.body instanceof FormData)) {
    headers['Content-Type'] = 'application/json';
  }

  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }

  return headers;
}

export async function authorizedFetch(
  url: string,
  init?: RequestInit,
): Promise<Response> {
  const headers = buildHeaders(init);
  let res = await fetch(url, { ...init, headers });

  // Auto-refresh on 401
  if (res.status === 401 && getRefreshToken()) {
    try {
      const newToken = await handleRefresh();
      headers['Authorization'] = `Bearer ${newToken}`;
      res = await fetch(url, { ...init, headers });
    } catch {
      throw new ApiError('Session expired', 401, -1);
    }
  }

  return res;
}

async function handleErrorResponse(res: Response): Promise<never> {
  let message = res.statusText;
  let code = -1;
  try {
    const body = (await res.json()) as ApiResponse<unknown>;
    message = body.message || message;
    code = body.code ?? code;
  } catch {
    // ignore parse errors
  }

  // Password expired — redirect to change password. Other 409 responses are
  // regular domain conflicts and must be surfaced to the caller.
  if (res.status === 409 && message === 'password expired') {
    window.dispatchEvent(
      new CustomEvent('password-expired', { detail: { message } }),
    );
  }
  throw new ApiError(message, res.status, code);
}

async function request<T>(url: string, init?: RequestInit): Promise<T> {
  const res = await authorizedFetch(url, init);

  if (!res.ok) {
    await handleErrorResponse(res);
  }

  const body = (await res.json()) as ApiResponse<T>;
  if (body.code !== 0) {
    throw new ApiError(body.message, res.status, body.code);
  }

  return body.data;
}

async function download(url: string): Promise<Blob> {
  const res = await authorizedFetch(url, { method: 'GET' });

  if (!res.ok) {
    await handleErrorResponse(res);
  }

  return res.blob();
}

export const api = {
  fetch: authorizedFetch,

  get: <T>(url: string) => request<T>(url),

  put: <T>(url: string, data: unknown) =>
    request<T>(url, { method: 'PUT', body: JSON.stringify(data) }),

  post: <T>(url: string, data?: unknown) =>
    request<T>(url, {
      method: 'POST',
      body: data ? JSON.stringify(data) : undefined,
    }),

  delete: <T>(url: string) => request<T>(url, { method: 'DELETE' }),

  patch: <T>(url: string, data?: unknown) =>
    request<T>(url, {
      method: 'PATCH',
      body: data ? JSON.stringify(data) : undefined,
    }),

  upload: <T>(url: string, form: FormData) =>
    request<T>(url, { method: 'POST', body: form }),

  download,
};

// Task types
export interface TaskInfo {
  name: string;
  type: 'scheduled' | 'async';
  description: string;
  cronExpr?: string;
  timeoutMs: number;
  maxRetries: number;
  status: 'active' | 'paused';
  updatedAt: string;
  lastExecution?: {
    timestamp: string;
    status: string;
    duration: number;
  };
}

export interface TaskExecution {
  id: number;
  taskName: string;
  trigger: 'cron' | 'manual' | 'api';
  status: 'pending' | 'running' | 'completed' | 'failed' | 'timeout' | 'stale';
  payload?: string;
  result?: string;
  error?: string;
  retryCount: number;
  startedAt?: string;
  finishedAt?: string;
  createdAt: string;
}

export interface TaskStats {
  totalTasks: number;
  pending: number;
  running: number;
  completedToday: number;
  failedToday: number;
}

export const taskApi = {
  list: (type?: string) =>
    api.get<TaskInfo[]>(`/api/v1/tasks${type ? `?type=${type}` : ''}`),

  get: (name: string) =>
    api.get<{ task: TaskInfo; recentExecutions: TaskExecution[] }>(
      `/api/v1/tasks/${name}`,
    ),

  executions: (name: string, page = 1, pageSize = 20) =>
    api.get<{
      list: TaskExecution[];
      total: number;
      page: number;
      pageSize: number;
    }>(`/api/v1/tasks/${name}/executions?page=${page}&pageSize=${pageSize}`),

  stats: () => api.get<TaskStats>('/api/v1/tasks/stats'),

  pause: (name: string) => api.post<null>(`/api/v1/tasks/${name}/pause`),

  resume: (name: string) => api.post<null>(`/api/v1/tasks/${name}/resume`),

  trigger: (name: string) =>
    api.post<{ executionId: number }>(`/api/v1/tasks/${name}/trigger`),
};

// --- AI Agent types ---

export interface AgentBase {
  id: number;
  name: string;
  description: string;
  avatar: string;
  type: 'assistant' | 'coding';
  isActive: boolean;
  visibility: 'private' | 'team' | 'public';
  createdBy: number;
  temperature: number;
  maxTokens: number;
  maxTurns: number;
  instructions?: string;
  suggestedPrompts?: string[];
  createdAt: string;
  updatedAt: string;
}

export interface AssistantAgentInfo extends AgentBase {
  type: 'assistant';
  strategy?: string;
  modelId?: number;
  systemPrompt?: string;
}

export interface CodingAgentInfo extends AgentBase {
  type: 'coding';
  runtime?: string;
  runtimeConfig?: Record<string, unknown>;
  execMode?: string;
  nodeId?: number;
  workspace?: string;
}

export type AgentInfo = AssistantAgentInfo | CodingAgentInfo;

export type AgentWithBindings = AgentInfo & {
  toolIds: number[];
  skillIds: number[];
  mcpServerIds: number[];
  knowledgeBaseIds: number[];
  knowledgeGraphIds: number[];
  capabilitySetBindings: AgentCapabilitySetBinding[];
};

export interface AgentCapabilitySetBinding {
  setId: number;
  itemIds: number[];
}

interface AgentDetailResponse {
  agent: AgentInfo;
  toolIds?: number[];
  skillIds?: number[];
  mcpServerIds?: number[];
  knowledgeBaseIds?: number[];
  knowledgeGraphIds?: number[];
  capabilitySetBindings?: AgentCapabilitySetBinding[];
}

export interface AgentTemplate {
  id: number;
  name: string;
  description: string;
  icon: string;
  type: string;
  config: Record<string, unknown>;
  createdAt: string;
}

export interface AgentSession {
  id: number;
  agentId: number;
  userId: number;
  status: 'running' | 'completed' | 'cancelled' | 'error';
  title: string;
  pinned: boolean;
  createdAt: string;
  updatedAt: string;
}

export interface SessionMessageMetadata {
  images?: string[];
  tool_name?: string;
  tool_args?: unknown;
  tool_call_id?: string;
  duration_ms?: number;
  status?: 'running' | 'completed' | 'error';
  [key: string]: unknown;
}

export interface SessionMessage {
  id: number;
  sessionId: number;
  role: 'user' | 'assistant' | 'tool_call' | 'tool_result';
  content: string;
  metadata?: SessionMessageMetadata;
  tokenCount: number;
  sequence: number;
  createdAt: string;
}

export interface AgentMemory {
  id: number;
  agentId: number;
  key: string;
  content: string;
  source: 'agent_generated' | 'user_set' | 'system';
  createdAt: string;
  updatedAt: string;
}

function makeTypedAgentApi(basePath: string) {
  return {
    list: (params?: { page?: number; pageSize?: number; keyword?: string }) => {
      const p = new URLSearchParams();
      if (params?.page) p.set('page', String(params.page));
      if (params?.pageSize) p.set('pageSize', String(params.pageSize));
      if (params?.keyword) p.set('keyword', params.keyword);
      return api.get<PaginatedResponse<AgentInfo>>(`${basePath}?${p}`);
    },

    get: async (id: number) => {
      const data = await api.get<AgentWithBindings | AgentDetailResponse>(`${basePath}/${id}`);
      if ('agent' in data) {
        return {
          ...data.agent,
          toolIds: data.toolIds ?? [],
          skillIds: data.skillIds ?? [],
          mcpServerIds: data.mcpServerIds ?? [],
          knowledgeBaseIds: data.knowledgeBaseIds ?? [],
          knowledgeGraphIds: data.knowledgeGraphIds ?? [],
          capabilitySetBindings: data.capabilitySetBindings ?? [],
        } satisfies AgentWithBindings;
      }
      return {
        ...data,
        toolIds: data.toolIds ?? [],
        skillIds: data.skillIds ?? [],
        mcpServerIds: data.mcpServerIds ?? [],
        knowledgeBaseIds: data.knowledgeBaseIds ?? [],
        knowledgeGraphIds: data.knowledgeGraphIds ?? [],
        capabilitySetBindings: data.capabilitySetBindings ?? [],
      };
    },

    create: (data: Partial<AgentInfo> & {
      toolIds?: number[];
      skillIds?: number[];
      mcpServerIds?: number[];
      knowledgeBaseIds?: number[];
      knowledgeGraphIds?: number[];
      capabilitySetBindings?: AgentCapabilitySetBinding[];
      templateId?: number;
    }) => api.post<AgentInfo>(basePath, data),

    update: (id: number, data: Partial<AgentInfo> & {
      toolIds?: number[];
      skillIds?: number[];
      mcpServerIds?: number[];
      knowledgeBaseIds?: number[];
      knowledgeGraphIds?: number[];
      capabilitySetBindings?: AgentCapabilitySetBinding[];
    }) => api.put<AgentInfo>(`${basePath}/${id}`, data),

    delete: (id: number) => api.delete<null>(`${basePath}/${id}`),

    templates: () => api.get<AgentTemplate[]>(`${basePath}/templates`),
  };
}

export const assistantAgentApi = makeTypedAgentApi('/api/v1/ai/assistant-agents');
export const codingAgentApi = makeTypedAgentApi('/api/v1/ai/coding-agents');

export const sessionApi = {
  list: (params?: { page?: number; pageSize?: number; agentId?: number }) => {
    const p = new URLSearchParams();
    if (params?.page) p.set('page', String(params.page));
    if (params?.pageSize) p.set('pageSize', String(params.pageSize));
    if (params?.agentId) p.set('agentId', String(params.agentId));
    return api.get<{ items: AgentSession[]; total: number }>(`/api/v1/ai/sessions?${p}`);
  },

  create: (agentId: number) =>
    api.post<AgentSession>('/api/v1/ai/sessions', { agentId }),

  get: (sid: number) =>
    api.get<{ session: AgentSession; messages: SessionMessage[] }>(`/api/v1/ai/sessions/${sid}`),

  delete: (sid: number) => api.delete<null>(`/api/v1/ai/sessions/${sid}`),

  update: (sid: number, data: { title?: string; pinned?: boolean }) =>
    api.put<AgentSession>(`/api/v1/ai/sessions/${sid}`, data),

  sendMessage: (sid: number, content: string, images?: string[]) =>
    api.post<SessionMessage>(`/api/v1/ai/sessions/${sid}/messages`, { content, images }),

  uploadMessageImage: (sid: number, file: File) => {
    const formData = new FormData();
    formData.append('file', file);
    // 使用 request 直接调用，避免 api.post 的 JSON.stringify
    return request<{ url: string }>(`/api/v1/ai/sessions/${sid}/images`, {
      method: 'POST',
      body: formData,
    });
  },

  cancel: (sid: number) => api.post<null>(`/api/v1/ai/sessions/${sid}/cancel`),

  editMessage: (sid: number, mid: number, content: string) =>
    api.put<SessionMessage>(`/api/v1/ai/sessions/${sid}/messages/${mid}`, { content }),

  continueGeneration: (sid: number) =>
    api.post<null>(`/api/v1/ai/sessions/${sid}/continue`),

  streamUrl: (sid: number) => `/api/v1/ai/sessions/${sid}/stream`,

  chatUrl: (sid: number) => `/api/v1/ai/sessions/${sid}/chat`,
};

export const memoryApi = {
  list: (agentId: number) =>
    api.get<AgentMemory[]>(`/api/v1/ai/agents/${agentId}/memories`),

  create: (agentId: number, data: { key: string; content: string }) =>
    api.post<AgentMemory>(`/api/v1/ai/agents/${agentId}/memories`, data),

  delete: (agentId: number, memoryId: number) =>
    api.delete<null>(`/api/v1/ai/agents/${agentId}/memories/${memoryId}`),
};

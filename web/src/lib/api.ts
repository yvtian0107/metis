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

function getAccessToken(): string | null {
  return localStorage.getItem('metis_access_token');
}

function getRefreshToken(): string | null {
  return localStorage.getItem('metis_refresh_token');
}

function setTokens(accessToken: string, refreshToken: string) {
  localStorage.setItem('metis_access_token', accessToken);
  localStorage.setItem('metis_refresh_token', refreshToken);
}

function clearTokens() {
  localStorage.removeItem('metis_access_token');
  localStorage.removeItem('metis_refresh_token');
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
  if (init?.body !== undefined && !hasContentType) {
    headers['Content-Type'] = 'application/json';
  }

  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }

  return headers;
}

async function authorizedFetch(
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

async function request<T>(url: string, init?: RequestInit): Promise<T> {
  const res = await authorizedFetch(url, init);

  // Password expired — redirect to change password
  if (res.status === 409) {
    let message = '密码已过期，请修改密码';
    try {
      const body = (await res.json()) as ApiResponse<unknown>;
      message = body.message || message;
    } catch {
      // ignore
    }
    window.dispatchEvent(
      new CustomEvent('password-expired', { detail: { message } }),
    );
    throw new ApiError(message, 409, -1);
  }

  if (!res.ok) {
    let message = res.statusText;
    try {
      const body = (await res.json()) as ApiResponse<unknown>;
      message = body.message || message;
    } catch {
      // ignore parse errors
    }
    throw new ApiError(message, res.status, -1);
  }

  const body = (await res.json()) as ApiResponse<T>;
  if (body.code !== 0) {
    throw new ApiError(body.message, res.status, body.code);
  }

  return body.data;
}

async function download(url: string): Promise<Blob> {
  const res = await authorizedFetch(url, { method: 'GET' });

  if (res.status === 409) {
    let message = '密码已过期，请修改密码';
    try {
      const body = (await res.json()) as ApiResponse<unknown>;
      message = body.message || message;
    } catch {
      // ignore
    }
    window.dispatchEvent(
      new CustomEvent('password-expired', { detail: { message } }),
    );
    throw new ApiError(message, 409, -1);
  }

  if (!res.ok) {
    let message = res.statusText;
    try {
      const body = (await res.json()) as ApiResponse<unknown>;
      message = body.message || message;
    } catch {
      // ignore parse errors
    }
    throw new ApiError(message, res.status, -1);
  }

  return res.blob();
}

export const api = {
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

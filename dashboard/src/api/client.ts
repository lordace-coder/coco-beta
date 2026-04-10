import axios from "axios";

const BASE = "/_/api";

export const api = axios.create({ baseURL: BASE });

// Attach token from localStorage on every request
api.interceptors.request.use((config) => {
  const token = localStorage.getItem("admin_token");
  if (token) config.headers.Authorization = `Bearer ${token}`;
  return config;
});

// On 401 → redirect to login (except on the login page itself)
api.interceptors.response.use(
  (r) => r,
  (err) => {
    if (err.response?.status === 401 && !window.location.pathname.includes("/login")) {
      localStorage.removeItem("admin_token");
      window.location.href = "/_/login";
    }
    return Promise.reject(err);
  }
);

// ── Auth ──────────────────────────────────────────────────────────────────────
export const authApi = {
  setupStatus: () => api.get<{ setup_complete: boolean }>("/auth/setup-status"),
  setup: (email: string, password: string) =>
    api.post<{ access_token: string; admin: Admin }>("/auth/setup", { email, password }),
  login: (email: string, password: string) =>
    api.post<{ access_token: string; admin: Admin }>("/auth/login", { email, password }),
  me: () => api.get<Admin>("/auth/me"),
};

// ── Projects ──────────────────────────────────────────────────────────────────
export const projectsApi = {
  list: () => api.get<{ data: Project[]; total: number }>("/projects"),
  get: (id: string) => api.get<Project>(`/projects/${id}`),
  create: (name: string) => api.post<Project>("/projects", { name }),
  update: (id: string, data: Partial<{ name: string; allowed_origins: string[]; active: boolean; configs: Record<string, unknown> }>) =>
    api.patch<Project>(`/projects/${id}`, data),
  delete: (id: string) => api.delete(`/projects/${id}`),
  regenKey: (id: string) => api.post<{ api_key: string }>(`/projects/${id}/regen-key`),
};

// ── Users ─────────────────────────────────────────────────────────────────────
export const usersApi = {
  list: (projectId: string, params?: { limit?: number; offset?: number }) =>
    api.get<PaginatedResponse<AppUser>>(`/projects/${projectId}/users`, { params }),
  create: (projectId: string, data: { email: string; password: string; data?: Record<string, unknown>; roles?: string[] }) =>
    api.post<AppUser>(`/projects/${projectId}/users`, data),
  get: (projectId: string, userId: string) =>
    api.get<AppUser>(`/projects/${projectId}/users/${userId}`),
  update: (projectId: string, userId: string, data: object) =>
    api.patch(`/projects/${projectId}/users/${userId}`, data),
  delete: (projectId: string, userId: string) =>
    api.delete(`/projects/${projectId}/users/${userId}`),
  deleteAll: (projectId: string) => api.delete(`/projects/${projectId}/users`),
};

// ── Collections ───────────────────────────────────────────────────────────────
export const collectionsApi = {
  list: (projectId: string) =>
    api.get<{ data: Collection[]; total: number }>(`/projects/${projectId}/collections`),
  create: (projectId: string, name: string) =>
    api.post<Collection>(`/projects/${projectId}/collections`, { name }),
  get: (projectId: string, colId: string) =>
    api.get<Collection & { document_count: number }>(`/projects/${projectId}/collections/${colId}`),
  update: (projectId: string, colId: string, data: CollectionUpdateRequest) =>
    api.patch<Collection>(`/projects/${projectId}/collections/${colId}`, data),
  delete: (projectId: string, colId: string) =>
    api.delete(`/projects/${projectId}/collections/${colId}`),
  listDocuments: (projectId: string, colId: string, params?: { limit?: number; offset?: number; sort?: string; order?: string }) =>
    api.get<PaginatedResponse<Document>>(`/projects/${projectId}/collections/${colId}/documents`, { params }),
  createDocument: (projectId: string, colId: string, data: Record<string, unknown>) =>
    api.post<Document>(`/projects/${projectId}/collections/${colId}/documents`, { data }),
  getDocument: (projectId: string, colId: string, docId: string) =>
    api.get<Document>(`/projects/${projectId}/collections/${colId}/documents/${docId}`),
  updateDocument: (projectId: string, colId: string, docId: string, data: object, override = false) =>
    api.patch<Document>(`/projects/${projectId}/collections/${colId}/documents/${docId}`, { data, override }),
  deleteDocument: (projectId: string, colId: string, docId: string) =>
    api.delete(`/projects/${projectId}/collections/${colId}/documents/${docId}`),
};

// ── Files ─────────────────────────────────────────────────────────────────────
export const filesApi = {
  list: (projectId: string, prefix?: string) =>
    api.get<{ data: FileEntry[]; total: number }>(`/projects/${projectId}/files`, { params: { prefix } }),
  delete: (projectId: string, key: string) =>
    api.delete(`/projects/${projectId}/files`, { data: { key } }),
};

// ── Config ────────────────────────────────────────────────────────────────────
export const configApi = {
  list: () => api.get<{ data: ConfigEntry[] }>("/config"),
  update: (items: { key: string; value: string }[]) => api.patch("/config", items),
  testSmtp: (to: string) => api.post("/config/smtp/test", { to }),
};

// ── Logs ──────────────────────────────────────────────────────────────────────
export const logsApi = {
  list: (projectId: string, params?: { limit?: number; offset?: number }) =>
    api.get<PaginatedResponse<ActivityLog>>(`/projects/${projectId}/logs`, { params }),
};

// ── Health ────────────────────────────────────────────────────────────────────
export const healthApi = {
  check: () => api.get("/health"),
};

// ── Functions ─────────────────────────────────────────────────────────────────
export const functionsApi = {
  list: (projectId: string) =>
    api.get<{ data: CloudFunction[]; total: number }>(`/projects/${projectId}/functions`),
  get: (projectId: string, fnId: string) =>
    api.get<CloudFunction>(`/projects/${projectId}/functions/${fnId}`),
  create: (projectId: string, data: CloudFunctionCreate) =>
    api.post<CloudFunction>(`/projects/${projectId}/functions`, data),
  update: (projectId: string, fnId: string, data: Partial<CloudFunctionCreate>) =>
    api.patch<CloudFunction>(`/projects/${projectId}/functions/${fnId}`, data),
  delete: (projectId: string, fnId: string) =>
    api.delete(`/projects/${projectId}/functions/${fnId}`),
  run: (projectId: string, fnId: string) =>
    api.post<{ success: boolean; output: string; duration_ms: number; error?: string }>(
      `/projects/${projectId}/functions/${fnId}/run`
    ),
  listRoutes: (projectId: string) =>
    api.get<{ data: HttpRoute[]; total: number }>(`/projects/${projectId}/functions/routes`),
};

// ── Types ─────────────────────────────────────────────────────────────────────
export interface Admin {
  id: string;
  email: string;
  created_at?: string;
}

export interface Project {
  id: string;
  name: string;
  api_key: string;
  active: boolean;
  allowed_origins: string[];
  configs: Record<string, unknown>;
  created_at: string;
}

export interface AppUser {
  id: string;
  email: string;
  data: Record<string, unknown>;
  roles: string[];
  email_verified: boolean;
  oauth_provider?: string;
  created_at: string;
}

export interface CollectionPermissions {
  create: string[];
  read: string[];
  update: string[];
  delete: string[];
}

export interface CollectionWebhooks {
  pre_save?: string;
  post_save?: string;
  pre_delete?: string;
  post_delete?: string;
}

export interface CollectionSentinels {
  list?: string;
  view?: string;
  create?: string;
  update?: string;
  delete?: string;
}

export interface CollectionUpdateRequest {
  name?: string;
  permissions?: CollectionPermissions;
  webhooks?: CollectionWebhooks;
  sentinels?: CollectionSentinels;
}

export interface Collection {
  id: string;
  name: string;
  project_id: string;
  created_at: string;
  document_count?: number;
  permissions?: CollectionPermissions;
  webhooks?: CollectionWebhooks;
  sentinels?: CollectionSentinels;
}

export interface Document {
  id: string;
  collection_id: string;
  data: Record<string, unknown>;
  created_at: string;
}

export interface FileEntry {
  key: string;
  size: number;
  last_modified: string;
  url: string;
}

export interface ConfigEntry {
  key: string;
  value: string;
  is_secret: boolean;
}

export interface ActivityLog {
  id: string;
  project_id: string;
  action: string;
  resource: string;
  resource_id: string;
  detail: string;
  created_at: string;
}

export type TriggerType = "http" | "hook" | "cron";

export interface TriggerConfig {
  // HTTP
  method?: string;
  path?: string;
  // Hook
  event?: string;
  collection?: string;
  // Cron
  schedule?: string;
}

export interface FunctionLog {
  run_at: string;
  duration_ms: number;
  success: boolean;
  output: string;
  error?: string;
}

export interface CloudFunction {
  id: string;
  project_id: string;
  name: string;
  code: string;
  trigger_type: TriggerType;
  trigger_config: TriggerConfig;
  enabled: boolean;
  timeout: number;
  logs: FunctionLog[];
  last_run_at?: string;
  last_error?: string;
  next_run?: string;
  prev_run?: string;
  created_at: string;
  updated_at: string;
}

export interface CloudFunctionCreate {
  name: string;
  code: string;
  trigger_type: TriggerType;
  trigger_config: TriggerConfig;
  enabled?: boolean;
  timeout?: number;
}

export interface HttpRoute {
  function_id: string;
  name: string;
  method: string;
  path: string;
  enabled: boolean;
}

export interface PaginatedResponse<T> {
  data: T[];
  total: number;
  limit: number;
  offset: number;
  has_more: boolean;
}

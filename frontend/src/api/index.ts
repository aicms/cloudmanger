import client from './client'
import type {
  CloudAccount,
  DashboardOverview,
  DNSRecord,
  Domain,
  LoginResponse,
  OperationLog,
  PageResult,
  Server,
  User,
} from '../types'

// ===== 认证 =====
export const authApi = {
  login: (username: string, password: string) =>
    client.post<unknown, LoginResponse>('/auth/login', { username, password }),
  profile: () => client.get<unknown, User>('/auth/profile'),
  changePassword: (old_password: string, new_password: string) =>
    client.put('/auth/password', { old_password, new_password }),
}

// ===== 仪表盘 =====
export const dashboardApi = {
  overview: () => client.get<unknown, DashboardOverview>('/dashboard/overview'),
}

// ===== 云账号 =====
export interface CloudAccountForm {
  name: string
  provider: string
  access_key_id: string
  access_key_secret: string
  region: string
  remark: string
}

export interface CloudAccountImportResult {
  total: number
  success: number
  failed: number
  results: {
    row: number
    provider: string
    name: string
    ok: boolean
    error?: string
  }[]
}

export interface CloudAccountBatchSyncResult {
  total: number
  success: number
  failed: number
  results: {
    id: number
    name: string
    ok: boolean
    error?: string
    servers: number
    domains: number
    records: number
  }[]
}

export const cloudAccountApi = {
  list: () => client.get<unknown, CloudAccount[]>('/cloud-accounts'),
  create: (data: CloudAccountForm) => client.post('/cloud-accounts', data),
  import: (file: File) => {
    const formData = new FormData()
    formData.append('file', file)
    return client.post<unknown, CloudAccountImportResult>('/cloud-accounts/import', formData)
  },
  update: (id: number, data: CloudAccountForm) => client.put(`/cloud-accounts/${id}`, data),
  remove: (id: number) => client.delete(`/cloud-accounts/${id}`),
  toggle: (id: number, enabled: boolean) =>
    client.put(`/cloud-accounts/${id}/toggle`, { enabled }),
  sync: (id: number) => client.post(`/cloud-accounts/${id}/sync`),
  batchSync: (ids?: number[]) =>
    client.post<unknown, CloudAccountBatchSyncResult>('/cloud-accounts/batch-sync', { ids: ids || [] }),
}

// ===== 云服务器 =====
export interface ServerQuery {
  page?: number
  page_size?: number
  provider?: string
  region?: string
  status?: string
  keyword?: string
  tag?: string
}

export interface ServerMeta {
  regions: string[]
  tags: string[]
}

export interface ServerUpdateForm {
  custom_name: string
  tags: string
}

export interface ServerBatchAction {
  action: string
  provider?: string
  region?: string
  status?: string
  keyword?: string
  tag?: string
}

export const serverApi = {
  list: (params: ServerQuery) =>
    client.get<unknown, PageResult<Server>>('/servers', { params }),
  meta: () => client.get<unknown, ServerMeta>('/servers/meta'),
  detail: (id: number) => client.get<unknown, Server>(`/servers/${id}`),
  update: (id: number, data: ServerUpdateForm) =>
    client.put(`/servers/${id}`, data),
  action: (id: number, action: string) =>
    client.post(`/servers/${id}/action`, { action }),
  batchAction: (data: ServerBatchAction) =>
    client.post<unknown, { success: boolean; count: number }>('/servers/batch-action', data),
}

// ===== 域名 =====
export interface DomainQuery {
  page?: number
  page_size?: number
  provider?: string
  keyword?: string
}

export const domainApi = {
  list: (params: DomainQuery) =>
    client.get<unknown, PageResult<Domain>>('/domains', { params }),
  detail: (id: number) => client.get<unknown, Domain>(`/domains/${id}`),
}

// ===== DNS 解析 =====
export interface DNSQuery {
  page?: number
  page_size?: number
  domain?: string
  domain_id?: number
  type?: string
  provider?: string
  keyword?: string
}

export interface DNSRecordForm {
  domain_id: number
  type: string
  host: string
  value: string
  ttl?: number
  priority?: number
}

export const dnsApi = {
  list: (params: DNSQuery) =>
    client.get<unknown, PageResult<DNSRecord>>('/dns-records', { params }),
  create: (data: DNSRecordForm) => client.post('/dns-records', data),
  update: (id: number, data: Partial<DNSRecordForm> & { status?: string }) =>
    client.put(`/dns-records/${id}`, data),
  remove: (id: number) => client.delete(`/dns-records/${id}`),
}

// ===== 操作日志 =====
export const logApi = {
  list: (params: { page?: number; page_size?: number; action?: string; keyword?: string }) =>
    client.get<unknown, PageResult<OperationLog>>('/operation-logs', { params }),
}

// ===== 设置 =====
export interface SettingsData {
  system_name: string
  platform_name: string
  version: string
  sync_interval: number
  sync_concurrency: number
  sync_concurrency_min: number
  sync_concurrency_max: number
  ssh_user: string
  ssh_port: number
  session_timeout: number
  theme: 'light' | 'dark'
}

export const settingsApi = {
  get: () => client.get<unknown, SettingsData>('/settings'),
  update: (data: Partial<SettingsData>) => client.put('/settings', data),
}

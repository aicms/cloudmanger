// 统一 API 响应
export interface ApiResponse<T = any> {
  code: number
  message: string
  data: T
}

// 分页结果
export interface PageResult<T> {
  list: T[]
  total: number
  page: number
  page_size: number
}

export interface User {
  id: number
  username: string
  display_name: string
  email: string
  role: string
  status: string
}

export interface LoginResponse {
  token: string
  expires_in: number
  user: User
}

export interface CloudAccount {
  id: number
  name: string
  provider: string
  access_key_id: string
  access_key_masked?: string
  region: string
  remark: string
  enabled: boolean
  last_synced_at: string | null
  created_at: string
}

export interface Server {
  id: number
  cloud_account_id: number
  provider: string
  category?: string
  instance_id: string
  name: string
  custom_name?: string
  status: string
  region: string
  zone: string
  cpu: number
  memory: number
  os: string
  instance_type: string
  public_ip: string
  inner_ip: string
  bandwidth: number
  tags: string
  expiry_date?: string
  synced_at: string
}

export interface Domain {
  id: number
  cloud_account_id: number
  provider: string
  domain_name: string
  registrar: string
  registration_date: string
  expiry_date: string
  dns_servers: string
  status: string
  synced_at: string
}

export interface DNSRecord {
  id: number
  domain_id: number
  domain_name: string
  cloud_account_id: number
  provider: string
  record_id: string
  type: string
  host: string
  value: string
  ttl: number
  priority: number
  status: string
  synced_at: string
}

export interface OperationLog {
  id: number
  user_id: number
  username: string
  action: string
  target: string
  detail: string
  ip: string
  status: string
  created_at: string
}

export interface DashboardOverview {
  server_total: number
  server_running: number
  server_stopped: number
  domain_total: number
  domain_expiring: number
  domain_expired: number
  provider_dist: Record<string, number>
  status_dist: Record<string, number>
  region_dist: Record<string, number>
  expiring_domains: Domain[]
  recent_logs: OperationLog[]
}

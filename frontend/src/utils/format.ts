import dayjs from 'dayjs'

// 格式化内存：MB -> GB
export function formatMemory(mb: number): string {
  if (!mb) return '-'
  if (mb >= 1024) return `${(mb / 1024).toFixed(0)} GB`
  return `${mb} MB`
}

// 角色中文名
export function roleName(role?: string): string {
  switch (role) {
    case 'admin':
      return '管理员'
    case 'viewer':
      return '只读'
    default:
      return role || '-'
  }
}

// 云厂商中文名
export function providerName(p: string): string {
  switch (p) {
    case 'aliyun':
      return '阿里云'
    case 'tencent':
      return '腾讯云'
    default:
      return p || '-'
  }
}

// 云厂商主题色
export function providerColor(p: string): string {
  switch (p) {
    case 'aliyun':
      return '#FF6A00'
    case 'tencent':
      return '#006EFF'
    default:
      return '#8c8c8c'
  }
}

// 是否轻量应用服务器（腾讯云 Lighthouse / 阿里云 轻量应用服务器）
export function isLightweight(category?: string): boolean {
  return category === 'lighthouse' || category === 'swas'
}

// 服务器状态 -> Tag 颜色
export function serverStatusColor(status: string): string {
  switch (status) {
    case 'Running':
      return 'green'
    case 'Stopped':
      return 'red'
    case 'Starting':
    case 'Stopping':
      return 'orange'
    default:
      return 'default'
  }
}

export function serverStatusText(status: string): string {
  switch (status) {
    case 'Running':
      return '运行中'
    case 'Stopped':
      return '已停止'
    case 'Starting':
      return '启动中'
    case 'Stopping':
      return '停止中'
    default:
      return status || '-'
  }
}

// 域名状态
export function domainStatusText(status: string): string {
  switch (status) {
    case 'ok':
      return '正常'
    case 'expiring':
      return '即将到期'
    case 'expired':
      return '已过期'
    default:
      return status || '-'
  }
}

export function domainStatusColor(status: string): string {
  switch (status) {
    case 'ok':
      return 'green'
    case 'expiring':
      return 'orange'
    case 'expired':
      return 'red'
    default:
      return 'default'
  }
}

// 距离到期天数
export function daysUntil(dateStr: string): number {
  if (!dateStr) return 0
  return dayjs(dateStr).startOf('day').diff(dayjs().startOf('day'), 'day')
}

// 格式化时间
export function formatTime(t?: string | null): string {
  if (!t) return '-'
  const d = dayjs(t)
  return d.isValid() ? d.format('YYYY-MM-DD HH:mm') : t
}

// 地域代码 -> 中文名
const REGION_NAMES: Record<string, string> = {
  'cn-hangzhou': '杭州',
  'cn-shanghai': '上海',
  'cn-beijing': '北京',
  'cn-shenzhen': '深圳',
  'cn-zhangjiakou': '张家口',
  'cn-huhehaote': '呼和浩特',
  'cn-chengdu': '成都',
  'cn-guangzhou': '广州',
  'ap-guangzhou': '广州',
  'ap-shanghai': '上海',
  'ap-beijing': '北京',
  'ap-chengdu': '成都',
  'ap-nanjing': '南京',
  'ap-hongkong': '香港',
  'ap-singapore': '新加坡',
  'ap-tokyo': '东京',
  'us-west-1': '美西',
  'us-east-1': '美东',
}

export function regionName(code: string): string {
  if (!code) return '-'
  return REGION_NAMES[code] ? `${REGION_NAMES[code]} (${code})` : code
}

// 操作日志动作 -> 中文名
export const ACTION_LABELS: Record<string, string> = {
  login: '登录',
  'password.change': '修改密码',
  'cloud_account.create': '新增云账号',
  'cloud_account.update': '编辑云账号',
  'cloud_account.delete': '删除云账号',
  'cloud_account.sync': '同步云账号',
  'server.start': '服务器开机',
  'server.stop': '服务器关机',
  'server.reboot': '服务器重启',
  'dns.create': '新增解析',
  'dns.update': '修改解析',
  'dns.delete': '删除解析',
}

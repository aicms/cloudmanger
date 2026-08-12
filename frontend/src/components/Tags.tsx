import { Tag } from 'antd'
import {
  providerColor,
  providerName,
  serverStatusColor,
  serverStatusText,
  domainStatusColor,
  domainStatusText,
} from '../utils/format'

// 云厂商标签
export function ProviderTag({ provider }: { provider: string }) {
  return (
    <Tag color={providerColor(provider)} style={{ marginInlineEnd: 0 }}>
      {providerName(provider)}
    </Tag>
  )
}

// 服务器状态标签
export function ServerStatusTag({ status }: { status: string }) {
  return <Tag color={serverStatusColor(status)}>{serverStatusText(status)}</Tag>
}

// 域名状态标签
export function DomainStatusTag({ status }: { status: string }) {
  return <Tag color={domainStatusColor(status)}>{domainStatusText(status)}</Tag>
}

// DNS 记录类型标签颜色
export function RecordTypeTag({ type }: { type: string }) {
  const colorMap: Record<string, string> = {
    A: 'blue',
    AAAA: 'geekblue',
    CNAME: 'purple',
    MX: 'cyan',
    TXT: 'gold',
    NS: 'magenta',
    SRV: 'volcano',
  }
  return <Tag color={colorMap[type] || 'default'}>{type}</Tag>
}

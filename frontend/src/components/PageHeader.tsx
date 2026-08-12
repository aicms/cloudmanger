import type { ReactNode } from 'react'
import { theme } from 'antd'

interface PageHeaderProps {
  title: string
  description?: string
  extra?: ReactNode
}

// 页面标题区：左侧标题 + 说明，右侧操作区，统一各页面对齐与间距
export default function PageHeader({ title, description, extra }: PageHeaderProps) {
  const { token } = theme.useToken()
  return (
    <div
      style={{
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between',
        flexWrap: 'wrap',
        gap: 12,
        marginBottom: 16,
      }}
    >
      <div>
        <div style={{ display: 'flex', alignItems: 'baseline', gap: 10 }}>
          <span style={{ fontSize: 20, fontWeight: 600, lineHeight: 1.4, color: token.colorText }}>
            {title}
          </span>
        </div>
        {description && (
          <div style={{ fontSize: 13, color: token.colorTextSecondary, marginTop: 2 }}>{description}</div>
        )}
      </div>
      {extra && <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>{extra}</div>}
    </div>
  )
}

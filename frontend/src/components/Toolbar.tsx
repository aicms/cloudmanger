import type { ReactNode } from 'react'
import { Space } from 'antd'

interface ToolbarProps {
  left?: ReactNode
  right?: ReactNode
}

// 表格上方工具栏：左侧筛选条件，右侧操作按钮，两端对齐、自动换行
export default function Toolbar({ left, right }: ToolbarProps) {
  return (
    <div
      style={{
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between',
        flexWrap: 'wrap',
        rowGap: 12,
        columnGap: 12,
        marginBottom: 16,
      }}
    >
      <Space size={12} wrap>
        {left}
      </Space>
      {right && (
        <Space size={8} wrap>
          {right}
        </Space>
      )}
    </div>
  )
}

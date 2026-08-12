import { useEffect, useState, useCallback } from 'react'
import { Table, Input, Button, Tag, Card, Select, Empty } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { ReloadOutlined, SearchOutlined, UndoOutlined } from '@ant-design/icons'
import { logApi } from '../api'
import type { OperationLog } from '../types'
import { formatTime, ACTION_LABELS } from '../utils/format'
import PageHeader from '../components/PageHeader'
import Toolbar from '../components/Toolbar'

const ACTION_OPTIONS = Object.entries(ACTION_LABELS).map(([value, label]) => ({
  value,
  label,
}))

export default function Logs() {
  const [data, setData] = useState<OperationLog[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(false)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(10)
  const [keyword, setKeyword] = useState('')
  const [action, setAction] = useState('')

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const res = await logApi.list({
        page,
        page_size: pageSize,
        keyword,
        action: action || undefined,
      })
      setData(res.list || [])
      setTotal(res.total)
    } finally {
      setLoading(false)
    }
  }, [page, pageSize, keyword, action])

  useEffect(() => {
    load()
  }, [load])

  const hasFilters = !!keyword || !!action
  const resetFilters = () => {
    setKeyword('')
    setAction('')
    setPage(1)
  }

  const columns: ColumnsType<OperationLog> = [
    {
      title: '时间',
      dataIndex: 'created_at',
      width: 160,
      render: (v: string) => <span className="mono-num">{formatTime(v)}</span>,
    },
    { title: '用户', dataIndex: 'username', width: 100 },
    {
      title: '操作',
      dataIndex: 'action',
      width: 140,
      render: (v: string) => <Tag>{ACTION_LABELS[v] || v}</Tag>,
    },
    { title: '目标', dataIndex: 'target', ellipsis: true },
    { title: '详情', dataIndex: 'detail', ellipsis: true },
    { title: 'IP', dataIndex: 'ip', width: 130, render: (v: string) => <span className="mono-num">{v}</span> },
    {
      title: '结果',
      dataIndex: 'status',
      width: 90,
      render: (v: string) =>
        v === 'success' ? <Tag color="green">成功</Tag> : <Tag color="red">失败</Tag>,
    },
  ]

  return (
    <div>
      <PageHeader title="操作日志" description="记录系统内的关键操作与执行结果" />

      <Card styles={{ body: { padding: 20 } }}>
        <Toolbar
          left={
            <>
              <Select
                placeholder="操作类型"
                allowClear
                showSearch
                optionFilterProp="label"
                style={{ width: 160 }}
                value={action || undefined}
                onChange={(v) => {
                  setAction(v || '')
                  setPage(1)
                }}
                options={ACTION_OPTIONS}
              />
              <Input
                placeholder="搜索用户/目标/详情"
                allowClear
                prefix={<SearchOutlined />}
                style={{ width: 280 }}
                value={keyword}
                onChange={(e) => setKeyword(e.target.value)}
                onPressEnter={() => {
                  setPage(1)
                  load()
                }}
              />
              {hasFilters && (
                <Button type="link" icon={<UndoOutlined />} onClick={resetFilters}>
                  重置
                </Button>
              )}
            </>
          }
          right={
            <Button icon={<ReloadOutlined />} onClick={load} loading={loading}>
              刷新
            </Button>
          }
        />

        <Table
          rowKey="id"
          loading={loading}
          columns={columns}
          dataSource={data}
          size="middle"
          scroll={{ x: 900 }}
          locale={{ emptyText: <Empty description="暂无操作日志" /> }}
          pagination={{
            current: page,
            pageSize,
            total,
            showSizeChanger: true,
            showTotal: (t) => `共 ${t} 条`,
            onChange: (p, ps) => {
              setPage(p)
              setPageSize(ps)
            },
          }}
        />
      </Card>
    </div>
  )
}

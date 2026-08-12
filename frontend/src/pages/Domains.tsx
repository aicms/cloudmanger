import { useEffect, useState, useCallback } from 'react'
import { Table, Input, Select, Button, Tag, Drawer, Descriptions, Card, Space } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { ReloadOutlined, SearchOutlined, UndoOutlined } from '@ant-design/icons'
import { useNavigate } from 'react-router-dom'
import { domainApi } from '../api'
import type { Domain } from '../types'
import { ProviderTag, DomainStatusTag } from '../components/Tags'
import { daysUntil, formatTime, providerName } from '../utils/format'
import PageHeader from '../components/PageHeader'
import Toolbar from '../components/Toolbar'

const EMPTY_FILTERS = { provider: '', keyword: '', status: '' }

export default function Domains() {
  const navigate = useNavigate()
  const [data, setData] = useState<Domain[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(false)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(10)
  const [filters, setFilters] = useState(EMPTY_FILTERS)
  const [detail, setDetail] = useState<Domain | null>(null)

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const res = await domainApi.list({ page, page_size: pageSize, ...filters })
      setData(res.list || [])
      setTotal(res.total)
    } finally {
      setLoading(false)
    }
  }, [page, pageSize, filters])

  useEffect(() => {
    load()
  }, [load])

  const columns: ColumnsType<Domain> = [
    {
      title: '域名',
      dataIndex: 'domain_name',
      render: (v: string, r) => (
        <a onClick={() => setDetail(r)}>
          <strong>{v}</strong>
        </a>
      ),
    },
    {
      title: '云厂商',
      dataIndex: 'provider',
      width: 100,
      render: (v: string) => <ProviderTag provider={v} />,
    },
    { title: '注册商', dataIndex: 'registrar', width: 110 },
    { title: '注册日期', dataIndex: 'registration_date', width: 120 },
    {
      title: '到期日期',
      dataIndex: 'expiry_date',
      width: 150,
      render: (d: string) => {
        const days = daysUntil(d)
        const color = days < 0 ? 'red' : days <= 7 ? 'red' : days <= 30 ? 'orange' : undefined
        return (
          <span>
            <Tag color={color}>{d}</Tag>
            <span style={{ fontSize: 12, color: '#8c8c8c', marginLeft: 4 }}>
              {days > 0 ? `剩 ${days} 天` : days < 0 ? `已过期 ${-days} 天` : '今日到期'}
            </span>
          </span>
        )
      },
    },
    {
      title: '状态',
      dataIndex: 'status',
      width: 100,
      render: (s: string) => <DomainStatusTag status={s} />,
    },
    {
      title: '操作',
      key: 'action',
      width: 120,
      render: (_, r) => (
        <Button type="link" size="small" onClick={() => navigate(`/dns-records?domain=${encodeURIComponent(r.domain_name)}`)}>
          解析记录
        </Button>
      ),
    },
  ]

  const hasFilters = Object.values(filters).some(Boolean)
  const resetFilters = () => {
    setFilters(EMPTY_FILTERS)
    setPage(1)
  }

  return (
    <div>
      <PageHeader title="域名管理" description="跨云域名统一视图，到期自动提醒" />

      <Card styles={{ body: { padding: 20 } }}>
        <Toolbar
          left={
            <>
              <Select
                placeholder="云厂商"
                allowClear
                style={{ width: 130 }}
                value={filters.provider || undefined}
                onChange={(v) => {
                  setFilters((f) => ({ ...f, provider: v || '' }))
                  setPage(1)
                }}
                options={[
                  { value: 'aliyun', label: '阿里云' },
                  { value: 'tencent', label: '腾讯云' },
                ]}
              />
              <Select
                placeholder="状态"
                allowClear
                style={{ width: 130 }}
                value={filters.status || undefined}
                onChange={(v) => {
                  setFilters((f) => ({ ...f, status: v || '' }))
                  setPage(1)
                }}
                options={[
                  { value: 'ok', label: '正常' },
                  { value: 'expiring', label: '即将到期' },
                  { value: 'expired', label: '已过期' },
                ]}
              />
              <Input
                placeholder="搜索域名"
                allowClear
                prefix={<SearchOutlined />}
                style={{ width: 220 }}
                value={filters.keyword}
                onChange={(e) => setFilters((f) => ({ ...f, keyword: e.target.value }))}
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
          pagination={{
            current: page,
            pageSize,
            total,
            showSizeChanger: true,
            showTotal: (t) => `共 ${t} 个域名`,
            onChange: (p, ps) => {
              setPage(p)
              setPageSize(ps)
            },
          }}
        />
      </Card>

      <Drawer
        title={detail?.domain_name}
        open={!!detail}
        onClose={() => setDetail(null)}
        width={480}
        extra={
          <Button type="link" onClick={() => navigate(`/dns-records?domain=${encodeURIComponent(detail!.domain_name)}`)}>
            管理解析记录
          </Button>
        }
      >
        {detail && (
          <Descriptions column={1} bordered size="small">
            <Descriptions.Item label="域名">{detail.domain_name}</Descriptions.Item>
            <Descriptions.Item label="云厂商">{providerName(detail.provider)}</Descriptions.Item>
            <Descriptions.Item label="注册商">{detail.registrar}</Descriptions.Item>
            <Descriptions.Item label="注册日期">{detail.registration_date}</Descriptions.Item>
            <Descriptions.Item label="到期日期">
              <Space>
                <span>{detail.expiry_date}</span>
                <DomainStatusTag status={detail.status} />
              </Space>
            </Descriptions.Item>
            <Descriptions.Item label="DNS 服务器">{detail.dns_servers || '-'}</Descriptions.Item>
            <Descriptions.Item label="同步时间">{formatTime(detail.synced_at)}</Descriptions.Item>
          </Descriptions>
        )}
      </Drawer>
    </div>
  )
}

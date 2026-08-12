import { useEffect, useState, useCallback } from 'react'
import { useSearchParams } from 'react-router-dom'
import {
  Table,
  Input,
  Select,
  Space,
  Button,
  Popconfirm,
  App,
  Modal,
  Form,
  InputNumber,
  Tag,
  Card,
  Switch,
  Empty,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { PlusOutlined, ReloadOutlined, SearchOutlined } from '@ant-design/icons'
import { dnsApi, domainApi } from '../api'
import type { DNSRecord, Domain } from '../types'
import { ProviderTag, RecordTypeTag } from '../components/Tags'
import { formatTime, providerName } from '../utils/format'
import PageHeader from '../components/PageHeader'
import Toolbar from '../components/Toolbar'

const RECORD_TYPES = ['A', 'AAAA', 'CNAME', 'MX', 'TXT', 'NS', 'SRV']

// 按记录类型校验记录值格式
function validateRecordValue(type: string, value: string): string | null {
  const v = (value || '').trim()
  if (!v) return '请输入记录值'
  switch (type) {
    case 'A':
      return /^(\d{1,3}\.){3}\d{1,3}$/.test(v) ? null : 'A 记录应为 IPv4 地址，如 192.168.1.1'
    case 'AAAA':
      return /^([0-9a-fA-F]{1,4}:){7}[0-9a-fA-F]{1,4}$/.test(v) ? null : 'AAAA 记录应为 IPv6 地址'
    case 'CNAME':
    case 'NS':
      return /\./.test(v) ? null : `${type} 记录应指向一个域名，如 example.com`
    case 'MX':
      return /\./.test(v) ? null : 'MX 记录应指向邮件服务器域名，如 mail.example.com'
    case 'TXT':
      return v.length <= 512 ? null : 'TXT 记录值过长（≤512 字符）'
    default:
      return null
  }
}

export default function DNSRecords() {
  const [searchParams] = useSearchParams()
  const [data, setData] = useState<DNSRecord[]>([])
  const [domains, setDomains] = useState<Domain[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(false)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(10)
  const [currentDomain, setCurrentDomain] = useState<string>('')
  const [typeFilter, setTypeFilter] = useState<string>('')
  const [keyword, setKeyword] = useState('')
  const [modalOpen, setModalOpen] = useState(false)
  const [editing, setEditing] = useState<DNSRecord | null>(null)
  const [togglingId, setTogglingId] = useState<number | null>(null)
  const [form] = Form.useForm()
  const { message } = App.useApp()

  // 加载域名列表供选择；支持从域名管理页带入指定域名（业务链路）
  useEffect(() => {
    domainApi.list({ page: 1, page_size: 100 }).then((res) => {
      const list = res.list || []
      setDomains(list)
      const fromQuery = searchParams.get('domain')
      if (fromQuery && list.some((d) => d.domain_name === fromQuery)) {
        setCurrentDomain(fromQuery)
      } else if (list.length > 0 && !currentDomain) {
        setCurrentDomain(list[0].domain_name)
      }
    })
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  // 从域名管理页带域名进入时，即使本页已挂载也切换当前域名（业务链路）
  useEffect(() => {
    const fromQuery = searchParams.get('domain')
    if (fromQuery && domains.some((d) => d.domain_name === fromQuery)) {
      setCurrentDomain(fromQuery)
      setPage(1)
    }
  }, [searchParams, domains])

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const res = await dnsApi.list({
        page,
        page_size: pageSize,
        domain: currentDomain,
        type: typeFilter,
        keyword,
      })
      setData(res.list || [])
      setTotal(res.total)
    } finally {
      setLoading(false)
    }
  }, [page, pageSize, currentDomain, typeFilter, keyword])

  useEffect(() => {
    if (currentDomain) load()
  }, [load, currentDomain])

  const openCreate = () => {
    setEditing(null)
    form.resetFields()
    form.setFieldsValue({ type: 'A', ttl: 600, priority: 0, host: '@' })
    setModalOpen(true)
  }

  const openEdit = (record: DNSRecord) => {
    setEditing(record)
    form.setFieldsValue({
      type: record.type,
      host: record.host,
      value: record.value,
      ttl: record.ttl,
      priority: record.priority,
    })
    setModalOpen(true)
  }

  const handleSubmit = async () => {
    const values = await form.validateFields()
    const valueErr = validateRecordValue(values.type, values.value)
    if (valueErr) {
      message.error(valueErr)
      return
    }
    const domain = domains.find((d) => d.domain_name === currentDomain)
    if (!domain) {
      message.error('请先选择域名')
      return
    }
    if (editing) {
      await dnsApi.update(editing.id, values)
      message.success('解析记录已更新')
    } else {
      await dnsApi.create({ domain_id: domain.id, ...values })
      message.success('解析记录已添加')
    }
    setModalOpen(false)
    load()
  }

  const handleDelete = async (id: number, host: string) => {
    await dnsApi.remove(id)
    message.success(`解析记录「${host}」已删除`)
    load()
  }

  // 启用 / 暂停切换
  const toggleStatus = async (record: DNSRecord) => {
    const next = record.status === 'enable' ? 'disable' : 'enable'
    setTogglingId(record.id)
    try {
      await dnsApi.update(record.id, { status: next })
      message.success(next === 'enable' ? '记录已启用' : '记录已暂停')
      load()
    } finally {
      setTogglingId(null)
    }
  }

  const columns: ColumnsType<DNSRecord> = [
    {
      title: '主机记录',
      dataIndex: 'host',
      width: 130,
      render: (v: string, r) => (
        <span>
          {v}
          <span style={{ color: '#bfbfbf' }}>.{r.domain_name}</span>
        </span>
      ),
    },
    {
      title: '类型',
      dataIndex: 'type',
      width: 90,
      render: (v: string) => <RecordTypeTag type={v} />,
    },
    {
      title: '记录值',
      dataIndex: 'value',
      ellipsis: true,
    },
    {
      title: 'TTL',
      dataIndex: 'ttl',
      width: 90,
      render: (v: number) => `${v}s`,
    },
    {
      title: '优先级',
      dataIndex: 'priority',
      width: 80,
      render: (v: number) => v || <span style={{ color: '#bfbfbf' }}>-</span>,
    },
    {
      title: '状态',
      dataIndex: 'status',
      width: 90,
      render: (v: string, r) => (
        <Popconfirm
          title={v === 'enable' ? '确认暂停该解析记录？' : '确认启用该解析记录？'}
          okText="确认"
          cancelText="取消"
          onConfirm={() => toggleStatus(r)}
        >
          <Switch
            size="small"
            checked={v === 'enable'}
            loading={togglingId === r.id}
            checkedChildren="启用"
            unCheckedChildren="暂停"
          />
        </Popconfirm>
      ),
    },
    {
      title: '云厂商',
      dataIndex: 'provider',
      width: 90,
      render: (v: string) => <ProviderTag provider={v} />,
    },
    { title: '同步时间', dataIndex: 'synced_at', width: 150, render: (v: string) => formatTime(v) },
    {
      title: '操作',
      key: 'action',
      width: 120,
      fixed: 'right',
      render: (_, r) => (
        <Space size={0}>
          <Button type="link" size="small" onClick={() => openEdit(r)}>
            编辑
          </Button>
          <Popconfirm
            title="确认删除该解析记录？"
            description={`主机记录：${r.host}.${r.domain_name}`}
            onConfirm={() => handleDelete(r.id, `${r.host}.${r.domain_name}`)}
            okText="删除"
            okButtonProps={{ danger: true }}
            cancelText="取消"
          >
            <Button type="link" size="small" danger>
              删除
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ]

  const isMX = Form.useWatch('type', form) === 'MX'

  return (
    <div>
      <PageHeader title="域名解析" description="DNS 解析记录管理（A / CNAME / MX / TXT 等）" />

      <Card styles={{ body: { padding: 20 } }}>
        <Toolbar
          left={
            <>
              <Select
                placeholder="选择域名"
                showSearch
                style={{ width: 240 }}
                value={currentDomain || undefined}
                onChange={(v) => {
                  setCurrentDomain(v)
                  setPage(1)
                }}
                options={domains.map((d) => ({
                  value: d.domain_name,
                  label: `${d.domain_name}（${providerName(d.provider)}）`,
                }))}
              />
              <Select
                placeholder="记录类型"
                allowClear
                style={{ width: 120 }}
                value={typeFilter || undefined}
                onChange={(v) => {
                  setTypeFilter(v || '')
                  setPage(1)
                }}
                options={RECORD_TYPES.map((t) => ({ value: t, label: t }))}
              />
              <Input
                placeholder="搜索主机/记录值"
                allowClear
                prefix={<SearchOutlined />}
                style={{ width: 220 }}
                value={keyword}
                onChange={(e) => setKeyword(e.target.value)}
                onPressEnter={() => {
                  setPage(1)
                  load()
                }}
              />
            </>
          }
          right={
            <>
              <Button icon={<ReloadOutlined />} onClick={load} loading={loading}>
                刷新
              </Button>
              <Button type="primary" icon={<PlusOutlined />} onClick={openCreate} disabled={!currentDomain}>
                添加记录
              </Button>
            </>
          }
        />

        <Table
          rowKey="id"
          loading={loading}
          columns={columns}
          dataSource={data}
          scroll={{ x: 1100 }}
          size="middle"
          locale={{
            emptyText: !currentDomain ? (
              <Empty description="请先在上方选择一个域名" />
            ) : (
              <Empty
                description={`域名「${currentDomain}」暂无解析记录`}
                image={Empty.PRESENTED_IMAGE_SIMPLE}
              >
                <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
                  添加第一条解析
                </Button>
              </Empty>
            ),
          }}
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

      <Modal
        title={editing ? '编辑解析记录' : '添加解析记录'}
        open={modalOpen}
        onOk={handleSubmit}
        onCancel={() => setModalOpen(false)}
        okText="保存"
        cancelText="取消"
        destroyOnClose
      >
        <Form form={form} layout="vertical" style={{ marginTop: 16 }}>
          <Form.Item label="主机记录" name="host" rules={[{ required: true, message: '请输入主机记录' }]}>
            <Input placeholder="@ 或 www、api 等" addonAfter={`.${currentDomain}`} />
          </Form.Item>
          <Form.Item label="记录类型" name="type" rules={[{ required: true }]}>
            <Select options={RECORD_TYPES.map((t) => ({ value: t, label: t }))} />
          </Form.Item>
          <Form.Item label="记录值" name="value" rules={[{ required: true, message: '请输入记录值' }]}>
            <Input placeholder="如 IP 地址、CNAME 目标、MX 服务器" />
          </Form.Item>
          <Space style={{ width: '100%' }}>
            <Form.Item label="TTL（秒）" name="ttl" style={{ width: 200 }}>
              <InputNumber min={60} max={86400} style={{ width: '100%' }} />
            </Form.Item>
            {isMX && (
              <Form.Item label="优先级" name="priority" style={{ width: 200 }}>
                <InputNumber min={0} max={100} style={{ width: '100%' }} />
              </Form.Item>
            )}
          </Space>
        </Form>
      </Modal>
    </div>
  )
}

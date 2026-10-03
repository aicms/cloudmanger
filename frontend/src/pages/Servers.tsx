import { useEffect, useState, useCallback, useRef } from 'react'
import { useSearchParams } from 'react-router-dom'
import {
  Table,
  Input,
  Select,
  Space,
  Button,
  Tag,
  App,
  Drawer,
  Descriptions,
  Tooltip,
  Card,
  Empty,
  Modal,
  Form,
  message as antdMessage,
} from 'antd'
import type { ColumnsType, TableProps } from 'antd/es/table'
import {
  ReloadOutlined,
  PlayCircleOutlined,
  PauseCircleOutlined,
  SearchOutlined,
  ThunderboltOutlined,
  UndoOutlined,
  CopyOutlined,
  EyeOutlined,
  EditOutlined,
  CodeOutlined,
} from '@ant-design/icons'
import { serverApi } from '../api'
import type { Server } from '../types'
import { ProviderTag, ServerStatusTag } from '../components/Tags'
import { formatMemory, formatTime, daysUntil, providerName, regionName, isLightweight } from '../utils/format'
import PageHeader from '../components/PageHeader'
import Toolbar from '../components/Toolbar'
import WebShellModal from '../components/WebShellModal'

const EMPTY_FILTERS = { provider: '', region: '', status: '', keyword: '', tag: '' }

// 解析 tags 字符串 "k1:v1,k2:v2" 为 Tag 数组
function parseTags(tags: string): string[] {
  if (!tags) return []
  return tags
    .split(',')
    .map((t) => t.trim())
    .filter(Boolean)
}

export default function Servers() {
  const [searchParams] = useSearchParams()
  const [data, setData] = useState<Server[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(false)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(10)
  const [filters, setFilters] = useState(EMPTY_FILTERS)
  const [regions, setRegions] = useState<string[]>([])
  const [allTags, setAllTags] = useState<string[]>([])
  const [detail, setDetail] = useState<Server | null>(null)
  const [selectedIds, setSelectedIds] = useState<number[]>([])
  const [batchLoading, setBatchLoading] = useState(false)
  const [filterBatchLoading, setFilterBatchLoading] = useState<'' | 'start' | 'stop' | 'reboot'>('')
  const [actioningId, setActioningId] = useState<number | null>(null)
  const [editOpen, setEditOpen] = useState(false)
  const [editTarget, setEditTarget] = useState<Server | null>(null)
  const [editSaving, setEditSaving] = useState(false)
  const [editForm] = Form.useForm()
  const [wsOpen, setWsOpen] = useState(false)
  const [wsServer, setWsServer] = useState<Server | null>(null)
  const { message, modal } = App.useApp()
  const keywordTimer = useRef<ReturnType<typeof setTimeout> | null>(null)

  // 业务联动：从仪表盘带 status / keyword 进入时预填筛选
  useEffect(() => {
    const status = searchParams.get('status')
    const keyword = searchParams.get('keyword')
    if (status || keyword) {
      setFilters((f) => ({ ...f, status: status || '', keyword: keyword || '' }))
      setPage(1)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const res = await serverApi.list({ page, page_size: pageSize, ...filters })
      setData(res.list || [])
      setTotal(res.total)
    } finally {
      setLoading(false)
    }
  }, [page, pageSize, filters])

  useEffect(() => {
    serverApi.meta().then((m) => {
      setRegions(m.regions || [])
      setAllTags(m.tags || [])
    })
  }, [])

  useEffect(() => {
    load()
  }, [load])

  const doAction = (record: Server, action: 'start' | 'stop' | 'reboot') => {
    const textMap = { start: '开机', stop: '关机', reboot: '重启' }
    modal.confirm({
      title: `确认${textMap[action]}`,
      content: `将对服务器「${record.name}」(${record.instance_id}) 执行${textMap[action]}操作`,
      okText: '确认',
      cancelText: '取消',
      okButtonProps: { danger: action === 'stop' },
      onOk: async () => {
        setActioningId(record.id)
        try {
          await serverApi.action(record.id, action)
          message.success(`${textMap[action]}指令已下发`)
          load()
        } finally {
          setActioningId(null)
        }
      },
    })
  }

  const doBatch = (action: 'start' | 'stop' | 'reboot') => {
    if (selectedIds.length === 0) return
    const textMap = { start: '开机', stop: '关机', reboot: '重启' }
    modal.confirm({
      title: `批量${textMap[action]}`,
      content: `将对选中的 ${selectedIds.length} 台服务器执行${textMap[action]}操作`,
      okText: '确认',
      cancelText: '取消',
      okButtonProps: { danger: action === 'stop' },
      onOk: async () => {
        setBatchLoading(true)
        try {
          await Promise.all(selectedIds.map((id) => serverApi.action(id, action)))
          message.success(`已对 ${selectedIds.length} 台服务器下发${textMap[action]}指令`)
          setSelectedIds([])
          load()
        } finally {
          setBatchLoading(false)
        }
      },
    })
  }

  // 按当前筛选条件（含标签）批量操作
  const doFilterBatch = (action: 'start' | 'stop' | 'reboot') => {
    const textMap = { start: '开机', stop: '关机', reboot: '重启' }
    const matched = total // 当前筛选条件下的总数
    if (matched === 0) {
      message.warning('当前筛选条件下没有匹配的服务器')
      return
    }
    const filterDesc = [
      filters.provider && `云厂商：${providerName(filters.provider)}`,
      filters.region && `地域：${regionName(filters.region)}`,
      filters.status && `状态：${filters.status}`,
      filters.keyword && `关键词：${filters.keyword}`,
      filters.tag && `标签：${filters.tag}`,
    ]
      .filter(Boolean)
      .join('、')
    modal.confirm({
      title: `按筛选条件批量${textMap[action]}`,
      content: (
        <div>
          <p>
            将对符合以下筛选条件的 <strong>{matched}</strong> 台服务器执行{textMap[action]}操作
          </p>
          {filterDesc && (
            <p style={{ color: '#8c8c8c', fontSize: 12, marginBottom: 0 }}>{filterDesc}</p>
          )}
        </div>
      ),
      okText: '确认',
      cancelText: '取消',
      okButtonProps: { danger: action === 'stop' },
      onOk: async () => {
        setFilterBatchLoading(action)
        try {
          const res = await serverApi.batchAction({
            action,
            provider: filters.provider || undefined,
            region: filters.region || undefined,
            status: filters.status || undefined,
            keyword: filters.keyword || undefined,
            tag: filters.tag || undefined,
          })
          message.success(`已对 ${res.count} 台服务器下发${textMap[action]}指令`)
          load()
        } finally {
          setFilterBatchLoading('')
        }
      },
    })
  }

  const copy = (text: string, label: string) => {
    navigator.clipboard?.writeText(text).then(
      () => antdMessage.success(`${label}已复制`),
      () => antdMessage.error('复制失败'),
    )
  }

  // 打开「自定义名称 / 标签」编辑
  const openEdit = (record: Server) => {
    setEditTarget(record)
    editForm.setFieldsValue({
      custom_name: record.custom_name || '',
      tags: parseTags(record.tags),
    })
    setEditOpen(true)
  }

  const saveEdit = async () => {
    const values = await editForm.validateFields()
    if (!editTarget) return
    setEditSaving(true)
    try {
      await serverApi.update(editTarget.id, {
        custom_name: (values.custom_name || '').trim(),
        tags: (values.tags || []).join(','),
      })
      message.success('已保存自定义名称与标签')
      setEditOpen(false)
      load()
    } finally {
      setEditSaving(false)
    }
  }

  const displayName = (r: Server) => r.custom_name?.trim() || r.name

  const openWebShell = (record: Server) => {
    if (!record.public_ip) {
      message.warning('该服务器无公网 IP，无法建立远程终端')
      return
    }
    setWsServer(record)
    setWsOpen(true)
  }

  const columns: ColumnsType<Server> = [
    {
      title: '云厂商',
      dataIndex: 'provider',
      width: 90,
      render: (v: string) => <ProviderTag provider={v} />,
    },
    {
      title: '名称 / 标签',
      dataIndex: 'name',
      width: 220,
      render: (v: string, r) => (
        <Space direction="vertical" size={2}>
          <Space size={4}>
            <span
              className="cm-clickable"
              style={{ fontWeight: 600, color: '#1677ff', whiteSpace: 'nowrap' }}
              onClick={() => setDetail(r)}
            >
              {displayName(r)}
            </span>
            {isLightweight(r.category) && (
              <Tag color="green" style={{ marginInlineStart: 6, marginInlineEnd: 0, fontSize: 11 }}>
                轻量
              </Tag>
            )}
            <Button
              type="text"
              size="small"
              icon={<EditOutlined style={{ fontSize: 12 }} />}
              onClick={() => openEdit(r)}
              aria-label="编辑名称与标签"
            />
          </Space>
          {r.custom_name?.trim() && r.custom_name.trim() !== r.name && (
            <span style={{ fontSize: 12, color: '#bfbfbf' }}>原名称：{r.name}</span>
          )}
          {parseTags(r.tags).length > 0 && (
            <Space size={4} wrap>
              {parseTags(r.tags).map((t) => (
                <Tag key={t} style={{ marginInlineEnd: 0, fontSize: 11 }}>
                  {t}
                </Tag>
              ))}
            </Space>
          )}
        </Space>
      ),
    },
    {
      title: '公网 IP',
      dataIndex: 'public_ip',
      width: 140,
      render: (v: string) =>
        v ? (
          <Space size={4}>
            <span className="mono-num">{v}</span>
            <Button
              type="text"
              size="small"
              icon={<CopyOutlined style={{ fontSize: 11 }} />}
              onClick={() => copy(v, '公网 IP')}
            />
          </Space>
        ) : (
          <span style={{ color: '#bfbfbf' }}>-</span>
        ),
    },
    {
      title: '状态',
      dataIndex: 'status',
      width: 90,
      render: (v: string) => <ServerStatusTag status={v} />,
    },
    {
      title: '地域',
      dataIndex: 'region',
      width: 130,
      render: (v: string) => <span title={v}>{regionName(v)}</span>,
    },
    {
      title: '配置',
      width: 130,
      render: (_, r) => (
        <Space direction="vertical" size={0}>
          <span>
            {r.cpu} 核 {formatMemory(r.memory)}
          </span>
          <span style={{ fontSize: 12, color: '#8c8c8c' }}>
            {r.bandwidth > 0 ? `带宽 ${r.bandwidth} Mbps` : '带宽 -'}
          </span>
        </Space>
      ),
    },
    {
      title: '到期时间',
      dataIndex: 'expiry_date',
      width: 130,
      render: (v: string) => {
        if (!v) return <span style={{ color: '#bfbfbf' }}>-</span>
        const d = daysUntil(v)
        let color = '#595959'
        let remain: string
        if (d < 0) {
          color = '#cf1322'
          remain = '已过期'
        } else if (d <= 7) {
          color = '#cf1322'
          remain = `剩 ${d} 天`
        } else if (d <= 30) {
          color = '#d46b08'
          remain = `剩 ${d} 天`
        } else {
          remain = `剩 ${d} 天`
        }
        return (
          <Space direction="vertical" size={0}>
            <span>{v}</span>
            <span style={{ fontSize: 12, color }}>{remain}</span>
          </Space>
        )
      },
    },
    {
      title: '操作',
      key: 'action',
      width: 140,
      fixed: 'right',
      render: (_, r) => (
        <Space size={0}>
          <Tooltip title="开机">
            <Button
              type="link"
              size="small"
              icon={<PlayCircleOutlined />}
              disabled={r.status === 'Running' || actioningId === r.id}
              loading={actioningId === r.id && r.status !== 'Stopped'}
              onClick={() => doAction(r, 'start')}
            />
          </Tooltip>
          <Tooltip title="关机">
            <Button
              type="link"
              size="small"
              danger
              icon={<PauseCircleOutlined />}
              disabled={r.status === 'Stopped' || actioningId === r.id}
              onClick={() => doAction(r, 'stop')}
            />
          </Tooltip>
          <Tooltip title="重启">
            <Button
              type="link"
              size="small"
              icon={<ThunderboltOutlined />}
              disabled={r.status === 'Stopped' || actioningId === r.id}
              onClick={() => doAction(r, 'reboot')}
            />
          </Tooltip>
          <Tooltip title="远程终端 (WebShell)">
            <Button
              type="link"
              size="small"
              icon={<CodeOutlined />}
              disabled={!r.public_ip}
              onClick={() => openWebShell(r)}
            />
          </Tooltip>
        </Space>
      ),
    },
  ]

  const rowSelection: TableProps<Server>['rowSelection'] = {
    selectedRowKeys: selectedIds,
    onChange: (keys) => setSelectedIds(keys as number[]),
  }

  const hasFilters = Object.values(filters).some(Boolean)
  const resetFilters = () => {
    setFilters(EMPTY_FILTERS)
    setPage(1)
  }

  // 关键词输入防抖，自动触发搜索
  const onKeywordChange = (val: string) => {
    setFilters((f) => ({ ...f, keyword: val }))
    if (keywordTimer.current) clearTimeout(keywordTimer.current)
    keywordTimer.current = setTimeout(() => {
      setPage(1)
      load()
    }, 400)
  }

  return (
    <div>
      <PageHeader title="云服务器" description="跨云统一管理（阿里云 ECS / 腾讯云 CVM）" />

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
                placeholder="地域"
                allowClear
                showSearch
                style={{ width: 180 }}
                value={filters.region || undefined}
                onChange={(v) => {
                  setFilters((f) => ({ ...f, region: v || '' }))
                  setPage(1)
                }}
                options={regions.map((r) => ({ value: r, label: regionName(r) }))}
              />
              <Select
                placeholder="状态"
                allowClear
                style={{ width: 120 }}
                value={filters.status || undefined}
                onChange={(v) => {
                  setFilters((f) => ({ ...f, status: v || '' }))
                  setPage(1)
                }}
                options={[
                  { value: 'Running', label: '运行中' },
                  { value: 'Stopped', label: '已停止' },
                  { value: 'Starting', label: '启动中' },
                ]}
              />
              <Input
                placeholder="搜索名称/IP/实例ID"
                allowClear
                prefix={<SearchOutlined />}
                style={{ width: 220 }}
                value={filters.keyword}
                onChange={(e) => onKeywordChange(e.target.value)}
                onPressEnter={() => {
                  setPage(1)
                  load()
                }}
              />
              {allTags.length > 0 && (
                <Select
                  mode="multiple"
                  maxTagCount="responsive"
                  placeholder="按标签筛选"
                  allowClear
                  style={{ minWidth: 180 }}
                  value={filters.tag ? filters.tag.split(',') : undefined}
                  onChange={(vals: string[]) => {
                    setFilters((f) => ({ ...f, tag: vals.join(',') }))
                    setPage(1)
                  }}
                  options={allTags.map((t) => ({ value: t, label: t }))}
                />
              )}
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

        {/* 按筛选条件批量操作栏 */}
        {hasFilters && (
          <Space
            style={{
              marginBottom: 12,
              padding: '8px 16px',
              background: '#f6ffed',
              borderRadius: 8,
              border: '1px solid #b7eb8f',
            }}
          >
            <span>
              当前筛选条件下共 <strong>{total}</strong> 台服务器
            </span>
            <Button
              size="small"
              type="primary"
              ghost
              icon={<PlayCircleOutlined />}
              loading={filterBatchLoading === 'start'}
              onClick={() => doFilterBatch('start')}
            >
              批量开机
            </Button>
            <Button
              size="small"
              danger
              ghost
              icon={<PauseCircleOutlined />}
              loading={filterBatchLoading === 'stop'}
              onClick={() => doFilterBatch('stop')}
            >
              批量关机
            </Button>
            <Button
              size="small"
              ghost
              icon={<ThunderboltOutlined />}
              loading={filterBatchLoading === 'reboot'}
              onClick={() => doFilterBatch('reboot')}
            >
              批量重启
            </Button>
            <Button size="small" type="link" icon={<UndoOutlined />} onClick={resetFilters}>
              清除筛选
            </Button>
          </Space>
        )}

        {/* 批量操作栏 */}
        {selectedIds.length > 0 && (
          <Space
            style={{
              marginBottom: 12,
              padding: '8px 16px',
              background: '#e6f4ff',
              borderRadius: 8,
              border: '1px solid #91caff',
            }}
          >
            <span>
              已选择 <strong>{selectedIds.length}</strong> 台服务器
            </span>
            <Button size="small" type="primary" icon={<PlayCircleOutlined />} loading={batchLoading} onClick={() => doBatch('start')}>
              批量开机
            </Button>
            <Button size="small" danger icon={<PauseCircleOutlined />} loading={batchLoading} onClick={() => doBatch('stop')}>
              批量关机
            </Button>
            <Button size="small" icon={<ThunderboltOutlined />} loading={batchLoading} onClick={() => doBatch('reboot')}>
              批量重启
            </Button>
            <Button size="small" type="link" onClick={() => setSelectedIds([])}>
              取消选择
            </Button>
          </Space>
        )}

        <Table
          rowKey="id"
          loading={loading}
          columns={columns}
          dataSource={data}
          rowSelection={rowSelection}
          size="middle"
          scroll={{ x: 1130 }}
          locale={{
            emptyText: (
              <Empty description="暂无云服务器">
                <span style={{ color: '#8c8c8c', fontSize: 13 }}>
                  请先在「云账号」中同步资源，或调整筛选条件
                </span>
              </Empty>
            ),
          }}
          pagination={{
            current: page,
            pageSize,
            total,
            showSizeChanger: true,
            showTotal: (t) => `共 ${t} 台`,
            onChange: (p, ps) => {
              setPage(p)
              setPageSize(ps)
            },
          }}
        />
      </Card>

      {/* 详情抽屉 */}
      <Drawer
        title={
          detail ? (
            <Space size={6}>
              {displayName(detail)}
              {isLightweight(detail.category) && (
                <Tag color="green" style={{ marginInlineEnd: 0, fontSize: 11 }}>
                  轻量
                </Tag>
              )}
            </Space>
          ) : (
            ''
          )
        }
        open={!!detail}
        onClose={() => setDetail(null)}
        width={520}
      >
        {detail && (
          <>
          <Descriptions column={1} bordered size="small">
            <Descriptions.Item label="云厂商">{providerName(detail.provider)}</Descriptions.Item>
            <Descriptions.Item label="状态">
              <ServerStatusTag status={detail.status} />
            </Descriptions.Item>
            <Descriptions.Item label="实例 ID">
              <Space size={4}>
                {detail.instance_id}
                <Button type="text" size="small" icon={<CopyOutlined />} onClick={() => copy(detail.instance_id, '实例 ID')} />
              </Space>
            </Descriptions.Item>
            <Descriptions.Item label="实例规格">{detail.instance_type}</Descriptions.Item>
            <Descriptions.Item label="CPU">{detail.cpu} 核</Descriptions.Item>
            <Descriptions.Item label="内存">{formatMemory(detail.memory)}</Descriptions.Item>
            <Descriptions.Item label="操作系统">{detail.os}</Descriptions.Item>
            <Descriptions.Item label="地域">{regionName(detail.region)}</Descriptions.Item>
            <Descriptions.Item label="可用区">{detail.zone}</Descriptions.Item>
            <Descriptions.Item label="公网 IP">
              {detail.public_ip ? (
                <Space size={4}>
                  {detail.public_ip}
                  <Button type="text" size="small" icon={<CopyOutlined />} onClick={() => copy(detail.public_ip, '公网 IP')} />
                </Space>
              ) : (
                '-'
              )}
            </Descriptions.Item>
            <Descriptions.Item label="内网 IP">{detail.inner_ip || '-'}</Descriptions.Item>
            <Descriptions.Item label="带宽">{detail.bandwidth > 0 ? `${detail.bandwidth} Mbps` : '-'}</Descriptions.Item>
            <Descriptions.Item label="标签">
              {parseTags(detail.tags).length > 0 ? (
                <Space size={4} wrap>
                  {parseTags(detail.tags).map((t) => (
                    <Tag key={t}>{t}</Tag>
                  ))}
                </Space>
              ) : (
                '-'
              )}
            </Descriptions.Item>
            <Descriptions.Item label="同步时间">{formatTime(detail.synced_at)}</Descriptions.Item>
          </Descriptions>
          <Space style={{ marginTop: 16, width: '100%' }} direction="vertical">
            <Button
              type="primary"
              ghost
              icon={<CodeOutlined />}
              block
              disabled={!detail?.public_ip}
              onClick={() => detail && openWebShell(detail)}
            >
              远程终端 (WebShell)
            </Button>
            <Button
              type="primary"
              ghost
              icon={<EditOutlined />}
              block
              onClick={() => detail && openEdit(detail)}
            >
              编辑名称与标签
            </Button>
          </Space>
          </>
        )}
      </Drawer>

      {/* 自定义名称 / 标签 编辑弹窗 */}
      <Modal
        title="编辑名称与标签"
        open={editOpen}
        onOk={saveEdit}
        onCancel={() => setEditOpen(false)}
        confirmLoading={editSaving}
        okText="保存"
        cancelText="取消"
        destroyOnClose
      >
        <Form form={editForm} layout="vertical" style={{ marginTop: 16 }}>
          <Form.Item
            label="自定义名称"
            name="custom_name"
            extra="设置后将覆盖云厂商原始名称，方便识别；留空则显示原名称"
          >
            <Input placeholder="如：生产-Web 网关" maxLength={128} allowClear />
          </Form.Item>
          <Form.Item
            label="标签"
            name="tags"
            extra="用于分类与筛选，可输入后回车添加自定义标签"
          >
            <Select mode="tags" placeholder="选择或输入标签" options={allTags.map((t) => ({ value: t }))} />
          </Form.Item>
        </Form>
      </Modal>

      {/* 远程终端 */}
      <WebShellModal open={wsOpen} server={wsServer} onClose={() => setWsOpen(false)} />
    </div>
  )
}

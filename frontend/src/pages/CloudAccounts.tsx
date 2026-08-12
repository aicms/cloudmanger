import { useEffect, useState, useCallback, useRef } from 'react'
import {
  Table,
  Button,
  Space,
  Popconfirm,
  Tag,
  App,
  Drawer,
  Form,
  Input,
  Select,
  Switch,
  Tooltip,
  Card,
  Empty,
} from 'antd'
import type { ColumnsType, TableProps } from 'antd/es/table'
import {
  PlusOutlined,
  ReloadOutlined,
  EditOutlined,
  DeleteOutlined,
  SyncOutlined,
  CopyOutlined,
  ImportOutlined,
} from '@ant-design/icons'
import {
  cloudAccountApi,
  type CloudAccountForm,
  type CloudAccountBatchSyncResult,
  type CloudAccountImportResult,
} from '../api'
import type { CloudAccount } from '../types'
import { ProviderTag } from '../components/Tags'
import { formatTime, providerName } from '../utils/format'
import PageHeader from '../components/PageHeader'

const REGION_OPTIONS: Record<string, { value: string; label: string }[]> = {
  aliyun: [
    { value: 'cn-hangzhou', label: '华东 1（杭州）' },
    { value: 'cn-shanghai', label: '华东 2（上海）' },
    { value: 'cn-beijing', label: '华北 2（北京）' },
    { value: 'cn-shenzhen', label: '华南 1（深圳）' },
    { value: 'cn-guangzhou', label: '华南 5（广州）' },
  ],
  tencent: [
    { value: 'ap-guangzhou', label: '华南地区（广州）' },
    { value: 'ap-shanghai', label: '华东地区（上海）' },
    { value: 'ap-beijing', label: '华北地区（北京）' },
    { value: 'ap-nanjing', label: '华东地区（南京）' },
    { value: 'ap-chengdu', label: '西南地区（成都）' },
  ],
}

export default function CloudAccounts() {
  const [data, setData] = useState<CloudAccount[]>([])
  const [loading, setLoading] = useState(false)
  const [drawerOpen, setDrawerOpen] = useState(false)
  const [editing, setEditing] = useState<CloudAccount | null>(null)
  const [syncing, setSyncing] = useState<number | null>(null)
  const [selectedIds, setSelectedIds] = useState<number[]>([])
  const [batchSyncing, setBatchSyncing] = useState(false)
  const [syncingAll, setSyncingAll] = useState(false)
  const [importing, setImporting] = useState(false)
  const importInputRef = useRef<HTMLInputElement>(null)
  const [form] = Form.useForm()
  const { message, modal } = App.useApp()
  const provider = Form.useWatch('provider', form) as string | undefined

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const res = await cloudAccountApi.list()
      setData(res || [])
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    load()
  }, [load])

  const openCreate = () => {
    setEditing(null)
    form.resetFields()
    form.setFieldsValue({ provider: 'aliyun', region: 'cn-hangzhou' })
    setDrawerOpen(true)
  }

  const openEdit = (record: CloudAccount) => {
    setEditing(record)
    form.setFieldsValue({
      name: record.name,
      provider: record.provider,
      access_key_id: record.access_key_id,
      region: record.region,
      remark: record.remark,
    })
    setDrawerOpen(true)
  }

  const handleSubmit = async () => {
    const values: CloudAccountForm = await form.validateFields()
    if (editing) {
      await cloudAccountApi.update(editing.id, values)
      message.success('云账号已更新')
    } else {
      await cloudAccountApi.create(values)
      message.success('云账号已添加')
    }
    setDrawerOpen(false)
    load()
  }

  const handleSync = async (id: number, name: string) => {
    setSyncing(id)
    const hide = message.loading(`正在同步「${name}」的资源...`, 0)
    try {
      await cloudAccountApi.sync(id)
      hide()
      message.success(`账号「${name}」同步完成`)
      load()
    } catch {
      hide()
    } finally {
      setSyncing(null)
    }
  }

  const showBatchResult = (res: CloudAccountBatchSyncResult) => {
    if (res.failed > 0) {
      modal.warning({
        title: '批量同步完成（存在失败）',
        content: `成功 ${res.success} 个，失败 ${res.failed} 个。失败账号：${res.results
          .filter((r) => !r.ok)
          .map((r) => r.name)
          .join('、')}`,
      })
    } else {
      message.success(`全部 ${res.success} 个账号同步成功`)
    }
  }

  const handleBatchSync = async () => {
    if (selectedIds.length === 0) return
    modal.confirm({
      title: `批量同步 ${selectedIds.length} 个云账号`,
      content: `将对选中的 ${selectedIds.length} 个云账号依次拉取最新资源，是否继续？`,
      okText: '开始同步',
      cancelText: '取消',
      onOk: async () => {
        const hide = message.loading(`正在批量同步 ${selectedIds.length} 个账号...`, 0)
        setBatchSyncing(true)
        try {
          const res = await cloudAccountApi.batchSync(selectedIds)
          hide()
          showBatchResult(res)
          setSelectedIds([])
          load()
        } catch {
          hide()
        } finally {
          setBatchSyncing(false)
        }
      },
    })
  }

  const handleSyncAll = async () => {
    modal.confirm({
      title: '同步全部云账号',
      content: '将对所有云账号依次拉取最新资源，是否继续？',
      okText: '开始同步',
      cancelText: '取消',
      onOk: async () => {
        const hide = message.loading('正在同步全部云账号...', 0)
        setSyncingAll(true)
        try {
          const res = await cloudAccountApi.batchSync([])
          hide()
          showBatchResult(res)
          load()
        } catch {
          hide()
        } finally {
          setSyncingAll(false)
        }
      },
    })
  }

  const showImportResult = (res: CloudAccountImportResult) => {
    if (res.failed > 0) {
      modal.warning({
        title: '导入完成（存在失败）',
        content: `共 ${res.total} 条，成功 ${res.success} 条，失败 ${res.failed} 条。${res.results
          .filter((r) => !r.ok)
          .map((r) => `第 ${r.row} 行：${r.error}`)
          .join('；')}`,
      })
    } else {
      message.success(`成功导入 ${res.success} 个云账号`)
    }
  }

  const handleImportClick = () => importInputRef.current?.click()

  const handleImportFile = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    e.target.value = ''
    if (!file) return
    setImporting(true)
    try {
      const res = await cloudAccountApi.import(file)
      showImportResult(res)
      load()
    } finally {
      setImporting(false)
    }
  }

  const rowSelection: TableProps<CloudAccount>['rowSelection'] = {
    selectedRowKeys: selectedIds,
    onChange: (keys) => setSelectedIds(keys as number[]),
  }

  const copyKey = (text: string) => {
    navigator.clipboard?.writeText(text).then(
      () => message.success('AccessKey ID 已复制'),
      () => message.error('复制失败'),
    )
  }

  const handleToggle = async (record: CloudAccount, enabled: boolean) => {
    await cloudAccountApi.toggle(record.id, enabled)
    message.success(enabled ? '已启用' : '已禁用')
    load()
  }

  const columns: ColumnsType<CloudAccount> = [
    {
      title: '账号名称',
      dataIndex: 'name',
      render: (v: string) => <strong>{v}</strong>,
    },
    {
      title: '云厂商',
      dataIndex: 'provider',
      width: 100,
      render: (v: string) => <ProviderTag provider={v} />,
    },
    {
      title: 'AccessKey ID',
      dataIndex: 'access_key_masked',
      render: (v: string) => (
        <Space size={4}>
          <span style={{ fontFamily: 'monospace', fontSize: 12 }}>{v}</span>
          <Button type="text" size="small" icon={<CopyOutlined style={{ fontSize: 11 }} />} onClick={() => copyKey(v)} />
        </Space>
      ),
    },
    { title: '默认地域', dataIndex: 'region', width: 140 },
    {
      title: '状态',
      dataIndex: 'enabled',
      width: 90,
      render: (v: boolean, r) => (
        <Switch size="small" checked={v} onChange={(c) => handleToggle(r, c)} />
      ),
    },
    {
      title: '最近同步',
      dataIndex: 'last_synced_at',
      width: 160,
      render: (v: string) => formatTime(v),
    },
    {
      title: '操作',
      key: 'action',
      width: 220,
      fixed: 'right',
      render: (_, r) => (
        <Space size={0}>
          <Tooltip title="同步资源">
            <Button
              type="link"
              size="small"
              icon={<SyncOutlined spin={syncing === r.id} />}
              loading={syncing === r.id}
              disabled={syncingAll || batchSyncing}
              onClick={() => handleSync(r.id, r.name)}
            />
          </Tooltip>
          <Button type="link" size="small" icon={<EditOutlined />} onClick={() => openEdit(r)} />
          <Popconfirm
            title="确认删除该云账号？"
            description="关联的缓存资源将保留，可在下次同步时刷新"
            onConfirm={async () => {
              await cloudAccountApi.remove(r.id)
              message.success('已删除')
              load()
            }}
            okText="删除"
            okButtonProps={{ danger: true }}
            cancelText="取消"
          >
            <Button type="link" size="small" danger icon={<DeleteOutlined />} />
          </Popconfirm>
        </Space>
      ),
    },
  ]

  return (
    <div>
      <PageHeader
        title="云账号管理"
        description="管理阿里云 / 腾讯云 API 密钥（密钥加密存储）"
        extra={
          <>
            <input
              ref={importInputRef}
              type="file"
              accept=".csv"
              style={{ display: 'none' }}
              onChange={handleImportFile}
            />
            <Tooltip title="选择阿里云 AccessKey 或腾讯云 SecretKey 官方导出的 CSV 文件，自动识别厂商并批量添加">
              <Button icon={<ImportOutlined />} onClick={handleImportClick} loading={importing}>
                导入密钥
              </Button>
            </Tooltip>
            <Button icon={<SyncOutlined spin={syncingAll} />} onClick={handleSyncAll} loading={syncingAll}>
              同步全部
            </Button>
            <Button icon={<ReloadOutlined />} onClick={load} loading={loading}>
              刷新
            </Button>
            <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
              添加云账号
            </Button>
          </>
        }
      />

      <Card styles={{ body: { padding: 20 } }}>
        {/* 批量同步操作栏 */}
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
              已选择 <strong>{selectedIds.length}</strong> 个云账号
            </span>
            <Button
              size="small"
              type="primary"
              ghost
              icon={<SyncOutlined />}
              loading={batchSyncing}
              onClick={handleBatchSync}
            >
              批量同步
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
          scroll={{ x: 1000 }}
          pagination={false}
          locale={{
            emptyText: (
              <Empty description="尚未添加云账号">
                <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
                  添加云账号
                </Button>
              </Empty>
            ),
          }}
        />
      </Card>

      <Drawer
        title={editing ? '编辑云账号' : '添加云账号'}
        open={drawerOpen}
        onClose={() => setDrawerOpen(false)}
        width={460}
        extra={
          <Space>
            <Button onClick={() => setDrawerOpen(false)}>取消</Button>
            <Button type="primary" onClick={handleSubmit}>
              保存
            </Button>
          </Space>
        }
      >
        <Form form={form} layout="vertical">
          <Form.Item label="账号名称" name="name" rules={[{ required: true, message: '请输入账号名称' }]}>
            <Input placeholder="如：阿里云-生产账号" />
          </Form.Item>
          <Form.Item label="云厂商" name="provider" rules={[{ required: true }]}>
            <Select
              options={[
                { value: 'aliyun', label: '阿里云 (Aliyun)' },
                { value: 'tencent', label: '腾讯云 (Tencent Cloud)' },
              ]}
            />
          </Form.Item>
          <Form.Item label="AccessKey ID" name="access_key_id"
            rules={[{ required: true, message: '请输入 AccessKey ID' }]}>
            <Input placeholder="LTAI... / AKID..." />
          </Form.Item>
          <Form.Item
            label="AccessKey Secret"
            name="access_key_secret"
            rules={[{ required: !editing, message: '请输入 AccessKey Secret' }]}
            extra={editing ? '留空则不修改原密钥' : '将加密存储，不会明文返回'}
          >
            <Input.Password placeholder={editing ? '编辑时留空表示不修改' : '请输入 AccessKey Secret'} />
          </Form.Item>
          <Form.Item label="默认地域" name="region">
            <Select
              showSearch
              allowClear
              placeholder="选择或输入地域"
              options={REGION_OPTIONS[provider || 'aliyun'] || []}
            />
          </Form.Item>
          <Form.Item label="备注" name="remark">
            <Input.TextArea rows={2} placeholder="可选" />
          </Form.Item>
        </Form>
      </Drawer>
    </div>
  )
}

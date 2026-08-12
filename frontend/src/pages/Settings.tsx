import { useState, useEffect } from 'react'
import {
  Card,
  Form,
  Input,
  Button,
  Alert,
  InputNumber,
  Tabs,
  Segmented,
  App,
  Space,
  Typography,
} from 'antd'
import { authApi, settingsApi, type SettingsData } from '../api'
import PageHeader from '../components/PageHeader'
import { useThemeStore } from '../stores/theme'
import { useAppStore } from '../stores/app'

const { Text } = Typography

export default function Settings() {
  const { message } = App.useApp()
  const themeMode = useThemeStore((s) => s.mode)
  const setThemeMode = useThemeStore((s) => s.setMode)
  const setAppPlatformName = useAppStore((s) => s.setPlatformName)
  const setAppSSH = useAppStore((s) => s.setSSH)

  const [data, setData] = useState<SettingsData | null>(null)
  const [form] = Form.useForm()
  const [saving, setSaving] = useState<Record<string, boolean>>({})

  // 各分区表单本地状态（受控），初始化自后端设置
  const [platformName, setPlatformName] = useState('')
  const [syncConcurrency, setSyncConcurrency] = useState(3)
  const [syncInterval, setSyncInterval] = useState(0)
  const [sshUser, setSshUser] = useState('root')
  const [sshPort, setSshPort] = useState(22)
  const [sessionTimeout, setSessionTimeout] = useState(24)

  useEffect(() => {
    settingsApi.get().then((s) => {
      setData(s)
      setPlatformName(s.platform_name)
      setSyncConcurrency(s.sync_concurrency)
      setSyncInterval(s.sync_interval)
      setSshUser(s.ssh_user)
      setSshPort(s.ssh_port)
      setSessionTimeout(s.session_timeout)
    })
  }, [])

  const setSavingKey = (key: string, v: boolean) =>
    setSaving((s) => ({ ...s, [key]: v }))

  // 通用 / 外观
  const handleSaveGeneral = async () => {
    if (!data) return
    setSavingKey('general', true)
    try {
      const name = platformName.trim() || data.platform_name
      await settingsApi.update({ platform_name: name, theme: themeMode })
      setAppPlatformName(name)
      message.success('通用设置已保存')
      setData({ ...data, platform_name: name })
    } finally {
      setSavingKey('general', false)
    }
  }

  // 同步设置
  const handleSaveSync = async () => {
    if (!data) return
    setSavingKey('sync', true)
    try {
      await settingsApi.update({ sync_concurrency: syncConcurrency, sync_interval: syncInterval })
      message.success(
        syncInterval > 0 ? `已保存，每 ${syncInterval} 分钟自动同步一次` : '已保存，自动同步已关闭'
      )
      setData({ ...data, sync_concurrency: syncConcurrency, sync_interval: syncInterval })
    } finally {
      setSavingKey('sync', false)
    }
  }

  // SSH 默认连接
  const handleSaveSSH = async () => {
    if (!data) return
    setSavingKey('ssh', true)
    try {
      await settingsApi.update({ ssh_user: sshUser, ssh_port: sshPort })
      setAppSSH(sshUser, sshPort)
      message.success('SSH 默认连接已保存')
      setData({ ...data, ssh_user: sshUser, ssh_port: sshPort })
    } finally {
      setSavingKey('ssh', false)
    }
  }

  // 安全设置（会话超时）
  const handleSaveSecurity = async () => {
    if (!data) return
    setSavingKey('security', true)
    try {
      await settingsApi.update({ session_timeout: sessionTimeout })
      message.success(`会话超时已更新为 ${sessionTimeout} 小时（下次登录生效）`)
      setData({ ...data, session_timeout: sessionTimeout })
    } finally {
      setSavingKey('security', false)
    }
  }

  const handleChangePassword = async () => {
    const values = await form.validateFields()
    await authApi.changePassword(values.old_password, values.new_password)
    message.success('密码修改成功')
    form.resetFields()
  }

  if (!data) {
    return (
      <div>
        <PageHeader title="系统设置" description="平台基础信息与账号安全" />
      </div>
    )
  }

  const items = [
    {
      key: 'general',
      label: '通用 / 外观',
      children: (
        <Card>
          <Alert
            type="info"
            showIcon
            style={{ marginBottom: 16 }}
            message="平台名称显示在左侧导航品牌区；主题切换即时生效并会在刷新后保持。"
          />
          <Space direction="vertical" size={20} style={{ width: '100%' }}>
            <div>
              <Text strong>平台名称</Text>
              <div style={{ marginTop: 8 }}>
                <Input
                  value={platformName}
                  maxLength={32}
                  onChange={(e) => setPlatformName(e.target.value)}
                  style={{ maxWidth: 360 }}
                  placeholder="多云管理平台"
                />
              </div>
            </div>
            <div>
              <Text strong>界面主题</Text>
              <div style={{ marginTop: 8 }}>
                <Segmented
                  value={themeMode}
                  onChange={(v) => {
                    const m = v as 'light' | 'dark'
                    setThemeMode(m)
                    settingsApi.update({ theme: m }).then(() => {
                      setData({ ...data, theme: m })
                    })
                  }}
                  options={[
                    { label: '浅色', value: 'light' },
                    { label: '深色', value: 'dark' },
                  ]}
                />
              </div>
            </div>
            <Button type="primary" loading={saving.general} onClick={handleSaveGeneral}>
              保存
            </Button>
          </Space>
        </Card>
      ),
    },
    {
      key: 'sync',
      label: '同步设置',
      children: (
        <Card>
          <Alert
            type="info"
            showIcon
            style={{ marginBottom: 16 }}
            message="批量同步云账号时的最大并发数；自动同步按设定间隔定时拉取全部账号资源（0 表示关闭）。"
          />
          <Space direction="vertical" size={20} style={{ width: '100%' }}>
            <div>
              <Text strong>批量同步并发数</Text>
              <div style={{ marginTop: 8 }}>
                <InputNumber
                  min={data.sync_concurrency_min}
                  max={data.sync_concurrency_max}
                  value={syncConcurrency}
                  onChange={(v) => setSyncConcurrency(v ?? 3)}
                  style={{ width: 160 }}
                />
              </div>
            </div>
            <div>
              <Text strong>自动同步间隔（分钟）</Text>
              <div style={{ marginTop: 8 }}>
                <InputNumber
                  min={0}
                  max={1440}
                  value={syncInterval}
                  onChange={(v) => setSyncInterval(v ?? 0)}
                  style={{ width: 160 }}
                />
                <Text type="secondary" style={{ marginLeft: 12 }}>
                  0 = 关闭自动同步
                </Text>
              </div>
            </div>
            <Button type="primary" loading={saving.sync} onClick={handleSaveSync}>
              保存
            </Button>
          </Space>
        </Card>
      ),
    },
    {
      key: 'ssh',
      label: 'SSH 默认连接',
      children: (
        <Card>
          <Alert
            type="info"
            showIcon
            style={{ marginBottom: 16 }}
            message="打开 WebShell 远程终端时自动预填以下用户名与端口，可手动修改。"
          />
          <Space direction="vertical" size={20} style={{ width: '100%' }}>
            <div>
              <Text strong>默认用户名</Text>
              <div style={{ marginTop: 8 }}>
                <Input
                  value={sshUser}
                  onChange={(e) => setSshUser(e.target.value)}
                  style={{ maxWidth: 240 }}
                  placeholder="root"
                />
              </div>
            </div>
            <div>
              <Text strong>默认端口</Text>
              <div style={{ marginTop: 8 }}>
                <InputNumber
                  min={1}
                  max={65535}
                  value={sshPort}
                  onChange={(v) => setSshPort(v ?? 22)}
                  style={{ width: 160 }}
                />
              </div>
            </div>
            <Button type="primary" loading={saving.ssh} onClick={handleSaveSSH}>
              保存
            </Button>
          </Space>
        </Card>
      ),
    },
    {
      key: 'security',
      label: '安全设置',
      children: (
        <Space direction="vertical" size={16} style={{ width: '100%' }}>
          <Card title="会话超时">
            <Space direction="vertical" size={20} style={{ width: '100%' }}>
              <div>
                <Text strong>登录会话有效期（小时）</Text>
                <div style={{ marginTop: 8 }}>
                  <InputNumber
                    min={1}
                    max={720}
                    value={sessionTimeout}
                    onChange={(v) => setSessionTimeout(v ?? 24)}
                    style={{ width: 160 }}
                  />
                  <Text type="secondary" style={{ marginLeft: 12 }}>
                    修改后下次登录生效
                  </Text>
                </div>
              </div>
              <Button type="primary" loading={saving.security} onClick={handleSaveSecurity}>
                保存
              </Button>
            </Space>
          </Card>
          <Card title="修改密码">
            <Form form={form} layout="vertical">
              <Form.Item
                label="原密码"
                name="old_password"
                rules={[{ required: true, message: '请输入原密码' }]}
              >
                <Input.Password placeholder="请输入原密码" />
              </Form.Item>
              <Form.Item
                label="新密码"
                name="new_password"
                rules={[
                  { required: true, message: '请输入新密码' },
                  { min: 6, message: '密码至少 6 位' },
                ]}
              >
                <Input.Password placeholder="至少 6 位" />
              </Form.Item>
              <Form.Item
                label="确认新密码"
                name="confirm"
                dependencies={['new_password']}
                rules={[
                  { required: true, message: '请再次输入新密码' },
                  ({ getFieldValue }) => ({
                    validator(_, value) {
                      if (!value || getFieldValue('new_password') === value) {
                        return Promise.resolve()
                      }
                      return Promise.reject(new Error('两次输入的密码不一致'))
                    },
                  }),
                ]}
              >
                <Input.Password placeholder="再次输入新密码" />
              </Form.Item>
              <Form.Item style={{ marginBottom: 0 }}>
                <Button type="primary" onClick={handleChangePassword}>
                  确认修改
                </Button>
              </Form.Item>
            </Form>
          </Card>
        </Space>
      ),
    },
  ]

  return (
    <div>
      <PageHeader title="系统设置" description="平台基础信息与账号安全" />
      <Card>
        <Tabs defaultActiveKey="general" items={items} />
      </Card>
    </div>
  )
}

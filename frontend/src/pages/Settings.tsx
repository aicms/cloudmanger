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
  const [saving, setSaving] = useState(false)

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

  // 统一保存：通用 / 同步 / SSH / 会话超时 一次提交
  const handleSaveAll = async () => {
    if (!data) return
    // 安全设置里填写了新密码时，一并修改密码（校验不通过则中止，不保存其它设置）
    const wantChangePwd = !!form.getFieldValue('new_password')
    if (wantChangePwd) {
      try {
        await form.validateFields()
      } catch {
        message.error('请检查修改密码表单：原密码、新密码（至少 6 位）、确认密码')
        return
      }
    }
    setSaving(true)
    try {
      if (wantChangePwd) {
        const values = form.getFieldsValue()
        await authApi.changePassword(values.old_password, values.new_password)
        form.resetFields()
      }
      const name = platformName.trim() || data.platform_name
      await settingsApi.update({
        platform_name: name,
        theme: themeMode,
        sync_concurrency: syncConcurrency,
        sync_interval: syncInterval,
        ssh_user: sshUser,
        ssh_port: sshPort,
        session_timeout: sessionTimeout,
      })
      setAppPlatformName(name)
      setAppSSH(sshUser, sshPort)
      setPlatformName(name)
      setData({
        ...data,
        platform_name: name,
        theme: themeMode,
        sync_concurrency: syncConcurrency,
        sync_interval: syncInterval,
        ssh_user: sshUser,
        ssh_port: sshPort,
        session_timeout: sessionTimeout,
      })
      message.success(wantChangePwd ? '设置已保存，密码已修改' : '设置已保存')
    } finally {
      setSaving(false)
    }
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
        <div>
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
          </Space>
        </div>
      ),
    },
    {
      key: 'sync',
      label: '同步设置',
      children: (
        <div>
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
          </Space>
        </div>
      ),
    },
    {
      key: 'ssh',
      label: 'SSH 默认连接',
      children: (
        <div>
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
          </Space>
        </div>
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
            </Space>
          </Card>
          <Card title="修改密码" extra={<Text type="secondary" style={{ fontSize: 13 }}>不修改密码请留空</Text>}>
            <Form form={form} layout="vertical">
              <Form.Item
                label="原密码"
                name="old_password"
                rules={[{ required: true, message: '请输入原密码' }]}
              >
                <Input.Password placeholder="请输入原密码" autoComplete="new-password" />
              </Form.Item>
              <Form.Item
                label="新密码"
                name="new_password"
                rules={[
                  { required: true, message: '请输入新密码' },
                  { min: 6, message: '密码至少 6 位' },
                ]}
              >
                <Input.Password placeholder="至少 6 位" autoComplete="new-password" />
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
                <Input.Password placeholder="再次输入新密码" autoComplete="new-password" />
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
      <div className="cm-save-bar">
        <Text type="secondary" style={{ fontSize: 13 }}>
          设置修改统一保存；若填写了新密码，会一并修改密码
        </Text>
        <Button type="primary" loading={saving} onClick={handleSaveAll}>
          保存
        </Button>
      </div>
    </div>
  )
}

import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Card, Form, Input, Button, Typography, App, Checkbox } from 'antd'
import { UserOutlined, LockOutlined, CloudOutlined } from '@ant-design/icons'
import { authApi } from '../api'
import { useAuthStore } from '../stores/auth'
import { useAppStore } from '../stores/app'

const { Title, Text } = Typography

const REMEMBER_KEY = 'cloud-manager-username'

export default function Login() {
  const [loading, setLoading] = useState(false)
  const [remember, setRemember] = useState(() => !!localStorage.getItem(REMEMBER_KEY))
  const [form] = Form.useForm()
  const navigate = useNavigate()
  const setAuth = useAuthStore((s) => s.setAuth)
  const { message } = App.useApp()

  const onFinish = async (values: { username: string; password: string }) => {
    setLoading(true)
    try {
      const resp = await authApi.login(values.username, values.password)
      setAuth(resp.token, resp.user)
      if (remember) localStorage.setItem(REMEMBER_KEY, values.username)
      else localStorage.removeItem(REMEMBER_KEY)
      useAppStore.getState().loadFromSettings()
      message.success('登录成功，正在进入控制台')
      setTimeout(() => navigate('/dashboard'), 350)
    } catch {
      // 错误已由拦截器统一提示
    } finally {
      setLoading(false)
    }
  }

  const fillDemo = () => {
    form.setFieldsValue({ username: 'admin', password: 'admin123' })
  }

  const initialUsername = localStorage.getItem(REMEMBER_KEY) || 'admin'

  return (
    <div
      style={{
        minHeight: '100vh',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        background:
          'radial-gradient(circle at 20% 20%, #4096ff 0%, transparent 45%), linear-gradient(135deg, #1677ff 0%, #0a3d91 100%)',
        padding: 16,
      }}
    >
      <Card
        style={{ width: 400, boxShadow: '0 20px 48px rgba(0,32,84,0.35)', border: 'none' }}
        styles={{ body: { padding: 36 } }}
      >
        <div style={{ textAlign: 'center', marginBottom: 28 }}>
          <div
            style={{
              width: 56,
              height: 56,
              margin: '0 auto',
              borderRadius: 14,
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              background: 'linear-gradient(135deg, #1677ff 0%, #4096ff 100%)',
              boxShadow: '0 8px 20px rgba(22,119,255,0.35)',
            }}
          >
            <CloudOutlined style={{ fontSize: 28, color: '#fff' }} />
          </div>
          <Title level={3} style={{ marginTop: 16, marginBottom: 4 }}>
            多云管理平台
          </Title>
          <Text type="secondary" style={{ fontSize: 13 }}>
            中小企业云服务器与域名统一管理
          </Text>
        </div>
        <Form
          form={form}
          layout="vertical"
          onFinish={onFinish}
          initialValues={{ username: initialUsername }}
          size="large"
        >
          <Form.Item name="username" rules={[{ required: true, message: '请输入用户名' }]}>
            <Input prefix={<UserOutlined style={{ color: '#bfbfbf' }} />} placeholder="用户名" />
          </Form.Item>
          <Form.Item name="password" rules={[{ required: true, message: '请输入密码' }]}>
            <Input.Password prefix={<LockOutlined style={{ color: '#bfbfbf' }} />} placeholder="密码" />
          </Form.Item>
          <Form.Item style={{ marginBottom: 8 }}>
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
              <Checkbox checked={remember} onChange={(e) => setRemember(e.target.checked)}>
                记住用户名
              </Checkbox>
              <Button type="link" size="small" style={{ padding: 0 }} onClick={fillDemo}>
                填入演示账号
              </Button>
            </div>
          </Form.Item>
          <Form.Item style={{ marginBottom: 0, marginTop: 8 }}>
            <Button type="primary" htmlType="submit" block loading={loading}>
              登录
            </Button>
          </Form.Item>
        </Form>
        <div style={{ marginTop: 20, textAlign: 'center' }}>
          <Text type="secondary" style={{ fontSize: 12 }}>
            默认账号：admin / admin123
          </Text>
        </div>
      </Card>
    </div>
  )
}

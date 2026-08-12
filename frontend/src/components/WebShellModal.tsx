import { useEffect, useRef, useState } from 'react'
import { Modal, Form, Input, Alert, Button, Radio, Space, Typography, App, InputNumber } from 'antd'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import '@xterm/xterm/css/xterm.css'
import type { Server } from '../types'
import { buildWebShellURL } from '../api/ws'
import { useAppStore } from '../stores/app'

const { Text } = Typography

interface WebShellModalProps {
  open: boolean
  server: Server | null
  onClose: () => void
}

interface Credentials {
  username: string
  port?: number
  authType: 'password' | 'key'
  password?: string
  privateKey?: string
  passphrase?: string
}

type ConnState = 'idle' | 'connecting' | 'connected' | 'error' | 'closed'

// WebShellModal 真实 SSH 远程终端：凭据在连接时由用户填写（平台仅保存云厂商 AccessKey，不含服务器 SSH 登录信息）。
export default function WebShellModal({ open, server, onClose }: WebShellModalProps) {
  const { message } = App.useApp()
  const [form] = Form.useForm<Credentials>()
  const { sshUser, sshPort } = useAppStore()
  const [connState, setConnState] = useState<ConnState>('idle')
  const [banner, setBanner] = useState<string>('')
  const [showForm, setShowForm] = useState(true)

  const termRef = useRef<HTMLDivElement>(null)
  const wsRef = useRef<WebSocket | null>(null)
  const termObjRef = useRef<Terminal | null>(null)
  const fitRef = useRef<FitAddon | null>(null)
  const resizeObsRef = useRef<ResizeObserver | null>(null)
  const serverIdRef = useRef<number | null>(null)
  const bufferRef = useRef<string[]>([]) // 终端就绪前缓存的远端输出
  const connStateRef = useRef<ConnState>('idle') // 避免 onerror/onclose 中 connState 闭包过期

  // 统一设置连接状态并同步 ref
  const setConn = (s: ConnState) => {
    connStateRef.current = s
    setConnState(s)
  }

  // 关闭弹窗时清理 WS 与终端实例
  const cleanup = () => {
    resizeObsRef.current?.disconnect()
    resizeObsRef.current = null
    if (wsRef.current) {
      wsRef.current.onclose = null
      wsRef.current.close()
      wsRef.current = null
    }
    if (termObjRef.current) {
      termObjRef.current.dispose()
      termObjRef.current = null
    }
    fitRef.current = null
  }

  useEffect(() => {
    if (!open) {
      cleanup()
      setConn('idle')
      setShowForm(true)
      setBanner('')
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open])

  useEffect(() => {
    return () => cleanup()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  const sendData = (data: string) => {
    const ws = wsRef.current
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify({ type: 'data', data }))
    }
  }

  const sendResize = (cols: number, rows: number) => {
    const ws = wsRef.current
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify({ type: 'resize', cols, rows }))
    }
  }

  const fitAndResize = () => {
    const term = termObjRef.current
    const fit = fitRef.current
    if (!term || !fit) return
    try {
      fit.fit()
      sendResize(term.cols, term.rows)
    } catch {
      /* 容器未渲染时忽略 */
    }
  }

  const startTerminal = () => {
    const el = termRef.current
    if (!el || termObjRef.current) return // 已初始化则跳过
    const term = new Terminal({
      cursorBlink: true,
      fontSize: 13,
      fontFamily: 'Menlo, Monaco, Consolas, "Courier New", monospace',
      theme: { background: '#1e1e1e' },
      scrollback: 2000,
    })
    const fit = new FitAddon()
    term.loadAddon(fit)
    term.open(el)
    termObjRef.current = term
    fitRef.current = fit
    fit.fit()

    term.onData((data) => sendData(data))

    // 容器尺寸变化时自动适配并通知远端
    const ro = new ResizeObserver(() => fitAndResize())
    ro.observe(el)
    resizeObsRef.current = ro

    term.focus()

    // 刷新连接建立初期缓存的输出，避免丢帧
    if (bufferRef.current.length) {
      bufferRef.current.forEach((d) => term.write(d))
      bufferRef.current = []
    }
    // 初始尺寸同步（服务端默认分配 24x80，这里按实际容器尺寸覆盖）
    setTimeout(fitAndResize, 50)
  }

  const connect = async () => {
    if (!server) return
    let creds: Credentials
    try {
      creds = await form.validateFields()
    } catch {
      return
    }
    if (!server.public_ip) {
      message.error('该服务器无公网 IP，无法建立远程终端')
      return
    }

    setConn('connecting')
    setBanner('')
    serverIdRef.current = server.id

    const url = buildWebShellURL(server.id)
    let ws: WebSocket
    try {
      ws = new WebSocket(url, 'webssh')
    } catch (e) {
      setConnState('error')
      setBanner('无法建立 WebSocket 连接：' + (e as Error).message)
      return
    }
    wsRef.current = ws

    ws.onopen = () => {
      // 首条消息必须为 auth，服务端会等待最多 15 秒
      const auth: Record<string, unknown> = {
        type: 'auth',
        username: creds.username,
        port: creds.port || 22,
      }
      if (creds.authType === 'password') {
        auth.password = creds.password || ''
      } else {
        auth.private_key = creds.privateKey || ''
        if (creds.passphrase) auth.passphrase = creds.passphrase
      }
      ws.send(JSON.stringify(auth))
      setShowForm(false)
      // 终端 div 会在 connState 变为 connected 后挂载，
      // 真正的终端初始化放在对应的 useEffect 中（此时 DOM 已就绪）。
      setConn('connected')
    }

    ws.onmessage = (ev) => {
      let msg: { type?: string; data?: string; status?: string; message?: string; code?: number }
      try {
        msg = JSON.parse(ev.data as string)
      } catch {
        return
      }
      const term = termObjRef.current
      switch (msg.type) {
        case 'status':
          if (msg.status === 'error') {
            setConn('error')
            setBanner(msg.message || '连接失败')
            term?.writeln(`\x1b[31m\r\n[连接错误] ${msg.message || '未知错误'}\x1b[0m`)
          } else if (msg.status === 'connected') {
            term?.writeln(`\x1b[32m\r\n${msg.message || '已连接'}\x1b[0m`)
          }
          break
        case 'data':
          // 终端尚未就绪时先缓存，待 startTerminal 时回放
          if (term && typeof msg.data === 'string') term.write(msg.data)
          else if (typeof msg.data === 'string') bufferRef.current.push(msg.data)
          break
        case 'exit':
          term?.writeln('\x1b[33m\r\n[会话已结束]\x1b[0m')
          setConn('closed')
          break
        default:
          break
      }
    }

    ws.onerror = () => {
      if (connStateRef.current !== 'connected') {
        setConn('error')
        setBanner('WebSocket 连接发生错误')
      }
    }

    ws.onclose = () => {
      if (connStateRef.current !== 'closed') {
        setConn('closed')
        termObjRef.current?.writeln('\x1b[33m\r\n[连接已断开]\x1b[0m')
      }
    }
  }

  // connState 变为 connected 后，终端容器已挂载，此处初始化 xterm
  useEffect(() => {
    if (connState === 'connected') {
      startTerminal()
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [connState])

  const handleClose = () => {
    cleanup()
    setConn('idle')
    setShowForm(true)
    setBanner('')
    onClose()
  }

  return (
    <Modal
      title={server ? `远程终端 · ${server.custom_name?.trim() || server.name} (${server.public_ip})` : '远程终端'}
      open={open}
      onCancel={handleClose}
      footer={
        connState === 'connected' ? (
          <Button danger onClick={handleClose}>
            断开并关闭
          </Button>
        ) : (
          <Space>
            <Button onClick={handleClose}>取消</Button>
            <Button type="primary" loading={connState === 'connecting'} onClick={connect}>
              连接
            </Button>
          </Space>
        )
      }
      width={860}
      destroyOnClose
      styles={{ body: { padding: connState === 'connected' || connState === 'closed' || connState === 'error' ? 0 : 24 } }}
    >
      {banner && (
        <Alert
          type={connState === 'connected' ? 'info' : 'error'}
          showIcon
          message={banner}
          style={{ marginBottom: 12 }}
        />
      )}

      {showForm && (
        <Form
          form={form}
          layout="vertical"
          initialValues={{ username: sshUser || 'root', port: sshPort || 22, authType: 'password' }}
        >
          <Form.Item
            label="登录用户名"
            name="username"
            rules={[{ required: true, message: '请输入 SSH 登录用户名' }]}
          >
            <Input placeholder="如 root" autoComplete="off" />
          </Form.Item>
          <Form.Item
            label="SSH 端口"
            name="port"
            rules={[{ required: true, message: '请输入 SSH 端口' }]}
          >
            <InputNumber min={1} max={65535} style={{ width: 160 }} />
          </Form.Item>
          <Form.Item label="认证方式" name="authType">
            <Radio.Group>
              <Radio value="password">密码</Radio>
              <Radio value="key">私钥</Radio>
            </Radio.Group>
          </Form.Item>
          <Form.Item noStyle shouldUpdate={(p, c) => p.authType !== c.authType}>
            {({ getFieldValue }) =>
              getFieldValue('authType') === 'password' ? (
                <Form.Item
                  label="密码"
                  name="password"
                  rules={[{ required: true, message: '请输入登录密码' }]}
                >
                  <Input.Password placeholder="SSH 登录密码" autoComplete="off" />
                </Form.Item>
              ) : (
                <>
                  <Form.Item
                    label="私钥（PEM）"
                    name="privateKey"
                    rules={[{ required: true, message: '请粘贴私钥内容' }]}
                  >
                    <Input.TextArea
                      rows={5}
                      placeholder="-----BEGIN OPENSSH PRIVATE KEY-----&#10;...&#10;-----END OPENSSH PRIVATE KEY-----"
                      style={{ fontFamily: 'monospace', fontSize: 12 }}
                    />
                  </Form.Item>
                  <Form.Item label="私钥口令（可选）" name="passphrase">
                    <Input.Password placeholder="若私钥未加密可留空" autoComplete="off" />
                  </Form.Item>
                </>
              )
            }
          </Form.Item>
          <Text type="secondary" style={{ fontSize: 12 }}>
            凭据仅用于本次建立 SSH 连接，不会被本平台保存。
          </Text>
        </Form>
      )}

      {connState === 'connecting' && (
        <div style={{ color: '#8c8c8c', padding: '40px 0', textAlign: 'center' }}>正在建立 SSH 连接…</div>
      )}

      {(connState === 'connected' || connState === 'closed' || connState === 'error') && (
        <div
          ref={termRef}
          style={{
            height: '60vh',
            background: '#1e1e1e',
            padding: 8,
            borderRadius: 4,
          }}
        />
      )}
    </Modal>
  )
}

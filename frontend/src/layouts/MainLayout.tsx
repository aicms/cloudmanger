import { useState } from 'react'
import { Outlet, useNavigate, useLocation } from 'react-router-dom'
import { Layout, Menu, Avatar, Dropdown, Space, theme, Tag, Tooltip, Button } from 'antd'
import type { MenuProps } from 'antd'
import {
  DashboardOutlined,
  CloudServerOutlined,
  GlobalOutlined,
  ApiOutlined,
  ClusterOutlined,
  HistoryOutlined,
  SettingOutlined,
  LogoutOutlined,
  UserOutlined,
  DownOutlined,
  FullscreenOutlined,
  FullscreenExitOutlined,
} from '@ant-design/icons'
import { useAuthStore } from '../stores/auth'
import { useAppStore } from '../stores/app'
import { roleName } from '../utils/format'

const { Header, Sider, Content } = Layout

interface NavItem {
  key: string
  icon: React.ReactNode
  label: string
}

const navItems: NavItem[] = [
  { key: '/dashboard', icon: <DashboardOutlined />, label: '仪表盘' },
  { key: '/servers', icon: <CloudServerOutlined />, label: '云服务器' },
  { key: '/domains', icon: <GlobalOutlined />, label: '域名管理' },
  { key: '/dns-records', icon: <ApiOutlined />, label: '域名解析' },
  { key: '/cloud-accounts', icon: <ClusterOutlined />, label: '云账号' },
  { key: '/logs', icon: <HistoryOutlined />, label: '操作日志' },
  { key: '/settings', icon: <SettingOutlined />, label: '系统设置' },
]

const menuItems: MenuProps['items'] = navItems.map((i) => ({
  key: i.key,
  icon: i.icon,
  label: i.label,
}))

export default function MainLayout() {
  const [collapsed, setCollapsed] = useState(false)
  const [isFullscreen, setIsFullscreen] = useState(false)
  const navigate = useNavigate()
  const location = useLocation()
  const { user, logout } = useAuthStore()
  const { platformName } = useAppStore()
  const { token: themeToken } = theme.useToken()

  const current =
    navItems.find((i) => location.pathname.startsWith(i.key)) || navItems[0]

  const handleLogout = () => {
    logout()
    navigate('/login')
  }

  const toggleFullscreen = () => {
    if (!document.fullscreenElement) {
      document.documentElement.requestFullscreen?.()
      setIsFullscreen(true)
    } else {
      document.exitFullscreen?.()
      setIsFullscreen(false)
    }
  }

  const userMenu: MenuProps = {
    items: [
      {
        key: 'profile',
        icon: <UserOutlined />,
        label: '个人中心',
        onClick: () => navigate('/settings'),
      },
      { type: 'divider' },
      {
        key: 'logout',
        icon: <LogoutOutlined />,
        label: '退出登录',
        onClick: handleLogout,
      },
    ],
  }

  return (
    <Layout style={{ minHeight: '100vh' }}>
      <Sider
        className="cm-sider"
        collapsible
        collapsed={collapsed}
        onCollapse={setCollapsed}
        theme="dark"
        width={224}
        style={{
          position: 'sticky',
          top: 0,
          height: '100vh',
          overflow: 'auto',
          background: 'linear-gradient(180deg, #0e2a4d 0%, #001529 70%)',
        }}
      >
        <div
          style={{
            height: 56,
            display: 'flex',
            alignItems: 'center',
            justifyContent: collapsed ? 'center' : 'flex-start',
            gap: 10,
            padding: collapsed ? 0 : '0 20px',
            color: '#fff',
            overflow: 'hidden',
            borderBottom: '1px solid rgba(255,255,255,0.06)',
          }}
        >
          <img
            src="/logo.svg"
            alt="logo"
            style={{
              width: 30,
              height: 30,
              flex: '0 0 auto',
              borderRadius: 8,
              boxShadow: '0 4px 12px rgba(22,119,255,0.4)',
            }}
          />
          {!collapsed && (
            <span style={{ fontSize: 16, fontWeight: 600, whiteSpace: 'nowrap', letterSpacing: 0.2 }}>
              {platformName || '多云管理平台'}
            </span>
          )}
        </div>
        <Menu
          theme="dark"
          mode="inline"
          selectedKeys={[current.key]}
          items={menuItems}
          onClick={({ key }) => navigate(key)}
          style={{ borderInlineEnd: 'none', paddingTop: 8 }}
        />
      </Sider>

      <Layout>
        <Header
          style={{
            padding: '0 24px',
            background: themeToken.colorBgContainer,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            borderBottom: `1px solid ${themeToken.colorBorderSecondary}`,
            position: 'sticky',
            top: 0,
            zIndex: 10,
          }}
        >
          <Space size={10}>
            <span style={{ color: themeToken.colorPrimary, fontSize: 16 }}>{current.icon}</span>
            <span style={{ fontSize: 15, fontWeight: 500, color: themeToken.colorText }}>
              {current.label}
            </span>
          </Space>
          <Space size={14}>
            <Tooltip title={isFullscreen ? '退出全屏' : '全屏'}>
              <Button
                type="text"
                shape="circle"
                aria-label="全屏切换"
                icon={isFullscreen ? <FullscreenExitOutlined /> : <FullscreenOutlined />}
                onClick={toggleFullscreen}
              />
            </Tooltip>
            <Dropdown menu={userMenu} placement="bottomRight" trigger={['click']}>
              <Space
                size={8}
                style={{
                  cursor: 'pointer',
                  padding: '4px 10px 4px 4px',
                  borderRadius: 20,
                  transition: 'background 0.2s',
                }}
                className="user-trigger"
              >
                <Avatar size={28} icon={<UserOutlined />} style={{ background: '#1677ff' }} />
                <span style={{ fontSize: 14 }}>
                  {user?.display_name || user?.username || '管理员'}
                </span>
                {user?.role && (
                  <Tag
                    color={user.role === 'admin' ? 'blue' : 'default'}
                    style={{ marginInlineEnd: 0, fontSize: 11, lineHeight: '16px' }}
                  >
                    {roleName(user.role)}
                  </Tag>
                )}
                <DownOutlined style={{ fontSize: 10, color: themeToken.colorTextTertiary }} />
              </Space>
            </Dropdown>
          </Space>
        </Header>

        <Content style={{ margin: 20 }}>
          <div className="cm-page" key={location.pathname}>
            <Outlet />
          </div>
        </Content>
      </Layout>
    </Layout>
  )
}

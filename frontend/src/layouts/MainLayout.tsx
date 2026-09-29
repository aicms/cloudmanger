import { useEffect, useState } from 'react'
import { Outlet, useNavigate, useLocation } from 'react-router-dom'
import { Layout, Menu, Avatar, Dropdown, Space, theme, Tag, Tooltip, Button, Drawer } from 'antd'
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
  MenuOutlined,
} from '@ant-design/icons'
import { useAuthStore } from '../stores/auth'
import { useAppStore } from '../stores/app'
import { roleName } from '../utils/format'

const { Header, Sider, Content } = Layout

const SIDER_BG = 'linear-gradient(180deg, #0e2a4d 0%, #001529 70%)'

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

// 监听媒体查询，初值同步取自 window，避免首屏闪烁
function useMediaQuery(query: string) {
  const [matches, setMatches] = useState(() => window.matchMedia(query).matches)
  useEffect(() => {
    const mql = window.matchMedia(query)
    const onChange = () => setMatches(mql.matches)
    onChange()
    mql.addEventListener('change', onChange)
    return () => mql.removeEventListener('change', onChange)
  }, [query])
  return matches
}

export default function MainLayout() {
  const isMobile = useMediaQuery('(max-width: 991px)') // 小屏：侧边栏改为抽屉
  const isNarrow = useMediaQuery('(max-width: 1199px)') // 中屏：侧边栏自动折叠为图标
  const [collapsed, setCollapsed] = useState(isNarrow)
  const [drawerOpen, setDrawerOpen] = useState(false)
  useEffect(() => {
    setCollapsed(isNarrow)
  }, [isNarrow])
  useEffect(() => {
    if (!isMobile) setDrawerOpen(false)
  }, [isMobile])
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

  const renderBrand = (isCollapsed: boolean) => (
    <div
      style={{
        height: 56,
        display: 'flex',
        alignItems: 'center',
        justifyContent: isCollapsed ? 'center' : 'flex-start',
        gap: 10,
        padding: isCollapsed ? 0 : '0 20px',
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
      {!isCollapsed && (
        <span style={{ fontSize: 16, fontWeight: 600, whiteSpace: 'nowrap', letterSpacing: 0.2 }}>
          {platformName || '多云管理平台'}
        </span>
      )}
    </div>
  )

  return (
    <Layout style={{ minHeight: '100vh' }}>
      {!isMobile && (
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
            background: SIDER_BG,
          }}
        >
          {renderBrand(collapsed)}
          <Menu
            theme="dark"
            mode="inline"
            selectedKeys={[current.key]}
            items={menuItems}
            onClick={({ key }) => navigate(key)}
            style={{ borderInlineEnd: 'none', paddingTop: 8 }}
          />
        </Sider>
      )}
      <Drawer
        className="cm-sider"
        placement="left"
        open={isMobile && drawerOpen}
        onClose={() => setDrawerOpen(false)}
        width={224}
        closable={false}
        styles={{ body: { padding: 0, background: SIDER_BG }, header: { display: 'none' } }}
      >
        {renderBrand(false)}
        <Menu
          theme="dark"
          mode="inline"
          selectedKeys={[current.key]}
          items={menuItems}
          onClick={({ key }) => {
            navigate(key)
            setDrawerOpen(false)
          }}
          style={{ borderInlineEnd: 'none', paddingTop: 8, background: 'transparent' }}
        />
      </Drawer>

      <Layout>
        <Header
          style={{
            padding: isMobile ? '0 12px' : '0 24px',
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
            {isMobile && (
              <Button
                type="text"
                aria-label="打开菜单"
                icon={<MenuOutlined />}
                onClick={() => setDrawerOpen(true)}
              />
            )}
            <span style={{ color: themeToken.colorPrimary, fontSize: 16 }}>{current.icon}</span>
            <span style={{ fontSize: 15, fontWeight: 500, color: themeToken.colorText }}>
              {current.label}
            </span>
          </Space>
          <Space size={isMobile ? 6 : 14}>
            {!isMobile && <Tooltip title={isFullscreen ? '退出全屏' : '全屏'}>
              <Button
                type="text"
                shape="circle"
                aria-label="全屏切换"
                icon={isFullscreen ? <FullscreenExitOutlined /> : <FullscreenOutlined />}
                onClick={toggleFullscreen}
              />
            </Tooltip>}
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
                {!isMobile && (
                  <span style={{ fontSize: 14 }}>
                    {user?.display_name || user?.username || '管理员'}
                  </span>
                )}
                {!isMobile && user?.role && (
                  <Tag
                    color={user.role === 'admin' ? 'blue' : 'default'}
                    style={{ marginInlineEnd: 0, fontSize: 11, lineHeight: '16px' }}
                  >
                    {roleName(user.role)}
                  </Tag>
                )}
                {!isMobile && <DownOutlined style={{ fontSize: 10, color: themeToken.colorTextTertiary }} />}
              </Space>
            </Dropdown>
          </Space>
        </Header>

        <Content className="cm-content">
          <div className="cm-page" key={location.pathname}>
            <Outlet />
          </div>
        </Content>
      </Layout>
    </Layout>
  )
}

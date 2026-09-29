import { type ReactNode, useEffect } from 'react'
import { ConfigProvider, theme as antdTheme } from 'antd'
import zhCN from 'antd/locale/zh_CN'
import { useThemeStore } from '../stores/theme'

const FONT_FAMILY =
  "-apple-system, BlinkMacSystemFont, 'Segoe UI', 'PingFang SC', 'Microsoft YaHei', Roboto, sans-serif"

// 与主题模式无关的通用 token / 组件配置
const baseToken = {
  colorPrimary: '#1677ff',
  borderRadius: 8,
  fontFamily: FONT_FAMILY,
  fontSize: 14,
  controlHeight: 36,
  wireframe: false,
}

const baseComponents = {
  Menu: {
    itemBorderRadius: 8,
    itemMarginInline: 10,
    itemHeight: 44,
    itemMarginBlock: 4,
    iconSize: 16,
    collapsedIconSize: 18,
    darkItemBg: 'transparent',
    darkSubMenuItemBg: 'transparent',
  },
  Card: {
    headerFontSize: 15,
    paddingLG: 20,
    borderRadiusLG: 12,
  },
  Table: {
    cellPaddingBlock: 12,
  },
  Statistic: {
    titleFontSize: 13,
  },
  Button: {
    controlHeight: 36,
    borderRadius: 8,
    fontWeight: 400,
  },
  Drawer: {
    borderRadius: 12,
  },
  Modal: {
    borderRadiusLG: 12,
  },
}

// 侧边栏在两种模式下都固定深色，其余颜色交给对应算法（defaultAlgorithm / darkAlgorithm）
// 自动生成——不要在这里写死浅色专属的十六进制颜色，否则暗色模式下会和算法算出的深色底/文字冲突
const lightExtras = {
  token: {
    colorBgLayout: '#f5f7fa',
    colorBorderSecondary: '#eef0f2',
    colorTextSecondary: '#8c8c8c',
    boxShadowSecondary: '0 6px 22px rgba(20, 40, 80, 0.08)',
  },
  components: {
    Layout: { siderBg: '#0b1f3a', headerBg: '#ffffff', headerHeight: 56, bodyBg: '#f5f7fa', headerPadding: '0 24px' },
    Table: { headerBg: '#fafafa', headerColor: 'rgba(0,0,0,0.75)', rowHoverBg: 'rgba(22,119,255,0.04)', borderColor: '#f0f0f0' },
  },
}

const darkExtras = {
  token: {
    boxShadowSecondary: '0 6px 22px rgba(0, 0, 0, 0.35)',
  },
  components: {
    Layout: { siderBg: '#0b1f3a', headerHeight: 56, headerPadding: '0 24px' },
    Table: { rowHoverBg: 'rgba(22,119,255,0.12)' },
  },
}

export default function ThemeProvider({ children }: { children: ReactNode }) {
  const mode = useThemeStore((s) => s.mode)
  const isDark = mode === 'dark'
  // 供全局 CSS（如吸底翻页条的背景色）区分明暗主题
  useEffect(() => {
    document.documentElement.dataset.theme = mode
  }, [mode])
  const algorithm = isDark ? antdTheme.darkAlgorithm : antdTheme.defaultAlgorithm
  const extras = isDark ? darkExtras : lightExtras

  const theme = {
    algorithm,
    token: { ...baseToken, ...extras.token },
    components: {
      ...baseComponents,
      Layout: extras.components.Layout,
      Table: { ...baseComponents.Table, ...extras.components.Table },
    },
  }

  return (
    <ConfigProvider locale={zhCN} theme={theme}>
      {children}
    </ConfigProvider>
  )
}

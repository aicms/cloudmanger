import React from 'react'
import ReactDOM from 'react-dom/client'
import { RouterProvider } from 'react-router-dom'
import { App as AntdApp } from 'antd'
import 'dayjs/locale/zh-cn'
import './index.css'
import { router } from './routes'
import ThemeProvider from './components/ThemeProvider'
import { useAppStore } from './stores/app'
import { useAuthStore } from './stores/auth'

// 启动即加载平台名 / SSH 默认值，供侧边栏品牌名与 WebShell 预填使用（仅在已登录时，避免未登录访问 /api/settings 触发 401 跳转循环）
if (useAuthStore.getState().token) {
  useAppStore.getState().loadFromSettings()
}

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <ThemeProvider>
      <AntdApp>
        <RouterProvider router={router} />
      </AntdApp>
    </ThemeProvider>
  </React.StrictMode>
)

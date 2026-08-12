import { Navigate } from 'react-router-dom'
import type { ReactNode } from 'react'
import { useAuthStore } from '../stores/auth'

// 路由守卫：未登录跳转登录页
export default function RequireAuth({ children }: { children: ReactNode }) {
  const token = useAuthStore((s) => s.token)
  if (!token) {
    return <Navigate to="/login" replace />
  }
  return <>{children}</>
}

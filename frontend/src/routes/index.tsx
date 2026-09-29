import { lazy, Suspense } from 'react'
import { createBrowserRouter, Navigate } from 'react-router-dom'
import { Spin } from 'antd'
import MainLayout from '../layouts/MainLayout'
import RequireAuth from './RequireAuth'

const Loading = () => (
  <div style={{ display: 'flex', justifyContent: 'center', padding: 80 }}>
    <Spin size="large" />
  </div>
)

const lazyEl = (Comp: React.LazyExoticComponent<React.ComponentType>) => (
  <Suspense fallback={<Loading />}>
    <Comp />
  </Suspense>
)

const Login = lazy(() => import('../pages/Login'))
const Dashboard = lazy(() => import('../pages/Dashboard'))
const Servers = lazy(() => import('../pages/Servers'))
const Domains = lazy(() => import('../pages/Domains'))
const DNSRecords = lazy(() => import('../pages/DNSRecords'))
const CloudAccounts = lazy(() => import('../pages/CloudAccounts'))
const Logs = lazy(() => import('../pages/Logs'))
const Settings = lazy(() => import('../pages/Settings'))

export const router = createBrowserRouter([
  {
    path: '/login',
    element: lazyEl(Login),
  },
  {
    path: '/',
    element: (
      <RequireAuth>
        <MainLayout />
      </RequireAuth>
    ),
    children: [
      { index: true, element: <Navigate to="/dashboard" replace /> },
      { path: 'dashboard', element: lazyEl(Dashboard) },
      { path: 'servers', element: lazyEl(Servers) },
      { path: 'domains', element: lazyEl(Domains) },
      { path: 'dns-records', element: lazyEl(DNSRecords) },
      { path: 'cloud-accounts', element: lazyEl(CloudAccounts) },
      { path: 'logs', element: lazyEl(Logs) },
      { path: 'settings', element: lazyEl(Settings) },
    ],
  },
])

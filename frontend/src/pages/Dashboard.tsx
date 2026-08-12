import React, { useEffect, useState, useCallback, lazy, Suspense } from 'react'
import { useNavigate } from 'react-router-dom'
import { Card, Col, Row, Statistic, Table, Tag, Skeleton, Empty, Button, theme } from 'antd'
import {
  CloudServerOutlined,
  PlayCircleOutlined,
  PauseCircleOutlined,
  GlobalOutlined,
  WarningOutlined,
  ReloadOutlined,
  ArrowRightOutlined,
} from '@ant-design/icons'
import { dashboardApi } from '../api'
import type { DashboardOverview } from '../types'
import {
  domainStatusColor,
  domainStatusText,
  daysUntil,
  providerName,
} from '../utils/format'
import { ProviderTag } from '../components/Tags'
import PageHeader from '../components/PageHeader'
import { useAuthStore } from '../stores/auth'

const ProviderChart = lazy(() => import('./DashboardCharts').then((m) => ({ default: m.ProviderChart })))
const StatusChart = lazy(() => import('./DashboardCharts').then((m) => ({ default: m.StatusChart })))

const PALETTE = ['#FF6A00', '#1677ff', '#722ed1', '#52c41a', '#fa8c16', '#13c2c2']

type StatProps = {
  title: string
  value: number
  icon: React.ReactNode
  color: string
  to: string
  suffix?: string
}

// 统计卡片：可点击跳转对应业务页
function StatCard({ title, value, icon, color, to, suffix }: StatProps) {
  const navigate = useNavigate()
  const { token } = theme.useToken()
  return (
    <Card
      className="cm-card-hover cm-clickable"
      styles={{ body: { padding: '18px 20px' } }}
      onClick={() => navigate(to)}
    >
      <div style={{ display: 'flex', alignItems: 'center', gap: 14 }}>
        <div
          className="cm-stat-icon"
          style={{
            width: 46,
            height: 46,
            flex: '0 0 auto',
            borderRadius: 12,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            fontSize: 21,
            color,
            background: `${color}14`,
          }}
        >
          {icon}
        </div>
        <div style={{ flex: 1 }}>
          <Statistic
            title={title}
            value={value}
            suffix={suffix}
            valueStyle={{ fontSize: 24, fontWeight: 600, color: token.colorText }}
          />
        </div>
        <ArrowRightOutlined style={{ color: '#bfbfbf', fontSize: 13 }} />
      </div>
    </Card>
  )
}

const statusMap: Record<string, string> = {
  Running: '运行中',
  Stopped: '已停止',
  Starting: '启动中',
  Stopping: '停止中',
}
const statusColor: Record<string, string> = {
  Running: '#52c41a',
  Stopped: '#ff4d4f',
  Starting: '#fa8c16',
  Stopping: '#fa8c16',
}

export default function Dashboard() {
  const [data, setData] = useState<DashboardOverview | null>(null)
  const [loading, setLoading] = useState(true)
  const [refreshing, setRefreshing] = useState(false)
  const navigate = useNavigate()
  const { user } = useAuthStore()
  const userName = user?.display_name || user?.username || '管理员'
  const { token } = theme.useToken()

  const load = useCallback(async (isRefresh = false) => {
    if (isRefresh) setRefreshing(true)
    else setLoading(true)
    try {
      const d = await dashboardApi.overview()
      setData(d)
    } finally {
      if (isRefresh) setRefreshing(false)
      else setLoading(false)
    }
  }, [])

  useEffect(() => {
    load()
  }, [load])

  const providerChartOpt = {
    tooltip: { trigger: 'item', formatter: '{b}: {c} ({d}%)' },
    legend: { bottom: 0, icon: 'circle', itemWidth: 8, itemHeight: 8, textStyle: { color: token.colorTextSecondary } },
    series: [
      {
        type: 'pie',
        radius: ['45%', '72%'],
        center: ['50%', '44%'],
        avoidLabelOverlap: true,
        itemStyle: { borderColor: token.colorBgContainer, borderWidth: 2, borderRadius: 6 },
        label: { show: false },
        emphasis: { label: { show: true, fontSize: 14, fontWeight: 600, formatter: '{b}\n{c}' } },
        data: Object.entries(data?.provider_dist || {}).map(([k, v]) => ({
          name: providerName(k),
          value: v,
        })),
        color: PALETTE,
      },
    ],
  }

  const statusChartOpt = {
    tooltip: { trigger: 'axis', axisPointer: { type: 'shadow' } },
    grid: { left: '3%', right: '4%', bottom: '3%', top: 16, containLabel: true },
    xAxis: {
      type: 'category',
      data: Object.keys(data?.status_dist || {}).map((k) => statusMap[k] || k),
      axisLabel: { color: token.colorTextSecondary },
      axisLine: { lineStyle: { color: token.colorBorderSecondary } },
    },
    yAxis: {
      type: 'value',
      minInterval: 1,
      splitLine: { lineStyle: { color: token.colorBorderSecondary } },
      axisLabel: { color: token.colorTextSecondary },
    },
    series: [
      {
        type: 'bar',
        data: Object.entries(data?.status_dist || {}).map(([k, v]) => ({
          value: v,
          itemStyle: { color: statusColor[k] || '#1677ff', borderRadius: [6, 6, 0, 0] },
        })),
        barWidth: '46%',
      },
    ],
  }

  return (
    <div>
      <PageHeader
        title="仪表盘"
        description={`${userName}，这是当前跨云资源的整体概览`}
        extra={
          <Button icon={<ReloadOutlined />} onClick={() => load(true)} loading={refreshing}>
            刷新
          </Button>
        }
      />

      {/* 核心指标 */}
      {!data ? (
        <Row gutter={[16, 16]}>
          {Array.from({ length: 4 }).map((_, i) => (
            <Col xs={12} sm={12} md={6} key={i}>
              <Card styles={{ body: { padding: 20 } }}>
                <Skeleton active paragraph={{ rows: 1 }} title={{ width: '60%' }} />
              </Card>
            </Col>
          ))}
        </Row>
      ) : (
        <Row gutter={[16, 16]}>
          <Col xs={12} sm={12} md={6}>
            <StatCard title="云服务器总数" value={data.server_total} icon={<CloudServerOutlined />} color="#1677ff" to="/servers" />
          </Col>
          <Col xs={12} sm={12} md={6}>
            <StatCard title="运行中" value={data.server_running} icon={<PlayCircleOutlined />} color="#52c41a" to="/servers?status=Running" />
          </Col>
          <Col xs={12} sm={12} md={6}>
            <StatCard title="已停止" value={data.server_stopped} icon={<PauseCircleOutlined />} color="#ff4d4f" to="/servers?status=Stopped" />
          </Col>
          <Col xs={12} sm={12} md={6}>
            <StatCard title="域名总数" value={data.domain_total} icon={<GlobalOutlined />} color="#722ed1" to="/domains" />
          </Col>
        </Row>
      )}

      {/* 资源分布（合并） + 到期提醒（右侧） */}
      <Row gutter={[16, 16]} style={{ marginTop: 16 }}>
        <Col xs={24} lg={16}>
          <Card title="资源分布">
            <Row gutter={[16, 16]}>
              <Col xs={24} sm={12}>
                <div style={{ fontSize: 13, color: token.colorTextSecondary, marginBottom: 4 }}>云厂商</div>
                {!data ? (
                  <Skeleton active paragraph={{ rows: 6 }} />
                ) : (
                  <Suspense fallback={<Skeleton active paragraph={{ rows: 6 }} />}>
                    <ProviderChart option={providerChartOpt} />
                  </Suspense>
                )}
              </Col>
              <Col xs={24} sm={12}>
                <div style={{ fontSize: 13, color: token.colorTextSecondary, marginBottom: 4 }}>服务器状态</div>
                {!data ? (
                  <Skeleton active paragraph={{ rows: 6 }} />
                ) : (
                  <Suspense fallback={<Skeleton active paragraph={{ rows: 6 }} />}>
                    <StatusChart option={statusChartOpt} />
                  </Suspense>
                )}
              </Col>
            </Row>
          </Card>
        </Col>
        <Col xs={24} lg={8}>
          <Card
            style={{ height: '100%' }}
            styles={{ body: { padding: 12, maxHeight: 320, overflow: 'auto' } }}
            title={
              <span>
                <WarningOutlined style={{ color: '#faad14', marginRight: 8 }} />
                域名到期提醒
              </span>
            }
          >
            {!data ? (
              <Skeleton active paragraph={{ rows: 4 }} />
            ) : (
              <Table
                size="small"
                rowKey="id"
                dataSource={data.expiring_domains}
                pagination={false}
                scroll={{ x: 300 }}
                locale={{ emptyText: <Empty description="近 30 天无即将到期" /> }}
                columns={[
                  {
                    title: '域名',
                    dataIndex: 'domain_name',
                    render: (v: string) => (
                      <a onClick={() => navigate('/domains')}>
                        <strong>{v}</strong>
                      </a>
                    ),
                  },
                  {
                    title: '剩余',
                    dataIndex: 'expiry_date',
                    width: 84,
                    render: (d: string) => {
                      const days = daysUntil(d)
                      return (
                        <Tag color={days <= 7 ? 'red' : 'orange'}>{days > 0 ? `${days} 天` : '已过期'}</Tag>
                      )
                    },
                  },
                ]}
              />
            )}
          </Card>
        </Col>
      </Row>
    </div>
  )
}

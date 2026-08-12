import ReactEChartsCore from 'echarts-for-react/lib/core'
import * as echarts from 'echarts/core'
import { PieChart, BarChart } from 'echarts/charts'
import { TooltipComponent, LegendComponent, GridComponent } from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'

echarts.use([PieChart, BarChart, TooltipComponent, LegendComponent, GridComponent, CanvasRenderer])

export function ProviderChart({ option }: { option: Record<string, unknown> }) {
  return <ReactEChartsCore echarts={echarts} option={option} style={{ height: 240 }} notMerge />
}

export function StatusChart({ option }: { option: Record<string, unknown> }) {
  return <ReactEChartsCore echarts={echarts} option={option} style={{ height: 240 }} notMerge />
}

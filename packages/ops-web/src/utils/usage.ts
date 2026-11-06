import type { EChartsOption } from 'echarts'

import type { UsageDailyPoint } from '@/types'

const BLUE = '#2563eb'
const BLACK = '#111111'
const GRAY = '#6b7280'
const LIGHT_GRAY = '#d1d5db'

const baseAxis = {
  axisLine: { lineStyle: { color: LIGHT_GRAY } },
  axisLabel: { color: GRAY, fontSize: 11 },
  splitLine: { lineStyle: { color: '#e5e7eb' } },
}

function formatDate(date: Date): string {
  const year = date.getUTCFullYear()
  const month = `${date.getUTCMonth() + 1}`.padStart(2, '0')
  const day = `${date.getUTCDate()}`.padStart(2, '0')
  return `${year}-${month}-${day}`
}

export function dateRange(start: Date, end: Date): string[] {
  const dates: string[] = []
  const current = new Date(
    Date.UTC(start.getUTCFullYear(), start.getUTCMonth(), start.getUTCDate()),
  )
  const last = new Date(Date.UTC(end.getUTCFullYear(), end.getUTCMonth(), end.getUTCDate()))
  while (current.getTime() <= last.getTime()) {
    dates.push(formatDate(current))
    current.setUTCDate(current.getUTCDate() + 1)
  }
  return dates
}

export function dateRangeForDays(days: number, now = new Date()): string[] {
  const end = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), now.getUTCDate()))
  const start = new Date(end)
  start.setUTCDate(start.getUTCDate() - Math.max(days - 1, 0))
  return dateRange(start, end)
}

export function fillUsageSeries(rows: UsageDailyPoint[], dates: string[]): UsageDailyPoint[] {
  const byDate = new Map<string, UsageDailyPoint>()
  for (const row of rows) {
    const current = byDate.get(row.date)
    if (current) {
      current.projectsCreated += row.projectsCreated
      current.inputTokens += row.inputTokens
      current.outputTokens += row.outputTokens
      current.modelRequests += row.modelRequests
      current.uploadCount += row.uploadCount
      current.uploadBytes += row.uploadBytes
      continue
    }
    byDate.set(row.date, { ...row })
  }
  return dates.map(
    (date) =>
      byDate.get(date) ?? {
        date,
        projectsCreated: 0,
        inputTokens: 0,
        outputTokens: 0,
        modelRequests: 0,
        uploadCount: 0,
        uploadBytes: 0,
      },
  )
}

function common(points: UsageDailyPoint[]): EChartsOption {
  return {
    grid: { left: 44, right: 20, top: 36, bottom: 28 },
    tooltip: { trigger: 'axis' },
    xAxis: { type: 'category', data: points.map((item) => item.date), ...baseAxis },
    animation: false,
  }
}

export function buildProjectsOption(points: UsageDailyPoint[]): EChartsOption {
  return {
    ...common(points),
    yAxis: { type: 'value', minInterval: 1, ...baseAxis },
    series: [
      {
        name: '创建项目',
        type: 'line',
        smooth: true,
        symbolSize: 6,
        data: points.map((item) => item.projectsCreated),
        itemStyle: { color: BLUE },
        areaStyle: { color: 'rgba(37, 99, 235, 0.08)' },
      },
    ],
  }
}

export function buildTokensOption(points: UsageDailyPoint[]): EChartsOption {
  return {
    ...common(points),
    legend: { data: ['输入 Token', '输出 Token'], textStyle: { color: GRAY }, top: 4 },
    yAxis: { type: 'value', ...baseAxis },
    series: [
      {
        name: '输入 Token',
        type: 'line',
        smooth: true,
        data: points.map((item) => item.inputTokens),
        itemStyle: { color: BLUE },
      },
      {
        name: '输出 Token',
        type: 'line',
        smooth: true,
        data: points.map((item) => item.outputTokens),
        itemStyle: { color: BLACK },
      },
    ],
  }
}

export function buildRequestsOption(points: UsageDailyPoint[]): EChartsOption {
  return {
    ...common(points),
    yAxis: { type: 'value', minInterval: 1, ...baseAxis },
    series: [
      {
        name: '模型请求',
        type: 'bar',
        data: points.map((item) => item.modelRequests),
        itemStyle: { color: BLUE },
      },
    ],
  }
}

export function buildUploadsOption(points: UsageDailyPoint[]): EChartsOption {
  return {
    ...common(points),
    legend: { data: ['上传文件数', '上传字节'], textStyle: { color: GRAY }, top: 4 },
    yAxis: [
      { type: 'value', minInterval: 1, ...baseAxis },
      { type: 'value', ...baseAxis, splitLine: { show: false } },
    ],
    series: [
      {
        name: '上传文件数',
        type: 'bar',
        data: points.map((item) => item.uploadCount),
        itemStyle: { color: BLUE },
      },
      {
        name: '上传字节',
        type: 'line',
        yAxisIndex: 1,
        smooth: true,
        data: points.map((item) => item.uploadBytes),
        itemStyle: { color: BLACK },
      },
    ],
  }
}

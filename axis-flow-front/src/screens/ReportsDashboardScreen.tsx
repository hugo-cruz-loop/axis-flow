import { useSearchParams } from 'react-router-dom'
import { IncidentChartWidget } from '@/components/reports/IncidentChartWidget'
import { useIncidentMetrics, useIncidentWeeklyTrend } from '@/hooks/use-reports'
import { incidentDashboardQuerySchema } from '@/schemas/reports-schemas'

const GROUP_BY_OPTIONS = [
  { value: 'day', label: 'Day' },
  { value: 'week', label: 'Week' },
  { value: 'month', label: 'Month' },
] as const

export function ReportsDashboardScreen() {
  const [searchParams, setSearchParams] = useSearchParams()

  const rawParams = {
    clientId: searchParams.get('clientId') ?? undefined,
    groupBy: searchParams.get('groupBy') ?? undefined,
  }

  const parseResult = incidentDashboardQuerySchema.safeParse(rawParams)
  const params = parseResult.success ? parseResult.data : { groupBy: 'week' as const }

  const metricsQuery = useIncidentMetrics(params)
  const trendQuery = useIncidentWeeklyTrend(params)

  const isLoading = metricsQuery.isLoading || trendQuery.isLoading
  const metrics = metricsQuery.data?.data ?? []
  const trends = trendQuery.data?.data ?? []

  function setGroupBy(value: 'day' | 'week' | 'month') {
    const next = new URLSearchParams(searchParams)
    next.set('groupBy', value)
    setSearchParams(next)
  }

  return (
    <main className="p-6 space-y-6">
      <h1 className="text-2xl font-bold text-slate-800">Reports Dashboard</h1>

      {/* Filters */}
      <div className="flex items-center gap-4 bg-slate-50 p-4 rounded-lg border border-slate-200">
        <div className="flex flex-col gap-1">
          <label htmlFor="groupby-select" className="text-xs font-medium text-slate-600">
            Group by
          </label>
          <select
            id="groupby-select"
            className="border border-slate-300 rounded px-2 py-1 text-sm focus-visible:ring-2 focus-visible:ring-blue-500"
            value={params.groupBy}
            onChange={(e) => setGroupBy(e.target.value as 'day' | 'week' | 'month')}
          >
            {GROUP_BY_OPTIONS.map((opt) => (
              <option key={opt.value} value={opt.value}>
                {opt.label}
              </option>
            ))}
          </select>
        </div>
      </div>

      <div className="bg-white rounded-lg border border-slate-200 p-6">
        <IncidentChartWidget
          metrics={metrics}
          trends={trends}
          isLoading={isLoading}
        />
      </div>
    </main>
  )
}

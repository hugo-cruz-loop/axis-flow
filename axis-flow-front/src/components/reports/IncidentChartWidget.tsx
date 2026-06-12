import type { GraficaStat, IncidenteCount } from '@/api/reports-service'

interface IncidentChartWidgetProps {
  metrics: IncidenteCount[]
  trends: GraficaStat[]
  isLoading: boolean
}

// ─── Colors and patterns ──────────────────────────────────────────────────────

// Dual coding: distinct colors (contrast >= 3:1 on white) + fill patterns
const METRIC_CONFIG: Record<string, { color: string; patternId: string; label: string }> = {
  Pending: {
    color: '#b45309',   // amber-700 — contrast 4.6:1 on white
    patternId: 'pattern-pending',
    label: 'Pending',
  },
  InProgress: {
    color: '#1d4ed8',   // blue-700 — contrast 5.9:1 on white
    patternId: 'pattern-inprogress',
    label: 'In Progress',
  },
  Resolved: {
    color: '#15803d',   // green-700 — contrast 4.5:1 on white
    patternId: 'pattern-resolved',
    label: 'Resolved',
  },
}

function getFallbackConfig(status: string, index: number) {
  const fallbackColors = ['#7c3aed', '#0e7490', '#be185d']
  const fallbackPatternIds = ['pattern-a', 'pattern-b', 'pattern-c']
  return {
    color: fallbackColors[index % fallbackColors.length],
    patternId: fallbackPatternIds[index % fallbackPatternIds.length],
    label: status,
  }
}

// ─── SVG patterns (dual coding) ───────────────────────────────────────────────

function SvgPatterns() {
  return (
    <defs>
      {/* Diagonal lines — Pending */}
      <pattern id="pattern-pending" width="6" height="6" patternUnits="userSpaceOnUse" patternTransform="rotate(45)">
        <line x1="0" y1="0" x2="0" y2="6" stroke="#b45309" strokeWidth="2" />
      </pattern>
      {/* Dots — InProgress */}
      <pattern id="pattern-inprogress" width="6" height="6" patternUnits="userSpaceOnUse">
        <circle cx="3" cy="3" r="1.5" fill="#1d4ed8" />
      </pattern>
      {/* Horizontal lines — Resolved */}
      <pattern id="pattern-resolved" width="6" height="6" patternUnits="userSpaceOnUse">
        <line x1="0" y1="3" x2="6" y2="3" stroke="#15803d" strokeWidth="2" />
      </pattern>
      {/* Fallback patterns */}
      <pattern id="pattern-a" width="6" height="6" patternUnits="userSpaceOnUse" patternTransform="rotate(30)">
        <line x1="0" y1="0" x2="0" y2="6" stroke="#7c3aed" strokeWidth="2" />
      </pattern>
      <pattern id="pattern-b" width="6" height="6" patternUnits="userSpaceOnUse">
        <rect x="2" y="2" width="2" height="2" fill="#0e7490" />
      </pattern>
      <pattern id="pattern-c" width="6" height="6" patternUnits="userSpaceOnUse">
        <line x1="0" y1="3" x2="6" y2="3" stroke="#be185d" strokeWidth="2" />
      </pattern>
    </defs>
  )
}

// ─── Bar chart ────────────────────────────────────────────────────────────────

function MetricsBarChart({ metrics }: { metrics: IncidenteCount[] }) {
  const maxCount = Math.max(...metrics.map((m) => m.count), 1)
  const chartWidth = 300
  const chartHeight = 160
  const barWidth = Math.min(60, (chartWidth - 40) / Math.max(metrics.length, 1) - 10)
  const gap = (chartWidth - 20 - barWidth * metrics.length) / Math.max(metrics.length + 1, 1)

  return (
    <svg
      viewBox={`0 0 ${chartWidth} ${chartHeight + 40}`}
      className="w-full max-w-sm"
      role="img"
      aria-label="Incident metrics bar chart"
    >
      <SvgPatterns />

      {/* Y-axis grid lines */}
      {[0, 0.25, 0.5, 0.75, 1].map((ratio) => {
        const y = 10 + chartHeight * (1 - ratio)
        return (
          <line
            key={ratio}
            x1="20"
            y1={y}
            x2={chartWidth - 10}
            y2={y}
            stroke="#e2e8f0"
            strokeWidth="1"
          />
        )
      })}

      {/* Bars */}
      {metrics.map((metric, i) => {
        const config =
          METRIC_CONFIG[metric.status] ?? getFallbackConfig(metric.status, i)
        const barHeight = (metric.count / maxCount) * chartHeight
        const x = 20 + gap + i * (barWidth + gap)
        const y = 10 + chartHeight - barHeight

        return (
          <g key={metric.status}>
            {/* Solid color fill */}
            <rect
              x={x}
              y={y}
              width={barWidth}
              height={barHeight}
              fill={config.color}
              opacity={0.3}
            />
            {/* Pattern overlay for dual coding */}
            <rect
              x={x}
              y={y}
              width={barWidth}
              height={barHeight}
              fill={`url(#${config.patternId})`}
              opacity={0.7}
            />
            {/* Count label above bar */}
            <text
              x={x + barWidth / 2}
              y={y - 4}
              textAnchor="middle"
              fontSize="11"
              fill={config.color}
              fontWeight="600"
            >
              {metric.count}
            </text>
            {/* Status label below bar */}
            <text
              x={x + barWidth / 2}
              y={10 + chartHeight + 16}
              textAnchor="middle"
              fontSize="9"
              fill="#475569"
            >
              {config.label}
            </text>
          </g>
        )
      })}
    </svg>
  )
}

// ─── Skeleton ─────────────────────────────────────────────────────────────────

function Skeleton() {
  return (
    <div className="animate-pulse space-y-4" aria-hidden="true">
      <div className="h-6 w-40 bg-slate-200 rounded" />
      <div className="flex items-end gap-4 h-40">
        {Array.from({ length: 3 }).map((_, i) => (
          <div key={i} className="flex-1 bg-slate-200 rounded" style={{ height: `${(i + 1) * 33}%` }} />
        ))}
      </div>
    </div>
  )
}

// ─── Main component ───────────────────────────────────────────────────────────

export function IncidentChartWidget({ metrics, trends, isLoading }: IncidentChartWidgetProps) {
  if (isLoading) {
    return <Skeleton />
  }

  return (
    <div className="space-y-6">
      {/* Metrics bar chart */}
      <div>
        <h3 className="text-sm font-semibold text-slate-700 mb-3">Incidents by Status</h3>
        {metrics.length > 0 ? (
          <MetricsBarChart metrics={metrics} />
        ) : (
          <p className="text-sm text-slate-500">No incident data available.</p>
        )}
      </div>

      {/* Weekly trend table */}
      {trends.length > 0 && (
        <div>
          <h3 className="text-sm font-semibold text-slate-700 mb-2">Weekly Evidence Trend</h3>
          <div className="overflow-x-auto">
            <table className="w-full text-xs text-left">
              <thead>
                <tr className="border-b border-slate-200">
                  <th scope="col" className="py-2 pr-4 font-semibold text-slate-600">Week</th>
                  <th scope="col" className="py-2 pr-4 font-semibold text-slate-600">Total</th>
                  <th scope="col" className="py-2 font-semibold text-slate-600">Compliant</th>
                </tr>
              </thead>
              <tbody>
                {trends.map((row) => (
                  <tr key={row.week} className="border-b border-slate-100">
                    <td className="py-1.5 pr-4 text-slate-700">{row.week}</td>
                    <td className="py-1.5 pr-4 text-slate-700">{row.total}</td>
                    <td className="py-1.5 text-slate-700">{row.compliant}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {/* Screen-reader backing table for chart (aria-hidden=false makes it accessible) */}
      <table className="sr-only" aria-label="Incident metrics data table" aria-hidden={false}>
        <thead>
          <tr>
            <th scope="col">Status</th>
            <th scope="col">Count</th>
          </tr>
        </thead>
        <tbody>
          {metrics.map((m) => (
            <tr key={m.status}>
              <td>{METRIC_CONFIG[m.status]?.label ?? m.status}</td>
              <td>{m.count}</td>
            </tr>
          ))}
        </tbody>
      </table>

      {/* Legend */}
      {metrics.length > 0 && (
        <div className="flex flex-wrap gap-3" role="list" aria-label="Chart legend">
          {metrics.map((m, i) => {
            const config = METRIC_CONFIG[m.status] ?? getFallbackConfig(m.status, i)
            return (
              <div key={m.status} className="flex items-center gap-1.5" role="listitem">
                <span
                  className="inline-block w-3 h-3 rounded-sm border"
                  style={{ backgroundColor: config.color, borderColor: config.color }}
                  aria-hidden="true"
                />
                <span className="text-xs text-slate-600">{config.label}</span>
              </div>
            )
          })}
        </div>
      )}
    </div>
  )
}

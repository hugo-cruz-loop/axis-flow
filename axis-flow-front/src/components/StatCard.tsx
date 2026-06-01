import { cn } from '@/lib/utils'

interface StatCardProps {
  icon: React.ElementType
  iconBg: string
  iconColor: string
  label: string
  value: number | string
  trend?: string
  trendPositive?: boolean
  loading?: boolean
}

export function StatCard({
  icon: Icon,
  iconBg,
  iconColor,
  label,
  value,
  trend,
  trendPositive,
  loading = false,
}: StatCardProps) {
  return (
    <div className="rounded-xl border border-slate-200 bg-white p-5 shadow-sm">
      <div className="flex items-start justify-between">
        <div className={cn('flex h-10 w-10 items-center justify-center rounded-full', iconBg)}>
          <Icon className={cn('h-5 w-5', iconColor)} />
        </div>
        {trend && (
          <span
            className={cn(
              'rounded-full px-2 py-0.5 text-xs font-medium',
              trendPositive
                ? 'bg-green-100 text-green-700'
                : 'bg-red-100 text-red-700',
            )}
          >
            {trend}
          </span>
        )}
      </div>
      <div className="mt-3">
        {loading ? (
          <div className="h-8 w-16 animate-pulse rounded bg-slate-200" />
        ) : (
          <p className="text-2xl font-bold text-slate-900">{value}</p>
        )}
        <p className="mt-0.5 text-sm text-slate-500">{label}</p>
      </div>
    </div>
  )
}

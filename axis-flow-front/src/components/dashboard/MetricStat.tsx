import React from 'react'

interface MetricStatProps {
  value: number
  label: string
  isPercentage?: boolean
}

export const MetricStat: React.FC<MetricStatProps> = ({
  value,
  label,
  isPercentage = false,
}) => {
  const formattedValue = isPercentage ? `${value}%` : value.toLocaleString()

  return (
    <div className="mt-3">
      <p className="text-3xl font-bold text-slate-900 dark:text-slate-100" data-testid="metric-value">
        {formattedValue}
      </p>
      <p className="mt-1 text-sm text-slate-500 dark:text-slate-400">
        {label}
      </p>
    </div>
  )
}

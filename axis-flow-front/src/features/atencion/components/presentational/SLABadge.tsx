interface SLABadgeProps {
  fechaVigencia: string
  estatus: number
}

export function SLABadge({ fechaVigencia, estatus }: SLABadgeProps) {
  // Only show SLA badge for open statuses (Pendiente=1, En Proceso=2)
  if (estatus === 3) return null

  const now = new Date()
  const deadline = new Date(fechaVigencia)
  const diffMs = deadline.getTime() - now.getTime()
  const daysLeft = Math.floor(diffMs / (1000 * 60 * 60 * 24))

  const isOverdue = diffMs < 0

  const colorClass = isOverdue
    ? 'bg-red-100 text-red-800'
    : daysLeft <= 3
      ? 'bg-yellow-100 text-yellow-800'
      : 'bg-green-100 text-green-800'

  const label = isOverdue ? 'overdue' : `${daysLeft}d left`

  return (
    <span
      className={`inline-block rounded-full px-2 py-0.5 text-xs font-medium ${colorClass}`}
      aria-label={`SLA: ${label}`}
    >
      {label}
    </span>
  )
}

import type { UserStatus } from '@/types/users'

const statusStyles: Record<UserStatus, string> = {
  ACTIVE: 'bg-green-100 text-green-800',
  PENDING_ACTIVATION: 'bg-amber-100 text-amber-800',
  INACTIVE: 'bg-gray-100 text-gray-800',
  SUSPENDED: 'bg-orange-100 text-orange-800',
  LOCKED: 'bg-red-100 text-red-800',
  DELETED: 'bg-red-100 text-red-800',
}

const statusLabels: Record<UserStatus, string> = {
  ACTIVE: 'Active',
  PENDING_ACTIVATION: 'Pending Activation',
  INACTIVE: 'Inactive',
  SUSPENDED: 'Suspended',
  LOCKED: 'Locked',
  DELETED: 'Deleted',
}

interface StatusBadgeProps {
  status: UserStatus
}

export function StatusBadge({ status }: StatusBadgeProps) {
  return (
    <span
      data-testid="status-badge"
      className={`inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium ${statusStyles[status]}`}
    >
      {/* Text label is always rendered — color alone is not sufficient for accessibility */}
      {statusLabels[status]}
    </span>
  )
}

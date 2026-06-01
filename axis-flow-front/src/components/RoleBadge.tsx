import type { UserRole } from '@/types/users'

const roleColors: Record<UserRole, string> = {
  ADMIN_CHECK_ON: 'bg-purple-100 text-purple-800',
  ADMINISTRADOR: 'bg-purple-100 text-purple-800',
  CLIENTE: 'bg-blue-100 text-blue-800',
  EMPLEADO: 'bg-gray-100 text-gray-800',
  SUPERVISOR: 'bg-indigo-100 text-indigo-800',
  OPERACIONES: 'bg-orange-100 text-orange-800',
  GESTOR: 'bg-teal-100 text-teal-800',
  RH: 'bg-pink-100 text-pink-800',
}

interface RoleBadgeProps {
  role: UserRole
}

export function RoleBadge({ role }: RoleBadgeProps) {
  return (
    <span
      data-testid="role-badge"
      className={`inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium ${roleColors[role]}`}
    >
      {role}
    </span>
  )
}

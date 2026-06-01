import { useAuthStore } from '@/store/authStore'
import { PermissionDeniedState } from './PermissionDeniedState'
import type { UserRole } from '@/types/users'

interface RoleGateProps {
  allowedRoles: UserRole[]
  children: React.ReactNode
}

export function RoleGate({ allowedRoles, children }: RoleGateProps) {
  const user = useAuthStore((s) => s.user)

  if (!user || !allowedRoles.includes(user.role)) {
    return <PermissionDeniedState />
  }

  return <>{children}</>
}

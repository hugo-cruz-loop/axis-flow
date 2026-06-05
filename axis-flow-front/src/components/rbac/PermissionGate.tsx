import { useHasPermission } from '@/hooks/usePermissions'

interface PermissionGateProps {
  permission: string
  children: React.ReactNode
  fallback?: React.ReactNode
}

export function PermissionGate({ permission, children, fallback = null }: PermissionGateProps) {
  return useHasPermission(permission) ? <>{children}</> : <>{fallback}</>
}

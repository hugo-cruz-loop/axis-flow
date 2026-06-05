import { useAuthStore } from '@/store/authStore'

export function userHasPermission(
  permissions: string[] | undefined,
  permission: string,
): boolean {
  return permissions?.includes(permission) ?? false
}

export function useHasPermission(permission: string): boolean {
  const permissions = useAuthStore((s) => s.user?.permissions)
  return userHasPermission(permissions, permission)
}

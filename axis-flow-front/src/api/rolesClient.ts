import axiosInstance from './axiosInstance'
import type { RoleWithCount, Permission, CreateRoleRequest, UpdateRoleRequest } from '@/types/rbac'

// Re-export types for backward compatibility
export type { RoleWithCount, Permission }

export async function listRoles(): Promise<RoleWithCount[]> {
  const response = await axiosInstance.get<RoleWithCount[]>('/roles/')
  return response.data
}

export async function listRolePermissions(roleCode: string): Promise<Permission[]> {
  const response = await axiosInstance.get<Permission[]>(`/roles/${roleCode}/permissions`)
  return response.data
}

// ── New v1 CRUD functions ─────────────────────────────────────────────────────

export async function createRole(req: CreateRoleRequest): Promise<RoleWithCount> {
  const response = await axiosInstance.post<RoleWithCount>('/v1/roles', req)
  return response.data
}

export async function updateRole(id: string, req: UpdateRoleRequest): Promise<RoleWithCount> {
  const response = await axiosInstance.put<RoleWithCount>(`/v1/roles/${id}`, req)
  return response.data
}

export async function deleteRole(id: string): Promise<void> {
  await axiosInstance.delete(`/v1/roles/${id}`)
}

export async function listV1Roles(): Promise<RoleWithCount[]> {
  const response = await axiosInstance.get<RoleWithCount[]>('/v1/roles')
  return response.data
}

export async function listRolePermissionsById(roleId: string): Promise<Permission[]> {
  const response = await axiosInstance.get<Permission[]>(`/v1/roles/${roleId}/permissions`)
  return response.data
}

export async function assignPermissionToRole(roleId: string, permissionId: string): Promise<void> {
  await axiosInstance.post(`/v1/roles/${roleId}/permissions`, { permission_id: permissionId })
}

export async function revokePermissionFromRole(roleId: string, permissionId: string): Promise<void> {
  await axiosInstance.delete(`/v1/roles/${roleId}/permissions/${permissionId}`)
}

export async function assignRoleToUser(userId: string, roleId: string): Promise<void> {
  await axiosInstance.post(`/v1/users/${userId}/roles`, { role_id: roleId })
}

export async function revokeRoleFromUser(userId: string, roleId: string): Promise<void> {
  await axiosInstance.delete(`/v1/users/${userId}/roles/${roleId}`)
}

import axiosInstance from './axiosInstance'
import type { Permission } from '@/types/rbac'

export async function listPermissions(module?: string): Promise<Permission[]> {
  const params = module ? { module } : {}
  const response = await axiosInstance.get<Permission[]>('/v1/permissions', { params })
  return response.data
}

export async function createPermission(req: {
  code: string
  name: string
  module: string
  description?: string
}): Promise<Permission> {
  const response = await axiosInstance.post<Permission>('/v1/permissions', req)
  return response.data
}

export async function updatePermission(
  id: string,
  req: { name: string; description?: string },
): Promise<Permission> {
  const response = await axiosInstance.put<Permission>(`/v1/permissions/${id}`, req)
  return response.data
}

export async function deletePermission(id: string): Promise<void> {
  await axiosInstance.delete(`/v1/permissions/${id}`)
}

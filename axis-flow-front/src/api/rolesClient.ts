import axiosInstance from './axiosInstance'

export interface RoleWithCount {
  id: string
  code: string
  name: string
  scope: 'GLOBAL' | 'TENANT'
  is_system: boolean
  permission_count: number
}

export interface Permission {
  id: string
  code: string
  name: string
  module: string
  description: string
}

export async function listRoles(): Promise<RoleWithCount[]> {
  const response = await axiosInstance.get<RoleWithCount[]>('/roles/')
  return response.data
}

export async function listRolePermissions(roleCode: string): Promise<Permission[]> {
  const response = await axiosInstance.get<Permission[]>(`/roles/${roleCode}/permissions`)
  return response.data
}

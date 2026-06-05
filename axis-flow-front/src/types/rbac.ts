export type RoleScope = 'GLOBAL' | 'TENANT'

export interface RoleWithCount {
  id: string
  code: string
  name: string
  description?: string
  scope: RoleScope
  is_system: boolean
  permission_count: number
}

export interface Permission {
  id: string
  code: string
  name: string
  module: string
  description?: string
}

export interface CreateRoleRequest {
  code: string
  name: string
  description?: string
  scope: RoleScope
}

export interface UpdateRoleRequest {
  name: string
  description?: string
}

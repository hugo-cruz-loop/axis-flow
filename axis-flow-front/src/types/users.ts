export type UserStatus =
  | 'PENDING_ACTIVATION'
  | 'ACTIVE'
  | 'INACTIVE'
  | 'SUSPENDED'
  | 'LOCKED'
  | 'DELETED'

// Role codes match identity_roles.code in the database and are embedded in the JWT.
export type UserRole =
  | 'ADMIN_CHECK_ON'
  | 'CLIENTE'
  | 'EMPLEADO'
  | 'SUPERVISOR'
  | 'OPERACIONES'
  | 'GESTOR'
  | 'ADMINISTRADOR'
  | 'RH'

export interface User {
  id: string
  email: string
  first_name: string
  last_name: string
  role: UserRole
  permissions: string[]
  status: UserStatus
}

export interface LoginResponse {
  access_token: string
  refresh_token: string
  user_id: string
  role: UserRole
}

export interface CreateUserRequest {
  email: string
  first_name: string
  last_name: string
  role_code: UserRole
  url_front?: string
}

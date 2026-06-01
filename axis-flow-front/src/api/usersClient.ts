import axiosInstance from './axiosInstance'
import type { CreateUserRequest, User, UserRole } from '@/types/users'

export async function createUser(req: CreateUserRequest): Promise<User> {
  const response = await axiosInstance.post<User>('/users/', req)
  return response.data
}

// Backend only exposes /api/users/filtradoUser/{role}/ — there is no GET /api/users/.
// To list all users call once per role and merge, or use a specific role filter.
// For the dashboard overview we default to fetching all known roles in parallel.
export async function listByRole(role: UserRole): Promise<User[]> {
  const response = await axiosInstance.get<User[]>(`/users/filtradoUser/${role}/`)
  return response.data
}

export const ALL_ROLES: UserRole[] = [
  'ADMIN_CHECK_ON',
  'CLIENTE',
  'EMPLEADO',
  'SUPERVISOR',
  'OPERACIONES',
  'GESTOR',
  'ADMINISTRADOR',
  'RH',
]

/** Fetch all users by querying every role and deduplicating by id. */
export async function listAllUsers(): Promise<User[]> {
  const results = await Promise.allSettled(
    ALL_ROLES.map((role) => listByRole(role)),
  )
  const seen = new Set<string>()
  const users: User[] = []
  for (const r of results) {
    if (r.status === 'fulfilled') {
      for (const u of r.value) {
        if (!seen.has(u.id)) {
          seen.add(u.id)
          users.push(u)
        }
      }
    }
  }
  return users
}

export async function resetPassword(
  token: string,
  password: string,
): Promise<void> {
  await axiosInstance.patch('/users/reset-pass/', { token, password })
}

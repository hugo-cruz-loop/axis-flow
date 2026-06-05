import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor, fireEvent } from '@testing-library/react'
import { MemoryRouter, Routes, Route } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { RolesPage } from '../RolesPage'
import { useAuthStore } from '@/store/authStore'
import type { User } from '@/types/users'

vi.mock('@/api/rolesClient', () => ({
  listV1Roles: vi.fn(),
  deleteRole: vi.fn(),
  createRole: vi.fn(),
  updateRole: vi.fn(),
  listRolePermissionsById: vi.fn(),
  assignPermissionToRole: vi.fn(),
  revokePermissionFromRole: vi.fn(),
  // keep legacy exports
  listRoles: vi.fn(),
  listRolePermissions: vi.fn(),
}))

vi.mock('@/api/permissionsClient', () => ({
  listPermissions: vi.fn(),
}))

import * as rolesClient from '@/api/rolesClient'
import * as permissionsClient from '@/api/permissionsClient'

const mockListV1Roles = vi.mocked(rolesClient.listV1Roles)
const mockListPermissions = vi.mocked(permissionsClient.listPermissions)

const adminUser: User = {
  id: 'u1',
  email: 'admin@example.com',
  first_name: 'Admin',
  last_name: 'User',
  role: 'ADMIN_CHECK_ON',
  permissions: ['roles:create', 'roles:update', 'roles:delete'],
  status: 'ACTIVE',
}

function renderPage() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter initialEntries={['/dashboard/roles']}>
        <Routes>
          <Route path="/dashboard/roles" element={<RolesPage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

describe('RolesPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    useAuthStore.setState({ user: adminUser, isAuthenticated: true, accessToken: 'tok' })
    mockListPermissions.mockResolvedValue([])
  })

  it('renders roles table', async () => {
    mockListV1Roles.mockResolvedValueOnce([
      {
        id: '1',
        code: 'ADMIN_CHECK_ON',
        name: 'Admin',
        scope: 'GLOBAL',
        is_system: true,
        permission_count: 5,
      },
    ])

    renderPage()

    await waitFor(() => {
      expect(screen.getAllByTestId('role-row')).toHaveLength(1)
    })
    expect(screen.getByText('Admin')).toBeInTheDocument()
  })

  it('opens drawer on new role button click', async () => {
    mockListV1Roles.mockResolvedValueOnce([])

    renderPage()

    await waitFor(() => {
      expect(screen.getByTestId('new-role-button')).toBeInTheDocument()
    })

    fireEvent.click(screen.getByTestId('new-role-button'))

    // Drawer opens — heading appears alongside the button label
    const headings = screen.getAllByText('New Role')
    expect(headings.length).toBeGreaterThanOrEqual(1)
  })
})

import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter } from 'react-router-dom'
import { PermissionsPage } from '../PermissionsPage'
import { useAuthStore } from '@/store/authStore'
import type { User } from '@/types/users'

vi.mock('@/api/permissionsClient', () => ({
  listPermissions: vi.fn(),
  createPermission: vi.fn(),
  updatePermission: vi.fn(),
  deletePermission: vi.fn(),
}))

import * as permissionsClient from '@/api/permissionsClient'

const mockListPermissions = vi.mocked(permissionsClient.listPermissions)
const mockCreatePermission = vi.mocked(permissionsClient.createPermission)

const adminCheckOnUser: User = {
  id: 'u1',
  email: 'admin@example.com',
  first_name: 'Admin',
  last_name: 'CheckOn',
  role: 'ADMIN_CHECK_ON',
  permissions: [],
  status: 'ACTIVE',
}

const regularAdminUser: User = {
  ...adminCheckOnUser,
  role: 'ADMINISTRADOR',
}

function renderPage() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter initialEntries={['/dashboard/permissions']}>
        <PermissionsPage />
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

describe('PermissionsPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockListPermissions.mockResolvedValue([
      {
        id: 'p1',
        code: 'roles:update',
        name: 'Update Roles',
        module: 'roles',
        description: 'Update role metadata',
      },
    ])
  })

  it('allows ADMIN_CHECK_ON to create a permission', async () => {
    useAuthStore.setState({ user: adminCheckOnUser, isAuthenticated: true, accessToken: 'tok' })
    mockCreatePermission.mockResolvedValueOnce({
      id: 'p2',
      code: 'reports:read',
      name: 'Read Reports',
      module: 'reports',
      description: 'Read reports',
    })

    renderPage()
    const user = userEvent.setup()

    await waitFor(() => {
      expect(screen.getByTestId('new-permission-button')).toBeInTheDocument()
    })

    await user.click(screen.getByTestId('new-permission-button'))
    await user.type(screen.getByLabelText(/code/i), 'reports:read')
    await user.type(screen.getByLabelText(/name/i), 'Read Reports')
    await user.type(screen.getByLabelText(/module/i), 'reports')
    await user.click(screen.getByRole('button', { name: /create permission/i }))

    await waitFor(() => {
      expect(mockCreatePermission).toHaveBeenCalledWith(
        {
          code: 'reports:read',
          name: 'Read Reports',
          module: 'reports',
          description: '',
        },
        expect.anything(),
      )
    })
  })

  it('hides create and disables mutations for ADMINISTRADOR', async () => {
    useAuthStore.setState({ user: regularAdminUser, isAuthenticated: true, accessToken: 'tok' })

    renderPage()

    await waitFor(() => {
      expect(screen.getByTestId('permission-row')).toBeInTheDocument()
    })

    expect(screen.queryByTestId('new-permission-button')).not.toBeInTheDocument()
    expect(screen.getByTestId('permission-edit-button')).toBeDisabled()
    expect(screen.getByTestId('permission-delete-button')).toBeDisabled()
  })
})

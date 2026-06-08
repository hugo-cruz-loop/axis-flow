import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter, Routes, Route } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { UsersListPage } from '../UsersListPage'

vi.mock('@/api/usersClient', () => ({
  listAllUsers: vi.fn(),
}))

import * as usersClient from '@/api/usersClient'
const mockListAllUsers = vi.mocked(usersClient.listAllUsers)

function renderUsersListPage() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter initialEntries={['/admin/users']}>
        <Routes>
          <Route path="/admin/users" element={<UsersListPage />} />
          <Route path="/admin/users/new" element={<div>Create User</div>} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

describe('UsersListPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('renders list of users', async () => {
    mockListAllUsers.mockResolvedValueOnce([
      {
        id: '1',
        email: 'alice@example.com',
        first_name: 'Alice',
        last_name: 'Smith',
        role: 'CLIENTE',
        permissions: [],
        status: 'ACTIVE',
      },
    ])

    renderUsersListPage()

    await waitFor(() => {
      expect(screen.getAllByTestId('user-row')).toHaveLength(1)
    })
    expect(screen.getByText('alice@example.com')).toBeInTheDocument()
  })

  it('shows empty state when no users', async () => {
    mockListAllUsers.mockResolvedValueOnce([])

    renderUsersListPage()

    await waitFor(() => {
      expect(screen.getByTestId('empty-state')).toBeInTheDocument()
    })
  })

  it('shows error state on fetch failure', async () => {
    mockListAllUsers.mockRejectedValueOnce(new Error('Network error'))

    renderUsersListPage()

    await waitFor(() => {
      expect(screen.getByTestId('error-state')).toBeInTheDocument()
    })
  })
})

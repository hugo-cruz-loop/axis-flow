import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Routes, Route } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { CreateUserPage } from '../CreateUserPage'

vi.mock('@/api/usersClient', () => ({
  createUser: vi.fn(),
  listByRole: vi.fn(),
}))

import * as usersClient from '@/api/usersClient'
const mockCreateUser = vi.mocked(usersClient.createUser)

function renderCreateUserPage() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter initialEntries={['/admin/users/new']}>
        <Routes>
          <Route path="/admin/users/new" element={<CreateUserPage />} />
          <Route path="/admin/users" element={<div>Users List</div>} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

describe('CreateUserPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('renders form fields: email, first name, last name, role', () => {
    renderCreateUserPage()
    expect(screen.getByLabelText(/email/i)).toBeInTheDocument()
    expect(screen.getByLabelText(/first name/i)).toBeInTheDocument()
    expect(screen.getByLabelText(/last name/i)).toBeInTheDocument()
    expect(screen.getByLabelText(/role/i)).toBeInTheDocument()
  })

  it('submits and shows success', async () => {
    mockCreateUser.mockResolvedValueOnce({
      id: '1',
      email: 'new@example.com',
      first_name: 'New',
      last_name: 'User',
      role: 'CLIENTE',
      status: 'PENDING_ACTIVATION',
    })

    renderCreateUserPage()
    const user = userEvent.setup()

    await user.type(screen.getByLabelText(/email/i), 'new@example.com')
    await user.type(screen.getByLabelText(/first name/i), 'New')
    await user.type(screen.getByLabelText(/last name/i), 'User')
    await user.selectOptions(screen.getByLabelText(/role/i), 'CLIENTE')
    await user.click(screen.getByRole('button', { name: /create user/i }))

    await waitFor(() => {
      expect(screen.getByText('Users List')).toBeInTheDocument()
    })
  })

  it('shows validation error for invalid email', async () => {
    renderCreateUserPage()
    const user = userEvent.setup()

    await user.type(screen.getByLabelText(/email/i), 'not-an-email')
    await user.click(screen.getByRole('button', { name: /create user/i }))

    await waitFor(() => {
      expect(screen.getAllByRole('alert').length).toBeGreaterThan(0)
    })
  })
})

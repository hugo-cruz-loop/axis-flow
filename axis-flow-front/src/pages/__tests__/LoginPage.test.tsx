import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Routes, Route } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { LoginPage } from '../LoginPage'

vi.mock('@/api/authClient', () => ({
  login: vi.fn(),
  me: vi.fn(),
  meWithToken: vi.fn(),
}))

vi.mock('@/store/authStore', () => ({
  useAuthStore: vi.fn((selector: (s: unknown) => unknown) =>
    selector({
      setAuth: vi.fn(),
      isAuthenticated: false,
    }),
  ),
}))

import * as authClient from '@/api/authClient'

const mockLogin = vi.mocked(authClient.login)
const mockMe = vi.mocked(authClient.meWithToken)

function renderLoginPage() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter initialEntries={['/login']}>
        <Routes>
          <Route path="/login" element={<LoginPage />} />
          <Route path="/account/me" element={<div>Account Page</div>} />
          <Route path="/dashboard" element={<div>Dashboard Page</div>} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

describe('LoginPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('renders email and password fields', () => {
    renderLoginPage()
    expect(screen.getByLabelText(/email/i)).toBeInTheDocument()
    expect(screen.getByLabelText(/password/i)).toBeInTheDocument()
  })

  it('shows error on invalid credentials', async () => {
    mockLogin.mockRejectedValueOnce(
      Object.assign(new Error('Unauthorized'), {
        response: { status: 401 },
      }),
    )

    renderLoginPage()
    const user = userEvent.setup()

    await user.type(screen.getByLabelText(/email/i), 'user@example.com')
    await user.type(screen.getByLabelText(/password/i), 'wrongpassword')
    await user.click(screen.getByRole('button', { name: /sign in/i }))

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeInTheDocument()
    })
  })

  it('redirects to /dashboard on success', async () => {
    mockLogin.mockResolvedValueOnce({
      access_token: 'token',
      refresh_token: 'refresh',
      user_id: '1',
      role: 'CLIENTE',
    })
    mockMe.mockResolvedValueOnce({
      id: '1',
      email: 'user@example.com',
      first_name: 'John',
      last_name: 'Doe',
      role: 'CLIENTE',
      status: 'ACTIVE',
    })

    renderLoginPage()
    const user = userEvent.setup()

    await user.type(screen.getByLabelText(/email/i), 'user@example.com')
    await user.type(screen.getByLabelText(/password/i), 'password123')
    await user.click(screen.getByRole('button', { name: /sign in/i }))

    await waitFor(() => {
      expect(mockMe).toHaveBeenCalledWith('token')
      expect(screen.getByText('Dashboard Page')).toBeInTheDocument()
    })
  })
})

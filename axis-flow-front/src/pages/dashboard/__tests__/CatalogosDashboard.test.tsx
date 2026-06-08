import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Routes, Route } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { CatalogosDashboard } from '../CatalogosDashboard'
import { useAuthStore } from '@/store/authStore'
import type { User } from '@/types/users'

vi.mock('@/api/catalogosClient', () => ({
  listCountries: vi.fn().mockResolvedValue([]),
  listStates: vi.fn().mockResolvedValue([]),
  listCitiesByState: vi.fn().mockResolvedValue([]),
  listBanks: vi.fn().mockResolvedValue([{ id: 1, code: 'BBVA', name: 'BBVA Bancomer' }]),
  listTaxRegimes: vi.fn().mockResolvedValue([]),
  listPaymentForms: vi.fn().mockResolvedValue([]),
  listPaymentConditions: vi.fn().mockResolvedValue([]),
  listWorkflowStatuses: vi.fn().mockResolvedValue([]),
  listComplaintTypes: vi.fn().mockResolvedValue([]),
  listServices: vi.fn().mockResolvedValue([]),
  listSubscriptionPlans: vi.fn().mockResolvedValue([]),
  listDatePeriodicities: vi.fn().mockResolvedValue([]),
  listJobCategories: vi.fn().mockResolvedValue([]),
  listJobTypes: vi.fn().mockResolvedValue([]),
  listHrAbsenceTypes: vi.fn().mockResolvedValue([]),
  createCountry: vi.fn(),
  deleteCountry: vi.fn(),
  createState: vi.fn(),
  deleteState: vi.fn(),
  createCity: vi.fn(),
  deleteCity: vi.fn(),
  createBank: vi.fn(),
  deleteBank: vi.fn(),
  createWorkflowStatus: vi.fn(),
  createJobCategory: vi.fn(),
  createJobType: vi.fn(),
}))

const adminUser: User = {
  id: '1',
  email: 'admin@example.com',
  first_name: 'Admin',
  last_name: 'User',
  role: 'ADMINISTRADOR',
  permissions: [],
  status: 'ACTIVE',
}

function renderPage() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter initialEntries={['/dashboard/catalogos']}>
        <Routes>
          <Route path="/dashboard/catalogos" element={<CatalogosDashboard />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

describe('CatalogosDashboard', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    useAuthStore.setState({ user: adminUser, isAuthenticated: true, accessToken: 'tok' })
  })

  it('renders Geography tab by default', () => {
    renderPage()
    expect(screen.getByText('Geography')).toBeInTheDocument()
    expect(screen.getByText('Countries')).toBeInTheDocument()
  })

  it('shows banks table in Financial tab', async () => {
    const user = userEvent.setup()
    renderPage()

    await user.click(screen.getByText('Financial'))

    await waitFor(() => {
      expect(screen.getByText('Banks')).toBeInTheDocument()
    })
    await waitFor(() => {
      expect(screen.getByText('BBVA Bancomer')).toBeInTheDocument()
    })
  })
})

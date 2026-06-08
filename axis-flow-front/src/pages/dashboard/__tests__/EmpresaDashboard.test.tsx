import { describe, it, expect, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Routes, Route } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { EmpresaDashboard } from '../empresa/EmpresaDashboard'

vi.mock('@/api/empresasClient', () => ({
  getEmpresa: vi.fn().mockResolvedValue({
    id: 1,
    nombre: 'Acme Corp',
    direccion: 'Av. Insurgentes 123',
    telefono: '5551234567',
    representante_id: 'uuid-1234',
    plan_id: 1,
    status: 'ACTIVE',
    created_at: '2024-01-01T00:00:00Z',
    updated_at: '2024-01-01T00:00:00Z',
  }),
  updateEmpresa: vi.fn(),
  getFiscal: vi.fn().mockResolvedValue({
    id: 1,
    empresa_id: 1,
    rfc: 'ACM850101XX1',
    razon_social: 'Acme Corp SA de CV',
  }),
  createFiscal: vi.fn(),
  updateFiscal: vi.fn(),
  listApoderados: vi.fn().mockResolvedValue([]),
  createApoderado: vi.fn(),
  updateApoderado: vi.fn(),
  deleteApoderado: vi.fn(),
  listServicios: vi.fn().mockResolvedValue([]),
  createServicio: vi.fn(),
  updateServicio: vi.fn(),
  deleteServicio: vi.fn(),
}))

vi.mock('@/api/catalogosClient', () => ({
  listSubscriptionPlans: vi.fn().mockResolvedValue([]),
}))

vi.mock('@/layouts/DashboardLayout', () => ({
  DashboardLayout: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
}))

function renderPage() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter initialEntries={['/dashboard/empresa/1']}>
        <Routes>
          <Route path="/dashboard/empresa/:id" element={<EmpresaDashboard />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

describe('EmpresaDashboard', () => {
  it('renders company name and status badge', async () => {
    renderPage()
    await waitFor(() => {
      expect(screen.getAllByText('Acme Corp').length).toBeGreaterThan(0)
    })
    expect(screen.getByText('ACTIVE')).toBeInTheDocument()
  })

  it('shows fiscal tab content', async () => {
    const user = userEvent.setup()
    renderPage()
    await waitFor(() => {
      expect(screen.getAllByText('Acme Corp').length).toBeGreaterThan(0)
    })
    await user.click(screen.getByRole('button', { name: /fiscal/i }))
    await waitFor(() => {
      expect(screen.getByText('ACM850101XX1')).toBeInTheDocument()
    })
  })
})

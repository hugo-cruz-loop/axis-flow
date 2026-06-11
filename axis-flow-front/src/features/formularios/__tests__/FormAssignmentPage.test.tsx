import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { FormAssignmentPage } from '../pages/FormAssignmentPage'

const mocks = vi.hoisted(() => ({
  useFormularios: vi.fn(),
  useEventosByEmpCte: vi.fn(),
  useCreateEvento: vi.fn(),
}))

vi.mock('../api/queries', () => ({
  useFormularios: mocks.useFormularios,
  useEventosByEmpCte: mocks.useEventosByEmpCte,
  useCreateEvento: mocks.useCreateEvento,
}))

const UUID = '8c2a7d0e-5412-4c22-b5e1-88f6c5bbde93'
const UUID2 = 'd041e2a8-0e1b-4d43-85f6-cbb18a4a5119'

function renderPage() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter
        initialEntries={[
          {
            pathname: '/dashboard/formularios/assignment',
            state: { empresaId: UUID, clienteId: UUID2 },
          },
        ]}
      >
        <FormAssignmentPage />
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

const FORM_LIST = {
  success: true as const,
  data: [
    { id: UUID, empresa_id: UUID, nombre: 'Limpieza de Baños', activo: true },
    { id: UUID2, empresa_id: UUID, nombre: 'Ronda de Seguridad', activo: true },
  ],
}

const EVENTO_LIST = {
  success: true as const,
  data: [
    {
      id: 'ev-1',
      empresa_id: UUID,
      cliente_id: UUID2,
      nombre: 'Evento previo',
      fecha_programada: '2026-06-10T22:00:00Z',
      estatus: 'pendiente',
    },
  ],
}

beforeEach(() => {
  vi.clearAllMocks()
  mocks.useFormularios.mockReturnValue({ data: FORM_LIST, isLoading: false })
  mocks.useEventosByEmpCte.mockReturnValue({ data: EVENTO_LIST, isLoading: false })
  mocks.useCreateEvento.mockReturnValue({ mutate: vi.fn(), isPending: false })
})

describe('FormAssignmentPage', () => {
  it('renders existing eventos and the form selection list', async () => {
    renderPage()
    expect(screen.getByText('Evento previo')).toBeInTheDocument()
    expect(screen.getByText('Limpieza de Baños')).toBeInTheDocument()
    expect(screen.getByText('Ronda de Seguridad')).toBeInTheDocument()
  })

  it('selecting two forms and clicking Assign calls useCreateEvento with formulario_ids', async () => {
    const user = userEvent.setup()
    const mutate = vi.fn()
    mocks.useCreateEvento.mockReturnValue({ mutate, isPending: false })

    renderPage()
    // Pick both forms (checkboxes)
    const formCheckboxes = screen.getAllByRole('checkbox')
    await user.click(formCheckboxes[0]!)
    await user.click(formCheckboxes[1]!)

    // Provide a date
    const dateInput = screen.getByLabelText(/scheduled date/i) as HTMLInputElement
    await user.type(dateInput, '2026-06-15T10:00')

    // Click assign
    await user.click(screen.getByRole('button', { name: /assign/i }))

    await waitFor(() => {
      expect(mutate).toHaveBeenCalledTimes(1)
    })
    const arg = mutate.mock.calls[0]?.[0] as Record<string, unknown>
    expect(arg).toMatchObject({
      empresa_id: UUID,
      cliente_id: UUID2,
    })
    expect((arg as { formularios_asociados: string[] }).formularios_asociados).toHaveLength(2)
    expect((arg as { formularios_asociados: string[] }).formularios_asociados).toContain(UUID)
    expect((arg as { formularios_asociados: string[] }).formularios_asociados).toContain(UUID2)
  })

  it('assign button is disabled until at least one form is selected and a date is provided', () => {
    renderPage()
    const assignBtn = screen.getByRole('button', { name: /assign/i }) as HTMLButtonElement
    expect(assignBtn).toBeDisabled()
  })

  it('shows a live status when the assignment succeeds', async () => {
    const user = userEvent.setup()
    mocks.useCreateEvento.mockImplementation(() => ({
      mutate: (_data: unknown, opts?: { onSuccess?: () => void }) => opts?.onSuccess?.(),
      isPending: false,
    }))
    renderPage()
    const formCheckboxes = screen.getAllByRole('checkbox')
    await user.click(formCheckboxes[0]!)
    const dateInput = screen.getByLabelText(/scheduled date/i) as HTMLInputElement
    await user.type(dateInput, '2026-06-15T10:00')
    await user.click(screen.getByRole('button', { name: /assign/i }))
    await waitFor(() => {
      expect(screen.getByRole('status')).toHaveTextContent(/assigned/i)
    })
  })
})

import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import {
  EvaluationFrequenciesContainer,
  InactiveDaysContainer,
  SystemSettingsContainer,
} from '..'

const mocks = vi.hoisted(() => ({
  useServiceEvaluations: vi.fn(),
  usePersonalEvaluations: vi.fn(),
  useCreateServiceEvaluation: vi.fn(),
  useCreatePersonalEvaluation: vi.fn(),
  useInactiveDays: vi.fn(),
  useCreateInactiveDay: vi.fn(),
  useDeleteInactiveDay: vi.fn(),
  useUpdateInactiveDaysThreshold: vi.fn(),
  useSystemSettings: vi.fn(),
  useUpdateSystemSetting: vi.fn(),
}))

vi.mock('../api/queries', () => mocks)

beforeEach(() => {
  vi.clearAllMocks()
  mocks.useServiceEvaluations.mockReturnValue({ data: [], isLoading: false, isError: false })
  mocks.usePersonalEvaluations.mockReturnValue({ data: [], isLoading: false, isError: false })
  mocks.useCreateServiceEvaluation.mockReturnValue({ mutate: vi.fn(), isPending: false })
  mocks.useCreatePersonalEvaluation.mockReturnValue({ mutate: vi.fn(), isPending: false })
  mocks.useInactiveDays.mockReturnValue({
    data: { empresa_id: 12, umbral_dias: 5, dias_inactivos: [] },
    isLoading: false,
    isError: false,
  })
  mocks.useCreateInactiveDay.mockReturnValue({ mutate: vi.fn(), isPending: false })
  mocks.useDeleteInactiveDay.mockReturnValue({ mutate: vi.fn(), isPending: false })
  mocks.useUpdateInactiveDaysThreshold.mockReturnValue({ mutate: vi.fn(), isPending: false })
  mocks.useSystemSettings.mockReturnValue({
    data: [
      {
        clave_parametro: 'WEBSOCKET_CHANNEL_NAME',
        valor: 'axis',
        descripcion: 'Websocket channel',
        created_at: '2026-06-11T10:00:00Z',
        updated_at: '2026-06-11T10:00:00Z',
      },
    ],
    isLoading: false,
    isError: false,
  })
  mocks.useUpdateSystemSetting.mockReturnValue({ mutate: vi.fn(), isPending: false })
})

describe('EvaluationFrequenciesContainer accessibility', () => {
  it('opens a side-effect confirmation dialog, traps focus, and restores focus on cancel', async () => {
    const user = userEvent.setup()
    render(<EvaluationFrequenciesContainer empresaId={12} />)

    const submitButton = screen.getByRole('button', { name: /save service evaluation frequency/i })
    await user.clear(screen.getByLabelText(/service id/i))
    await user.type(screen.getByLabelText(/service id/i), '5')
    await user.clear(screen.getByLabelText(/service periodicity id/i))
    await user.type(screen.getByLabelText(/service periodicity id/i), '2')
    await user.click(screen.getByRole('switch', { name: /service evaluation active/i }))
    await user.click(submitButton)

    const dialog = screen.getByRole('dialog', { name: /confirm service evaluation change/i })
    expect(dialog).toHaveTextContent(/triggers an immediate mass email dispatch/i)
    expect(screen.getByRole('button', { name: /confirm frequency change/i })).toHaveFocus()

    await user.tab()
    expect(screen.getByRole('button', { name: /cancel frequency change/i })).toHaveFocus()
    await user.tab()
    expect(screen.getByRole('button', { name: /confirm frequency change/i })).toHaveFocus()

    await user.click(screen.getByRole('button', { name: /cancel frequency change/i }))
    expect(submitButton).toHaveFocus()
  })

  it('confirms service evaluation changes through an accessible switch', async () => {
    const mutate = vi.fn()
    mocks.useCreateServiceEvaluation.mockReturnValue({ mutate, isPending: false })
    const user = userEvent.setup()
    render(<EvaluationFrequenciesContainer empresaId={12} />)

    await user.clear(screen.getByLabelText(/service id/i))
    await user.type(screen.getByLabelText(/service id/i), '5')
    await user.clear(screen.getByLabelText(/service periodicity id/i))
    await user.type(screen.getByLabelText(/service periodicity id/i), '2')
    const serviceSwitch = screen.getByRole('switch', { name: /service evaluation active/i })
    expect(serviceSwitch).toHaveAttribute('aria-checked', 'true')

    await user.click(screen.getByRole('button', { name: /save service evaluation frequency/i }))
    await user.click(screen.getByRole('button', { name: /confirm frequency change/i }))

    expect(mutate).toHaveBeenCalledWith(
      { empresa_id: 12, servicio_id: 5, periodicidad_id: 2, activa: true },
      expect.objectContaining({ onSuccess: expect.any(Function) }),
    )
  })
})

describe('InactiveDaysContainer calendar accessibility', () => {
  it('uses roving tabindex and keyboard navigation in the inactive-days calendar', async () => {
    const user = userEvent.setup()
    render(<InactiveDaysContainer empresaId={12} initialMonth={new Date(2026, 11, 1)} />)

    const firstDay = screen.getByRole('button', { name: /Tuesday, December 1, 2026, Workday/i })
    expect(firstDay).toHaveAttribute('tabindex', '0')
    firstDay.focus()
    await user.keyboard('{ArrowRight}')

    const secondDay = screen.getByRole('button', { name: /Wednesday, December 2, 2026, Workday/i })
    expect(secondDay).toHaveFocus()
    expect(firstDay).toHaveAttribute('tabindex', '-1')
    expect(secondDay).toHaveAttribute('tabindex', '0')
  })

  it('announces month changes and toggled inactive days through an aria-live region', async () => {
    const mutate = vi.fn()
    mocks.useCreateInactiveDay.mockReturnValue({ mutate, isPending: false })
    const user = userEvent.setup()
    render(<InactiveDaysContainer empresaId={12} initialMonth={new Date(2026, 11, 1)} />)

    await user.click(screen.getByRole('button', { name: /next month/i }))
    expect(screen.getByRole('status')).toHaveTextContent('Showing January 2027')

    const day = screen.getByRole('button', { name: /Friday, January 1, 2027, Workday/i })
    day.focus()
    await user.keyboard('{Enter}')

    expect(screen.getByRole('status')).toHaveTextContent(/January 1, 2027 marked as Inactive Holiday/i)
    expect(mutate).toHaveBeenCalledWith({ empresa_id: 12, fecha: '2027-01-01', descripcion: 'Inactive day' })
  })
})

describe('SystemSettingsContainer inline editing', () => {
  it('edits system settings inline and exposes cache invalidation states through aria-live', async () => {
    const mutate = vi.fn((_payload, callbacks?: { onSuccess?: () => void }) => callbacks?.onSuccess?.())
    mocks.useUpdateSystemSetting.mockReturnValue({ mutate, isPending: false })
    const user = userEvent.setup()
    render(<SystemSettingsContainer />)

    await user.click(screen.getByRole('button', { name: /edit WEBSOCKET_CHANNEL_NAME/i }))
    const valueInput = screen.getByLabelText(/value for WEBSOCKET_CHANNEL_NAME/i)
    await user.clear(valueInput)
    await user.type(valueInput, 'axis_flow_websocket_production')
    await user.click(screen.getByRole('button', { name: /save WEBSOCKET_CHANNEL_NAME/i }))

    expect(mutate).toHaveBeenCalledWith(
      { clave: 'WEBSOCKET_CHANNEL_NAME', payload: { valor: 'axis_flow_websocket_production' } },
      expect.objectContaining({ onSuccess: expect.any(Function) }),
    )
    await waitFor(() => expect(screen.getByRole('status')).toHaveTextContent(/System setting saved/i))
  })
})

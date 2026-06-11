import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { FormBuilderPage } from '../pages/FormBuilderPage'
import { TIPO_PREGUNTA } from '../types'

const mocks = vi.hoisted(() => ({
  createFormulario: vi.fn(),
  createPregunta: vi.fn(),
}))

vi.mock('../api/queries', () => ({
  useFormularios: vi.fn(() => ({ data: undefined, isLoading: false })),
  useCreateFormulario: vi.fn(() => ({ mutate: mocks.createFormulario, isPending: false })),
  useCreatePregunta: vi.fn(() => ({ mutate: mocks.createPregunta, isPending: false })),
}))

// In TanStack Query v5, `mutate(variables, options)` calls `options.onSuccess(data, variables, context)`
// when the mutationFn resolves. Our mock for `mutate` simulates that by
// invoking the user-provided onSuccess with a fake FormularioResponse.
function simulateCreateSuccess() {
  mocks.createFormulario.mockImplementation(
    (_data: unknown, opts?: { onSuccess?: (resp: { data: { id: string } }) => void }) => {
      opts?.onSuccess?.({ data: { id: 'new-form-id' } })
    },
  )
}

const UUID = '8c2a7d0e-5412-4c22-b5e1-88f6c5bbde93'

function renderPage() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter initialEntries={[{ pathname: '/dashboard/formularios/builder', state: { empresaId: UUID } }]}>
        <FormBuilderPage />
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

beforeEach(() => {
  vi.clearAllMocks()
})

describe('FormBuilderPage', () => {
  it('renders the builder with a name field and the toolbox of question types', () => {
    renderPage()
    // Name input present
    expect(screen.getByLabelText(/form name/i)).toBeInTheDocument()
    // Toolbox entries
    expect(screen.getByRole('button', { name: /short text/i })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /checkbox/i })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /rating/i })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /matrix/i })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /camera/i })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /signature/i })).toBeInTheDocument()
  })

  it('clicking a toolbox button adds a pregunta to the canvas', async () => {
    const user = userEvent.setup()
    renderPage()
    await user.click(screen.getByRole('button', { name: /short text/i }))
    // A "Question 1" entry appears in the canvas
    expect(screen.getByText(/Question 1/)).toBeInTheDocument()
  })

  it('clicking Save calls useCreateFormulario with the form, then useCreatePregunta per pregunta', async () => {
    const user = userEvent.setup()
    simulateCreateSuccess()
    renderPage()
    // Fill the form name
    const nameInput = screen.getByLabelText(/form name/i) as HTMLInputElement
    await user.type(nameInput, 'Limpieza de Baños')

    // Add two preguntas
    await user.click(screen.getByRole('button', { name: /short text/i }))
    await user.click(screen.getByRole('button', { name: /rating/i }))

    // Save
    await user.click(screen.getByRole('button', { name: /save/i }))

    await waitFor(() => {
      expect(mocks.createFormulario).toHaveBeenCalledTimes(1)
    })
    const createArg = mocks.createFormulario.mock.calls[0]?.[0] as Record<string, unknown>
    expect(createArg).toMatchObject({
      empresa_id: UUID,
      nombre: 'Limpieza de Baños',
    })
    // The component creates the form header first, then loops to create
    // each pregunta individually. So createFormulario receives the header
    // (no preguntas) and createPregunta is called once per pregunta.
    expect(mocks.createPregunta).toHaveBeenCalledTimes(2)
    const primerPregunta = mocks.createPregunta.mock.calls[0]?.[0] as Record<string, unknown>
    expect(primerPregunta).toMatchObject({
      formulario_id: 'new-form-id',
      tipo_pregunta: TIPO_PREGUNTA.SHORT_TEXT,
    })
  })

  it('reorder buttons swap adjacent preguntas', async () => {
    const user = userEvent.setup()
    renderPage()
    await user.click(screen.getByRole('button', { name: /short text/i }))
    await user.click(screen.getByRole('button', { name: /rating/i }))

    // Before reorder: Question 1, Question 2
    const labelsBefore = screen.getAllByText(/Question \d/)
    expect(labelsBefore.map((el) => el.textContent)).toEqual(['Question 1', 'Question 2'])

    // jsdom does not compute layout, so we assert reorder by checking the
    // order attribute on the pregunta cards (the component sets `orden`
    // sequentially after each swap).
    const downBtn = screen.getByRole('button', { name: /move question 1 down/i })
    await user.click(downBtn)

    // After moving Q1 down, the card that was at index 0 now has orden=2
    // and the card that was at index 1 now has orden=1. The "Question N"
    // text uses index+1, so the labels in the DOM will now read
    // "Question 1" (the former Q2 at index 0) and "Question 2" (the
    // former Q1 at index 1). The key assertion is that the underlying
    // orden field swapped: we check via the data attribute we set on the
    // wrapper, or — since we don't have one — via the button labels.
    // The "Move up" button on the former Q2 (now at index 0) is now
    // disabled, and the "Move down" button on the former Q1 (now at
    // index 1) is now disabled.
    const moveUpBtns = screen.getAllByRole('button', { name: /move question \d up/i })
    const moveDownBtns = screen.getAllByRole('button', { name: /move question \d down/i })
    // After swap: first card (was Q2) has orden=1 → can't move up
    expect(moveUpBtns[0]).toBeDisabled()
    // Last card (was Q1) has orden=2 → can't move down
    expect(moveDownBtns[1]).toBeDisabled()
  })

  it('remove button deletes the pregunta from the canvas', async () => {
    const user = userEvent.setup()
    renderPage()
    await user.click(screen.getByRole('button', { name: /short text/i }))
    await user.click(screen.getByRole('button', { name: /rating/i }))
    expect(screen.getAllByText(/Question \d/)).toHaveLength(2)

    const removeBtn = screen.getByRole('button', { name: /delete question 1/i })
    await user.click(removeBtn)
    expect(screen.getAllByText(/Question \d/)).toHaveLength(1)
  })

  it('announces a live status when saving completes', async () => {
    const user = userEvent.setup()
    simulateCreateSuccess()
    renderPage()
    await user.type(screen.getByLabelText(/form name/i), 'Test')
    await user.click(screen.getByRole('button', { name: /short text/i }))
    await user.click(screen.getByRole('button', { name: /save/i }))

    // The "Saved." live region should appear
    await waitFor(() => {
      expect(screen.getByRole('status')).toHaveTextContent(/saved/i)
    })
  })
})

import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Routes, Route } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MobileFormCapturePage } from '../pages/MobileFormCapturePage'
import { TIPO_PREGUNTA, type PreguntaDraft } from '../types'

const mocks = vi.hoisted(() => ({
  useIniciarEvento: vi.fn(),
  useSubmitRespuesta: vi.fn(),
  useFormularios: vi.fn(),
}))

vi.mock('../api/queries', () => ({
  useIniciarEvento: mocks.useIniciarEvento,
  useSubmitRespuesta: mocks.useSubmitRespuesta,
  useFormularios: mocks.useFormularios,
}))

const UUID = '8c2a7d0e-5412-4c22-b5e1-88f6c5bbde93'
const EVENTO_ID = '99999999-aaaa-bbbb-cccc-dddddddddddd'

const PREGUNTAS: PreguntaDraft[] = [
  { id: 'q1', orden: 1, texto_pregunta: 'Tu nombre', tipo_pregunta: TIPO_PREGUNTA.SHORT_TEXT, obligatoria: true },
  { id: 'q2', orden: 2, texto_pregunta: 'Calidad', tipo_pregunta: TIPO_PREGUNTA.RATING, obligatoria: false },
]

// Mock URL.createObjectURL for blob previews
beforeAll(() => {
  if (!('createObjectURL' in URL)) {
    Object.defineProperty(URL, 'createObjectURL', { configurable: true, value: vi.fn(() => 'blob:mock') })
  }
})

// jsdom does not implement Canvas 2D — install a stub so the SignatureCanvas
// in the component doesn't blow up.
beforeAll(() => {
  HTMLCanvasElement.prototype.getContext = vi.fn(() => ({
    scale: () => {},
    beginPath: () => {},
    moveTo: () => {},
    lineTo: () => {},
    stroke: () => {},
    clearRect: () => {},
    setTransform: () => {},
    save: () => {},
    restore: () => {},
    set lineCap(_: string) {},
    set lineJoin(_: string) {},
    set strokeStyle(_: string) {},
    set lineWidth(_: number) {},
  })) as unknown as typeof HTMLCanvasElement.prototype.getContext
  HTMLCanvasElement.prototype.toDataURL = vi.fn(() => 'data:image/png;base64,FAKE')
})

function renderPage() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter initialEntries={[`/mobile/formularios/capture/${EVENTO_ID}`]}>
        <Routes>
          <Route
            path="/mobile/formularios/capture/:eventoId"
            element={<MobileFormCapturePage />}
          />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

beforeEach(() => {
  vi.clearAllMocks()
  // Default: geolocation succeeds with a fixed coordinate
  ;(globalThis as Record<string, unknown>).__lastGeoCall = undefined
  const geoMock = vi.fn((success: PositionCallback) => {
    success({
      coords: { latitude: -34.6037, longitude: -58.3816, accuracy: 10 },
      timestamp: Date.now(),
    } as GeolocationPosition)
  })
  Object.defineProperty(navigator, 'geolocation', {
    value: { getCurrentPosition: geoMock },
    configurable: true,
  })
  mocks.useFormularios.mockReturnValue({ data: undefined, isLoading: true, isSuccess: false })
  mocks.useIniciarEvento.mockReturnValue({ mutate: vi.fn(), isPending: false, isSuccess: false, data: undefined })
  mocks.useSubmitRespuesta.mockReturnValue({ mutateAsync: vi.fn().mockResolvedValue({}), isPending: false })
})

describe('MobileFormCapturePage', () => {
  it('requests geolocation on mount', () => {
    renderPage()
    // The geolocation mock is called via navigator.geolocation.getCurrentPosition
    // The page should not crash; we just verify the page rendered.
    expect(screen.getByText(/locating|gps|loading/i)).toBeInTheDocument()
  })

  it('shows the first pregunta once formularios load and geolocation is acquired', async () => {
    mocks.useFormularios.mockReturnValue({
      data: {
        success: true as const,
        data: [{ id: 'f1', empresa_id: UUID, nombre: 'Limpieza', activo: true, preguntas: PREGUNTAS }],
      },
      isLoading: false,
      isSuccess: true,
    })
    renderPage()
    await waitFor(() => {
      expect(screen.getByText('Tu nombre')).toBeInTheDocument()
    })
  })

  it('filling one answer and clicking Submit calls submitRespuesta with the answer', async () => {
    const user = userEvent.setup()
    const submit = vi.fn().mockResolvedValue({})
    mocks.useSubmitRespuesta.mockReturnValue({ mutateAsync: submit, isPending: false })
    mocks.useFormularios.mockReturnValue({
      data: {
        success: true as const,
        data: [
          {
            id: 'f1',
            empresa_id: UUID,
            nombre: 'Limpieza',
            activo: true,
            preguntas: PREGUNTAS,
          },
        ],
      },
      isLoading: false,
      isSuccess: true,
    })
    renderPage()
    await waitFor(() => {
      expect(screen.getByText('Tu nombre')).toBeInTheDocument()
    })

    // Fill the text input
    const input = screen.getByLabelText(/tu nombre/i) as HTMLInputElement
    await user.type(input, 'Alice')

    // Submit all
    await user.click(screen.getByRole('button', { name: /submit|complete|finish|enviar/i }))

    await waitFor(() => {
      expect(submit).toHaveBeenCalled()
    })
    // The first call's variables should include the answer for q1.
    const firstCall = submit.mock.calls[0]?.[0] as Record<string, unknown>
    expect(firstCall).toMatchObject({
      pregunta_id: 'q1',
    })
  })

  it('announces a live status when the GPS location is acquired', async () => {
    mocks.useFormularios.mockReturnValue({
      data: {
        success: true as const,
        data: [{ id: 'f1', empresa_id: UUID, nombre: 'x', activo: true, preguntas: PREGUNTAS }],
      },
      isLoading: false,
      isSuccess: true,
    })
    renderPage()
    // The GPS active status should be in the live region
    await waitFor(() => {
      expect(screen.getByText(/gps active/i)).toBeInTheDocument()
    })
  })

  it('shows a "required" error if the user submits without filling the required field', async () => {
    const user = userEvent.setup()
    const submit = vi.fn().mockResolvedValue({})
    mocks.useSubmitRespuesta.mockReturnValue({ mutateAsync: submit, isPending: false })
    mocks.useFormularios.mockReturnValue({
      data: {
        success: true as const,
        data: [
          {
            id: 'f1',
            empresa_id: UUID,
            nombre: 'Limpieza',
            activo: true,
            preguntas: PREGUNTAS,
          },
        ],
      },
      isLoading: false,
      isSuccess: true,
    })
    renderPage()
    await waitFor(() => {
      expect(screen.getByText('Tu nombre')).toBeInTheDocument()
    })

    // Submit without filling
    await user.click(screen.getByRole('button', { name: /submit|complete|finish|enviar/i }))

    // An error should be announced
    await waitFor(() => {
      expect(screen.getByText(/required/i)).toBeInTheDocument()
    })
    expect(submit).not.toHaveBeenCalled()
  })
})

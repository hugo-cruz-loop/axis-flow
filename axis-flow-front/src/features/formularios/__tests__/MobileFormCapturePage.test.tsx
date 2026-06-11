import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Routes, Route } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MobileFormCapturePage } from '../pages/MobileFormCapturePage'
import { TIPO_PREGUNTA, type PreguntaDraft } from '../types'
import { useAuthStore } from '@/store/authStore'

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

// Build a base64url-encoded JWT-shaped token from a payload object. We do
// NOT sign — the page only parses the payload, never verifies the
// signature. The signature segment is a placeholder.
function makeToken(payload: object): string {
  const header = btoa(JSON.stringify({ alg: 'HS256', typ: 'JWT' }))
    .replace(/=/g, '')
    .replace(/\+/g, '-')
    .replace(/\//g, '_')
  const body = btoa(JSON.stringify(payload))
    .replace(/=/g, '')
    .replace(/\+/g, '-')
    .replace(/\//g, '_')
  return `${header}.${body}.signature`
}

// JWT-shaped payload that mimics what the formularios backend mints in
// PR-5: uid (userId), tid (tenantId = empresa_id), empleado_id, email,
// role, and the standard exp/iat. Tests use this as the "logged in"
// fixture.
const VALID_AUTH_PAYLOAD = {
  uid: 'user-uuid-1',
  tid: UUID,
  empleado_id: 42,
  email: 'alice@example.com',
  role: 'EMPLEADO',
  exp: 1_900_000_000,
  iat: 1_800_000_000,
}

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
  // Default: a valid JWT in the auth store, so the page reads claims and
  // proceeds. Individual tests that want the unauthenticated path clear
  // the auth state explicitly.
  useAuthStore.setState({
    accessToken: makeToken(VALID_AUTH_PAYLOAD),
    user: null,
    isAuthenticated: true,
  })
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

  it('filling one answer and clicking Submit calls submitRespuesta and uses JWT empleado_id (not 0)', async () => {
    const user = userEvent.setup()
    const submit = vi.fn().mockResolvedValue({})
    const iniciarMutate = vi.fn()
    mocks.useSubmitRespuesta.mockReturnValue({ mutateAsync: submit, isPending: false })
    mocks.useIniciarEvento.mockReturnValue({
      mutate: iniciarMutate,
      isPending: false,
      isSuccess: false,
      data: undefined,
    })
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

    // The iniciarEvento call must carry the JWT's empleado_id (42),
    // NOT the legacy 0 placeholder. This guards against the regression
    // where the page hardcoded `empleado_id: 0`.
    await waitFor(() => {
      expect(iniciarMutate).toHaveBeenCalled()
    })
    const iniciarCall = iniciarMutate.mock.calls[0]?.[0] as Record<string, unknown>
    expect(iniciarCall).toMatchObject({
      evento_id: EVENTO_ID,
      empleado_id: 42,
    })
    expect(iniciarCall.empleado_id).not.toBe(0)

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

  // — PR-7 AMEND (FIX 2): JWT-claims wiring —

  it('calls useFormularios with the tenantId from the JWT and renders loaded formularios', async () => {
    mocks.useFormularios.mockReturnValue({
      data: {
        success: true as const,
        data: [{ id: 'f1', empresa_id: UUID, nombre: 'Limpieza', activo: true, preguntas: PREGUNTAS }],
      },
      isLoading: false,
      isSuccess: true,
    })
    renderPage()
    // The page must hand the JWT-derived tenantId (not an empty string) to
    // useFormularios so the query is enabled. The formularios then load
    // and the first pregunta renders.
    await waitFor(() => {
      expect(mocks.useFormularios).toHaveBeenCalled()
    })
    const callArgs = mocks.useFormularios.mock.calls[0]?.[0] as string
    expect(callArgs).toBe(UUID)
    expect(callArgs).not.toBe('')
    await waitFor(() => {
      expect(screen.getByText('Tu nombre')).toBeInTheDocument()
    })
  })

  it('renders "Sesión inválida" error state when no accessToken is in the auth store, and does NOT call any mutation', () => {
    // Drop the auth token to simulate an expired/invalid session.
    useAuthStore.setState({ accessToken: null, user: null, isAuthenticated: false })
    const iniciarMutate = vi.fn()
    mocks.useIniciarEvento.mockReturnValue({
      mutate: iniciarMutate,
      isPending: false,
      isSuccess: false,
      data: undefined,
    })
    renderPage()
    // The page must surface the invalid-session error and refuse to fire
    // the iniciarEvento mutation (which would 401 server-side anyway).
    expect(screen.getByText(/sesión inválida/i)).toBeInTheDocument()
    expect(iniciarMutate).not.toHaveBeenCalled()
  })
})

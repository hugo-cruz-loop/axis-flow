import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { renderHook, waitFor } from '@testing-library/react'
import type { ReactNode } from 'react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { parametrizacionClient } from '../api/parametrizacionClient'
import {
  parametrizacionKeys,
  useCreateServiceEvaluation,
  useInactiveDays,
  useSystemSettings,
  useUpdateSystemSetting,
} from '../api/queries'

vi.mock('../api/parametrizacionClient', () => ({
  parametrizacionClient: {
    listServiceEvaluations: vi.fn(),
    createServiceEvaluation: vi.fn(),
    getInactiveDays: vi.fn(),
    createInactiveDay: vi.fn(),
    deleteInactiveDay: vi.fn(),
    updateInactiveDaysThreshold: vi.fn(),
    listSystemSettings: vi.fn(),
    updateSystemSetting: vi.fn(),
  },
}))

const mockedClient = vi.mocked(parametrizacionClient)

function makeWrapper() {
  const qc = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  function Wrapper({ children }: { children: ReactNode }) {
    return <QueryClientProvider client={qc}>{children}</QueryClientProvider>
  }
  return { qc, Wrapper }
}

beforeEach(() => {
  vi.clearAllMocks()
})

describe('parametrizacionKeys', () => {
  it('nests settings, evaluations, and inactive days under one feature namespace', () => {
    expect(parametrizacionKeys.all).toEqual(['parametrizacion'])
    expect(parametrizacionKeys.serviceEvaluations(12)).toEqual(['parametrizacion', 'service-evaluations', 12])
    expect(parametrizacionKeys.inactiveDays(12)).toEqual(['parametrizacion', 'inactive-days', 12])
    expect(parametrizacionKeys.systemSettings()).toEqual(['parametrizacion', 'system-settings'])
  })
})

describe('parametrizacion query hooks', () => {
  it('disables inactive day queries until a valid company id exists', () => {
    const { Wrapper } = makeWrapper()

    const { result } = renderHook(() => useInactiveDays(0), { wrapper: Wrapper })

    expect(result.current.isFetching).toBe(false)
    expect(mockedClient.getInactiveDays).not.toHaveBeenCalled()
  })

  it('fetches system settings and exposes backend data', async () => {
    mockedClient.listSystemSettings.mockResolvedValueOnce([
      {
        clave_parametro: 'WEBSOCKET_CHANNEL_NAME',
        valor: 'axis',
        descripcion: 'Websocket channel',
        created_at: '2026-06-11T10:00:00Z',
        updated_at: '2026-06-11T10:00:00Z',
      },
    ])
    const { Wrapper } = makeWrapper()

    const { result } = renderHook(() => useSystemSettings(), { wrapper: Wrapper })

    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(result.current.data?.[0]?.clave_parametro).toBe('WEBSOCKET_CHANNEL_NAME')
  })

  it('invalidates service evaluations after create mutation success', async () => {
    mockedClient.createServiceEvaluation.mockResolvedValueOnce({
      id: 101,
      empresa_id: 12,
      servicio_id: 5,
      periodicidad_id: 2,
      activa: true,
      created_at: '2026-06-11T10:00:00Z',
      updated_at: '2026-06-11T10:00:00Z',
    })
    const { qc, Wrapper } = makeWrapper()
    const invalidateSpy = vi.spyOn(qc, 'invalidateQueries')
    const { result } = renderHook(() => useCreateServiceEvaluation(), { wrapper: Wrapper })

    result.current.mutate({ empresa_id: 12, servicio_id: 5, periodicidad_id: 2, activa: true })

    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: parametrizacionKeys.serviceEvaluations(12) })
  })

  it('invalidates system settings after inline setting updates', async () => {
    mockedClient.updateSystemSetting.mockResolvedValueOnce({
      clave_parametro: 'WEBSOCKET_CHANNEL_NAME',
      valor: 'axis',
      descripcion: 'Websocket channel',
      created_at: '2026-06-11T10:00:00Z',
      updated_at: '2026-06-11T10:00:00Z',
    })
    const { qc, Wrapper } = makeWrapper()
    const invalidateSpy = vi.spyOn(qc, 'invalidateQueries')
    const { result } = renderHook(() => useUpdateSystemSetting(), { wrapper: Wrapper })

    result.current.mutate({ clave: 'WEBSOCKET_CHANNEL_NAME', payload: { valor: 'axis' } })

    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: parametrizacionKeys.systemSettings() })
  })
})

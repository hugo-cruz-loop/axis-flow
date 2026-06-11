import { describe, it, expect, vi, beforeEach } from 'vitest'
import { renderHook, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { useFormularios, useCreateFormulario, formulariosKeys } from '../api/queries'
import { formulariosClient } from '../api/formulariosClient'
import type { ReactNode } from 'react'

// Mock the client module — every hook delegates to it.
vi.mock('../api/formulariosClient', () => ({
  formulariosClient: {
    getFormulariosByEmpresa: vi.fn(),
    createFormulario: vi.fn(),
  },
}))

const mockedClient = vi.mocked(formulariosClient)

const UUID = '8c2a7d0e-5412-4c22-b5e1-88f6c5bbde93'

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

describe('formulariosKeys', () => {
  it('all starts with "formularios"', () => {
    expect(formulariosKeys.all[0]).toBe('formularios')
  })

  it('formularios(empresaId) nests under all', () => {
    expect(formulariosKeys.formularios(UUID)).toEqual(['formularios', 'formularios', UUID])
  })

  it('eventos nests under all', () => {
    const UUID2 = 'd041e2a8-0e1b-4d43-85f6-cbb18a4a5119'
    expect(formulariosKeys.eventos(UUID, UUID2)).toEqual(['formularios', 'eventos', UUID, UUID2])
  })
})

describe('useFormularios', () => {
  it('calls formulariosClient.getFormulariosByEmpresa and exposes the data', async () => {
    const data = { success: true as const, data: [] }
    mockedClient.getFormulariosByEmpresa.mockResolvedValueOnce(data)
    const { Wrapper } = makeWrapper()
    const { result } = renderHook(() => useFormularios(UUID), { wrapper: Wrapper })
    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(result.current.data).toEqual(data)
    expect(mockedClient.getFormulariosByEmpresa).toHaveBeenCalledWith(UUID, undefined)
  })

  it('passes params through to the client', async () => {
    mockedClient.getFormulariosByEmpresa.mockResolvedValueOnce({ success: true, data: [] })
    const { Wrapper } = makeWrapper()
    const params = { activo: true, page: 2, limit: 5 }
    const { result } = renderHook(() => useFormularios(UUID, params), { wrapper: Wrapper })
    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(mockedClient.getFormulariosByEmpresa).toHaveBeenCalledWith(UUID, params)
  })

  it('is disabled when empresaId is empty', async () => {
    const { Wrapper } = makeWrapper()
    const { result } = renderHook(() => useFormularios(''), { wrapper: Wrapper })
    // Disabled → should not fetch and should not be loading
    expect(result.current.isFetching).toBe(false)
    expect(mockedClient.getFormulariosByEmpresa).not.toHaveBeenCalled()
  })
})

describe('useCreateFormulario', () => {
  it('invokes formulariosClient.createFormulario and invalidates the formularios list', async () => {
    mockedClient.createFormulario.mockResolvedValueOnce({
      success: true,
      data: { id: UUID, empresa_id: UUID, nombre: 'x', activo: true },
    })
    const { qc, Wrapper } = makeWrapper()

    // Seed the cache so we can verify invalidation
    const invalidateSpy = vi.spyOn(qc, 'invalidateQueries')
    const { result } = renderHook(() => useCreateFormulario(), { wrapper: Wrapper })

    result.current.mutate({ empresa_id: UUID, nombre: 'x', activo: true })

    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(mockedClient.createFormulario.mock.calls[0]?.[0]).toEqual(
      expect.objectContaining({
        empresa_id: UUID,
        nombre: 'x',
        activo: true,
      }),
    )
    expect(invalidateSpy).toHaveBeenCalledWith(
      expect.objectContaining({
        queryKey: formulariosKeys.formularios(UUID),
      }),
    )
  })
})

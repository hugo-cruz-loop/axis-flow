import { describe, it, expect, vi, beforeEach } from 'vitest'
import { renderHook, waitFor } from '@testing-library/react'
import * as React from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { DashboardService } from '../../api/dashboard-service'
import {
  useEvaluacionesClientes,
  useEmpleadosCount,
  useActividadesRealizadas,
  useTodasAusencias,
  useServiciosLocalidad,
  useEstatusAtencionSeguimiento,
  useTotalTrabajosActivos,
  useTotalAusenciaTrabajadores
} from '../use-dashboard-queries'

// Mock the DashboardService to isolate hook state from actual API execution
vi.mock('../../api/dashboard-service', () => ({
  DashboardService: {
    getEvaluacionesClientes: vi.fn(),
    getEmpleadosCount: vi.fn(),
    getActividadesRealizadas: vi.fn(),
    getTodasAusencias: vi.fn(),
    getServiciosLocalidad: vi.fn(),
    getEstatusAtencionSeguimiento: vi.fn(),
    getTotalTrabajosActivos: vi.fn(),
    getTotalAusenciaTrabajadores: vi.fn(),
  }
}))

const createWrapper = () => {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
      },
    },
  })
  return ({ children }: { children: React.ReactNode }) => (
    React.createElement(QueryClientProvider, { client: queryClient }, children)
  )
}

beforeEach(() => {
  vi.clearAllMocks()
})

describe('React Query Dashboard Hooks', () => {
  it('useEvaluacionesClientes calls service and handles enabled parameter', async () => {
    const mockData = [{ cliente_id: 'c1', cliente_nombre: 'Acme', promedio_puntuacion: 8.8, total_evaluaciones: 2 }]
    vi.mocked(DashboardService.getEvaluacionesClientes).mockResolvedValueOnce(mockData)

    // Render with enabled=false first
    const { result, rerender } = renderHook(
      ({ id, enabled }) => useEvaluacionesClientes(id, enabled),
      {
        wrapper: createWrapper(),
        initialProps: { id: '123', enabled: false }
      }
    )

    expect(DashboardService.getEvaluacionesClientes).not.toHaveBeenCalled()
    expect(result.current.isLoading).toBe(false)

    // Rerender with enabled=true
    rerender({ id: '123', enabled: true })

    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(DashboardService.getEvaluacionesClientes).toHaveBeenCalledWith('123')
    expect(result.current.data).toEqual(mockData)
  })

  it('useEmpleadosCount calls service if id is provided', async () => {
    const mockData = { empresa_id: 123, total_empleados: 5 }
    vi.mocked(DashboardService.getEmpleadosCount).mockResolvedValueOnce(mockData)

    const { result } = renderHook(() => useEmpleadosCount('123'), {
      wrapper: createWrapper()
    })

    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(DashboardService.getEmpleadosCount).toHaveBeenCalledWith('123')
    expect(result.current.data).toEqual(mockData)
  })

  it('useActividadesRealizadas calls service if id is provided', async () => {
    const mockData = { empresa_id: 123, total_actividades_finalizadas: 10, periodo: 'month' }
    vi.mocked(DashboardService.getActividadesRealizadas).mockResolvedValueOnce(mockData)

    const { result } = renderHook(() => useActividadesRealizadas('123'), {
      wrapper: createWrapper()
    })

    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(DashboardService.getActividadesRealizadas).toHaveBeenCalledWith('123')
    expect(result.current.data).toEqual(mockData)
  })

  it('useTodasAusencias calls service if id is provided', async () => {
    const mockData = { empresa_id: 123, total_ausencias: 2, total_historico_inasistencias: 2 }
    vi.mocked(DashboardService.getTodasAusencias).mockResolvedValueOnce(mockData)

    const { result } = renderHook(() => useTodasAusencias('123'), {
      wrapper: createWrapper()
    })

    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(DashboardService.getTodasAusencias).toHaveBeenCalledWith('123')
    expect(result.current.data).toEqual(mockData)
  })

  it('useServiciosLocalidad calls service if id is provided', async () => {
    const mockData = [{ localidad_id: 'l1', localidad_nombre: 'Loc', servicio_id: 's1', servicio_nombre: 'Srv', empleados_asignados: 1 }]
    vi.mocked(DashboardService.getServiciosLocalidad).mockResolvedValueOnce(mockData)

    const { result } = renderHook(() => useServiciosLocalidad('client-1'), {
      wrapper: createWrapper()
    })

    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(DashboardService.getServiciosLocalidad).toHaveBeenCalledWith('client-1')
    expect(result.current.data).toEqual(mockData)
  })

  it('useEstatusAtencionSeguimiento calls service if id is provided', async () => {
    const mockData = { cliente_id: 'client-1', pendiente: 1, en_proceso: 2, finalizado: 3, total_tickets: 6, pendientes: 1, finalizados: 3 }
    vi.mocked(DashboardService.getEstatusAtencionSeguimiento).mockResolvedValueOnce(mockData)

    const { result } = renderHook(() => useEstatusAtencionSeguimiento('client-1'), {
      wrapper: createWrapper()
    })

    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(DashboardService.getEstatusAtencionSeguimiento).toHaveBeenCalledWith('client-1')
    expect(result.current.data).toEqual(mockData)
  })

  it('useTotalTrabajosActivos calls service if id is provided', async () => {
    const mockData = { empresa_id: 123, total_vacantes_activas: 2 }
    vi.mocked(DashboardService.getTotalTrabajosActivos).mockResolvedValueOnce(mockData)

    const { result } = renderHook(() => useTotalTrabajosActivos('123'), {
      wrapper: createWrapper()
    })

    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(DashboardService.getTotalTrabajosActivos).toHaveBeenCalledWith('123')
    expect(result.current.data).toEqual(mockData)
  })

  it('useTotalAusenciaTrabajadores calls service if id is provided', async () => {
    const mockData = { empresa_id: 123, tasa_absentismo: 5.0, dias_laborables_totales: 100, total_inasistencias: 5, empleados_afectados: 1, total_ausencias_actuales: 5 }
    vi.mocked(DashboardService.getTotalAusenciaTrabajadores).mockResolvedValueOnce(mockData)

    const { result } = renderHook(() => useTotalAusenciaTrabajadores('123'), {
      wrapper: createWrapper()
    })

    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(DashboardService.getTotalAusenciaTrabajadores).toHaveBeenCalledWith('123')
    expect(result.current.data).toEqual(mockData)
  })
})

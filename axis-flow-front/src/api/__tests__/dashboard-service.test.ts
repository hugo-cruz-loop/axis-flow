import { describe, it, expect, vi, beforeEach } from 'vitest'
import axios from 'axios'

vi.mock('@/store/authStore', () => ({
  useAuthStore: { getState: () => ({ accessToken: 'test-token' }) },
}))

vi.mock('axios', async () => {
  const actual = await vi.importActual<typeof import('axios')>('axios')
  return {
    ...actual,
    default: {
      create: vi.fn(() => mockAxiosInstance),
    },
  }
})

const mockGet = vi.fn()
const mockAxiosInstance = {
  get: mockGet,
  interceptors: {
    request: { use: vi.fn() },
    response: { use: vi.fn() },
  },
}

let DashboardService: typeof import('../dashboard-service').DashboardService

beforeEach(async () => {
  vi.clearAllMocks()
  vi.resetModules()
  const mod = await import('../dashboard-service')
  DashboardService = mod.DashboardService
})

describe('DashboardService API Client', () => {
  it('getEvaluacionesClientes calls correct endpoint and returns evaluations array', async () => {
    const mockResponse = {
      data: {
        data: {
          empresa_id: 123,
          evaluaciones: [
            {
              cliente_id: '8c2a7d0e-5412-4c22-b5e1-88f6c5bbde93',
              cliente_nombre: 'Acme Corp',
              promedio_puntuacion: 9.2,
              total_evaluaciones: 15
            }
          ]
        }
      }
    }
    mockGet.mockResolvedValueOnce(mockResponse)

    const result = await DashboardService.getEvaluacionesClientes('123')

    expect(mockGet).toHaveBeenCalledWith('/api/v1/dashboards/admin-empresa/123/evaluaciones-clientes')
    expect(result).toHaveLength(1)
    expect(result[0].cliente_nombre).toBe('Acme Corp')
    expect(result[0].promedio_puntuacion).toBe(9.2)
  })

  it('getEmpleadosCount calls correct endpoint and returns counts', async () => {
    const mockResponse = {
      data: {
        data: {
          empresa_id: 123,
          total_empleados: 50
        }
      }
    }
    mockGet.mockResolvedValueOnce(mockResponse)

    const result = await DashboardService.getEmpleadosCount('123')

    expect(mockGet).toHaveBeenCalledWith('/api/v1/dashboards/admin-empresa/123/empleados/count')
    expect(result.total_empleados).toBe(50)
  })

  it('getActividadesRealizadas calls correct endpoint and returns activities', async () => {
    const mockResponse = {
      data: {
        data: {
          empresa_id: 123,
          total_actividades_finalizadas: 150,
          periodo: 'month'
        }
      }
    }
    mockGet.mockResolvedValueOnce(mockResponse)

    const result = await DashboardService.getActividadesRealizadas('123')

    expect(mockGet).toHaveBeenCalledWith('/api/v1/dashboards/admin-empresa/123/actividades/count')
    expect(result.total_actividades_finalizadas).toBe(150)
  })

  it('getTodasAusencias calls correct endpoint and returns mapped absences', async () => {
    const mockResponse = {
      data: {
        data: {
          empresa_id: 123,
          total_ausencias: 8
        }
      }
    }
    mockGet.mockResolvedValueOnce(mockResponse)

    const result = await DashboardService.getTodasAusencias('123')

    expect(mockGet).toHaveBeenCalledWith('/api/v1/dashboards/admin-empresa/123/ausencias/count')
    expect(result.total_ausencias).toBe(8)
    expect(result.total_historico_inasistencias).toBe(8)
  })

  it('getServiciosLocalidad calls correct endpoint and returns flattened services array', async () => {
    const mockResponse = {
      data: {
        data: {
          cliente_id: '9b1b8c0d-e2f4-4321-a1b2-c3d4e5f6a7b8',
          localidades: [
            {
              localidad_id: '8c2a7d0e-5412-4c22-b5e1-88f6c5bbde93',
              localidad_nombre: 'Downtown',
              servicios: [
                {
                  servicio_id: 101,
                  servicio_nombre: 'Security',
                  empleados_asignados: 4
                }
              ]
            }
          ]
        }
      }
    }
    mockGet.mockResolvedValueOnce(mockResponse)

    const result = await DashboardService.getServiciosLocalidad('9b1b8c0d-e2f4-4321-a1b2-c3d4e5f6a7b8')

    expect(mockGet).toHaveBeenCalledWith('/api/v1/dashboards/cliente/9b1b8c0d-e2f4-4321-a1b2-c3d4e5f6a7b8/servicios-localidad')
    expect(result).toHaveLength(1)
    expect(result[0].localidad_nombre).toBe('Downtown')
    expect(result[0].servicio_id).toBe('101')
    expect(result[0].servicio_nombre).toBe('Security')
    expect(result[0].empleados_asignados).toBe(4)
  })

  it('getEstatusAtencionSeguimiento calls correct endpoint and returns mapped status', async () => {
    const mockResponse = {
      data: {
        data: {
          cliente_id: '9b1b8c0d-e2f4-4321-a1b2-c3d4e5f6a7b8',
          tickets: {
            pendiente: 3,
            en_proceso: 5,
            finalizado: 12
          },
          total_tickets: 20
        }
      }
    }
    mockGet.mockResolvedValueOnce(mockResponse)

    const result = await DashboardService.getEstatusAtencionSeguimiento('9b1b8c0d-e2f4-4321-a1b2-c3d4e5f6a7b8')

    expect(mockGet).toHaveBeenCalledWith('/api/v1/dashboards/cliente/9b1b8c0d-e2f4-4321-a1b2-c3d4e5f6a7b8/atencion-seguimiento/status')
    expect(result.pendiente).toBe(3)
    expect(result.pendientes).toBe(3)
    expect(result.finalizado).toBe(12)
    expect(result.finalizados).toBe(12)
  })

  it('getTotalTrabajosActivos calls correct endpoint and returns open jobs', async () => {
    const mockResponse = {
      data: {
        data: {
          empresa_id: 123,
          total_vacantes_activas: 6,
          vacantes: []
        }
      }
    }
    mockGet.mockResolvedValueOnce(mockResponse)

    const result = await DashboardService.getTotalTrabajosActivos('123')

    expect(mockGet).toHaveBeenCalledWith('/api/v1/dashboards/rh/123/bolsa-trabajo/vacantes-activas')
    expect(result.total_vacantes_activas).toBe(6)
  })

  it('getTotalAusenciaTrabajadores calls correct endpoint and returns absenteeism metrics', async () => {
    const mockResponse = {
      data: {
        data: {
          empresa_id: 123,
          tasa_absentismo: 3.4,
          dias_laborables_totales: 220,
          total_inasistencias: 7,
          empleados_afectados: 3
        }
      }
    }
    mockGet.mockResolvedValueOnce(mockResponse)

    const result = await DashboardService.getTotalAusenciaTrabajadores('123')

    expect(mockGet).toHaveBeenCalledWith('/api/v1/dashboards/rh/123/empleados/absentismo')
    expect(result.tasa_absentismo).toBe(3.4)
    expect(result.total_ausencias_actuales).toBe(7)
  })
})

// Suppress unused warning
void axios

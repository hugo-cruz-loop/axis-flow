import { beforeEach, describe, expect, it, vi } from 'vitest'

const mocks = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
  patch: vi.fn(),
  delete: vi.fn(),
}))

vi.mock('@/api/axiosInstance', () => ({
  default: mocks,
}))

import { parametrizacionClient, ParametrizacionApiError } from '../api/parametrizacionClient'

beforeEach(() => {
  vi.clearAllMocks()
})

describe('parametrizacionClient', () => {
  it('lists service evaluations using the OpenAPI query parameter and unwraps the success envelope', async () => {
    mocks.get.mockResolvedValueOnce({
      data: {
        success: true,
        data: [
          {
            id: 101,
            empresa_id: 12,
            servicio_id: 5,
            periodicidad_id: 2,
            activa: true,
            created_at: '2026-06-11T10:00:00Z',
            updated_at: '2026-06-11T10:00:00Z',
          },
        ],
      },
    })

    const result = await parametrizacionClient.listServiceEvaluations(12)

    expect(mocks.get).toHaveBeenCalledWith('/v1/parametrizacion/evaluacion-servicio', {
      params: { empresa_id: 12 },
    })
    expect(result[0]?.servicio_id).toBe(5)
  })

  it('creates inactive days, updates system settings, and unwraps their envelopes', async () => {
    mocks.post.mockResolvedValueOnce({
      data: {
        success: true,
        data: {
          id: 154,
          empresa_id: 12,
          fecha: '2026-12-25',
          descripcion: 'Christmas',
          created_at: '2026-06-11T10:00:00Z',
          updated_at: '2026-06-11T10:00:00Z',
        },
      },
    })
    mocks.patch.mockResolvedValueOnce({
      data: {
        success: true,
        data: {
          clave_parametro: 'WEBSOCKET_CHANNEL_NAME',
          valor: 'axis_flow_websocket_production',
          descripcion: 'Websocket channel',
          created_at: '2026-06-11T10:00:00Z',
          updated_at: '2026-06-11T10:00:00Z',
        },
      },
    })

    const inactiveDay = await parametrizacionClient.createInactiveDay({
      empresa_id: 12,
      fecha: '2026-12-25',
      descripcion: 'Christmas',
    })
    const setting = await parametrizacionClient.updateSystemSetting('WEBSOCKET_CHANNEL_NAME', {
      valor: 'axis_flow_websocket_production',
    })

    expect(mocks.post).toHaveBeenCalledWith('/v1/parametrizacion/dias-inactivos', {
      empresa_id: 12,
      fecha: '2026-12-25',
      descripcion: 'Christmas',
    })
    expect(mocks.patch).toHaveBeenCalledWith('/v1/parametrizacion/sistema/WEBSOCKET_CHANNEL_NAME', {
      valor: 'axis_flow_websocket_production',
    })
    expect(inactiveDay.id).toBe(154)
    expect(setting.clave_parametro).toBe('WEBSOCKET_CHANNEL_NAME')
  })

  it('throws a typed API error with validation details when the backend returns an error envelope', async () => {
    mocks.post.mockRejectedValueOnce({
      response: {
        status: 400,
        data: {
          success: false,
          error: {
            code: 'VALIDATION_ERROR',
            message: 'The request payload failed schema validation.',
            details: { empresa_id: ['empresa_id is required'] },
          },
        },
      },
    })

    await expect(
      parametrizacionClient.createServiceEvaluation({
        empresa_id: 0,
        servicio_id: 5,
        periodicidad_id: 2,
        activa: true,
      }),
    ).rejects.toMatchObject({
      name: 'ParametrizacionApiError',
      status: 400,
      envelope: {
        error: {
          code: 'VALIDATION_ERROR',
          details: { empresa_id: ['empresa_id is required'] },
        },
      },
    })
    expect(ParametrizacionApiError).toBeDefined()
  })
})

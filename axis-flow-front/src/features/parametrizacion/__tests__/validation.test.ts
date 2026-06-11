import { describe, expect, it } from 'vitest'
import {
  errorEnvelopeSchema,
  inactiveDayFormSchema,
  serviceEvaluationFormSchema,
  systemSettingFormSchema,
} from '../schemas/validation'

describe('parametrizacion validation schemas', () => {
  it('accepts OpenAPI numeric IDs for service evaluation forms', () => {
    const parsed = serviceEvaluationFormSchema.parse({
      empresa_id: 12,
      servicio_id: 5,
      periodicidad_id: 2,
      activa: true,
    })

    expect(parsed).toEqual({ empresa_id: 12, servicio_id: 5, periodicidad_id: 2, activa: true })
  })

  it('rejects invalid inactive day dates and overlong descriptions', () => {
    const result = inactiveDayFormSchema.safeParse({
      empresa_id: 12,
      fecha: 'not-a-date',
      descripcion: 'x'.repeat(256),
    })

    expect(result.success).toBe(false)
    expect(result.error?.issues.map((issue) => issue.path.join('.'))).toEqual(
      expect.arrayContaining(['fecha', 'descripcion']),
    )
  })

  it('validates uppercase system parameter keys and error envelopes', () => {
    expect(
      systemSettingFormSchema.safeParse({ clave_parametro: 'WEBSOCKET_CHANNEL_NAME', valor: 'axis' }).success,
    ).toBe(true)
    expect(systemSettingFormSchema.safeParse({ clave_parametro: 'lowercase', valor: 'axis' }).success).toBe(false)

    const parsedError = errorEnvelopeSchema.parse({
      success: false,
      error: {
        code: 'VALIDATION_ERROR',
        message: 'Invalid payload',
        details: { empresa_id: ['empresa_id is required'] },
      },
    })

    expect(parsedError.error.details?.empresa_id).toEqual(['empresa_id is required'])
  })
})

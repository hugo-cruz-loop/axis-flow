import { z } from 'zod'
import axiosInstance from '@/api/axiosInstance'
import {
  companyInactiveDaysSchema,
  errorEnvelopeSchema,
  inactiveDaySchema,
  inactiveDaysThresholdResponseSchema,
  personalEvaluationSchema,
  serviceEvaluationSchema,
  successEnvelopeSchema,
  systemSettingSchema,
} from '../schemas/validation'
import type {
  CompanyInactiveDays,
  ErrorEnvelope,
  InactiveDay,
  InactiveDayForm,
  InactiveDaysThreshold,
  InactiveDaysThresholdResponse,
  PersonalEvaluation,
  PersonalEvaluationForm,
  ServiceEvaluation,
  ServiceEvaluationForm,
  SystemSetting,
  SystemSettingForm,
} from '../types'

const BASE_URL = '/v1/parametrizacion'

export class ParametrizacionApiError extends Error {
  readonly status?: number
  readonly envelope: ErrorEnvelope

  constructor(envelope: ErrorEnvelope, status?: number) {
    super(envelope.error.message)
    this.name = 'ParametrizacionApiError'
    this.status = status
    this.envelope = envelope
  }
}

async function withErrorEnvelope<T>(request: Promise<{ data: unknown }>, schema: { parse: (data: unknown) => T }) {
  try {
    const response = await request
    return schema.parse(response.data)
  } catch (error) {
    const maybeResponse = error as { response?: { status?: number; data?: unknown } }
    const parsed = errorEnvelopeSchema.safeParse(maybeResponse.response?.data)
    if (parsed.success) {
      throw new ParametrizacionApiError(parsed.data, maybeResponse.response?.status)
    }
    throw error
  }
}

function unwrap<T>(envelope: { success: true; data: T }) {
  return envelope.data
}

export const parametrizacionClient = {
  async listServiceEvaluations(empresaId: number): Promise<ServiceEvaluation[]> {
    return unwrap(
      await withErrorEnvelope(
        axiosInstance.get(`${BASE_URL}/evaluacion-servicio`, { params: { empresa_id: empresaId } }),
        successEnvelopeSchema(serviceEvaluationSchema.array()),
      ),
    )
  },

  async createServiceEvaluation(payload: ServiceEvaluationForm): Promise<ServiceEvaluation> {
    return unwrap(
      await withErrorEnvelope(
        axiosInstance.post(`${BASE_URL}/evaluacion-servicio`, payload),
        successEnvelopeSchema(serviceEvaluationSchema),
      ),
    )
  },

  async listPersonalEvaluations(empresaId: number): Promise<PersonalEvaluation[]> {
    return unwrap(
      await withErrorEnvelope(
        axiosInstance.get(`${BASE_URL}/evaluacion-personal/filtrar`, { params: { empresa_id: empresaId } }),
        successEnvelopeSchema(personalEvaluationSchema.array()),
      ),
    )
  },

  async createPersonalEvaluation(payload: PersonalEvaluationForm): Promise<PersonalEvaluation> {
    return unwrap(
      await withErrorEnvelope(
        axiosInstance.post(`${BASE_URL}/evaluacion-personal`, payload),
        successEnvelopeSchema(personalEvaluationSchema),
      ),
    )
  },

  async getInactiveDays(empresaId: number): Promise<CompanyInactiveDays> {
    return unwrap(
      await withErrorEnvelope(
        axiosInstance.get(`${BASE_URL}/dias-inactivos/empresa/${empresaId}`),
        successEnvelopeSchema(companyInactiveDaysSchema),
      ),
    )
  },

  async createInactiveDay(payload: InactiveDayForm): Promise<InactiveDay> {
    return unwrap(
      await withErrorEnvelope(
        axiosInstance.post(`${BASE_URL}/dias-inactivos`, payload),
        successEnvelopeSchema(inactiveDaySchema),
      ),
    )
  },

  async deleteInactiveDay(id: number): Promise<{ message: string }> {
    return unwrap(
      await withErrorEnvelope(
        axiosInstance.delete(`${BASE_URL}/dias-inactivos/${id}`),
        successEnvelopeSchema(z.object({ message: z.string() })),
      ),
    )
  },

  async updateInactiveDaysThreshold(
    payload: InactiveDaysThreshold,
  ): Promise<InactiveDaysThresholdResponse> {
    return unwrap(
      await withErrorEnvelope(
        axiosInstance.post(`${BASE_URL}/dias-inactivos/umbral`, payload),
        successEnvelopeSchema(inactiveDaysThresholdResponseSchema),
      ),
    )
  },

  async listSystemSettings(): Promise<SystemSetting[]> {
    return unwrap(
      await withErrorEnvelope(
        axiosInstance.get(`${BASE_URL}/sistema`),
        successEnvelopeSchema(systemSettingSchema.array()),
      ),
    )
  },

  async updateSystemSetting(
    clave: string,
    payload: Pick<SystemSettingForm, 'valor'>,
  ): Promise<SystemSetting> {
    return unwrap(
      await withErrorEnvelope(
        axiosInstance.patch(`${BASE_URL}/sistema/${clave}`, payload),
        successEnvelopeSchema(systemSettingSchema),
      ),
    )
  },
}

import axios from 'axios'
import { useAuthStore } from '@/store/authStore'
import {
  formularioResponseSchema,
  formularioListResponseSchema,
  preguntaResponseSchema,
  eventoResponseSchema,
  eventoListResponseSchema,
  eventoIniciadoResponseSchema,
  respuestaCreateSchema,
  respuestaResponseSchema,
} from '../schemas/validation'
import type {
  FormularioCreate,
  FormularioResponse,
  FormularioListResponse,
  PreguntaCreate,
  PreguntaResponse,
  EventoCreate,
  EventoResponse,
  EventoListResponse,
  EventoIniciadoCreate,
  EventoIniciadoResponse,
  RespuestaCreate,
  RespuestaResponse,
} from '../types'

// Token is read from Zustand in-memory store (XSS-safe, never persisted to disk).
// Mirrors the atencionClient pattern: own axios instance, own interceptor,
// scoped baseURL so the Vite dev-proxy at /api routes everything correctly.
const formulariosAxios = axios.create({
  baseURL:
    (import.meta.env.VITE_API_BASE_URL as string | undefined)
      ? `${import.meta.env.VITE_API_BASE_URL as string}/api/v1/formularios`
      : '/api/v1/formularios',
  timeout: 30000, // multipart upload / PDF render may take longer than the default
})

formulariosAxios.interceptors.request.use((config) => {
  const token = useAuthStore.getState().accessToken
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

/**
 * Thin wrapper around formulariosAxios. Every public method validates the
 * response payload against its Zod schema — callers receive a typed value or
 * a ZodError, never an untyped blob.
 */
export const formulariosClient = {
  // — Form design —

  async createFormulario(body: FormularioCreate): Promise<FormularioResponse> {
    const res = await formulariosAxios.post('/formulario', body)
    return formularioResponseSchema.parse(res.data)
  },

  async getFormulariosByEmpresa(
    empresaId: string,
    params?: { activo?: boolean; page?: number; limit?: number },
  ): Promise<FormularioListResponse> {
    const res = await formulariosAxios.get(`/formulario/byempresa/${empresaId}`, {
      params: {
        activo: params?.activo,
        page: params?.page,
        limit: params?.limit,
      },
    })
    return formularioListResponseSchema.parse(res.data)
  },

  async createPregunta(body: PreguntaCreate): Promise<PreguntaResponse> {
    const res = await formulariosAxios.post('/pregunta', body)
    return preguntaResponseSchema.parse(res.data)
  },

  // — Evento + iniciado —

  async createEvento(body: EventoCreate): Promise<EventoResponse> {
    const res = await formulariosAxios.post('/evento', body)
    return eventoResponseSchema.parse(res.data)
  },

  async iniciarEvento(body: EventoIniciadoCreate): Promise<EventoIniciadoResponse> {
    const res = await formulariosAxios.post('/evento_iniciado', body)
    return eventoIniciadoResponseSchema.parse(res.data)
  },

  async getEventosByEmpCte(
    empId: string,
    cteId: string,
    params?: { estatus?: string; page?: number; limit?: number },
  ): Promise<EventoListResponse> {
    const res = await formulariosAxios.get(`/evento/byEmpId/${empId}/${cteId}`, {
      params: {
        estatus: params?.estatus,
        page: params?.page,
        limit: params?.limit,
      },
    })
    return eventoListResponseSchema.parse(res.data)
  },

  // — Respuesta (JSON) —

  async submitRespuestaJSON(body: RespuestaCreate): Promise<RespuestaResponse> {
    // Sanity check the body at the boundary so we never POST a malformed payload.
    respuestaCreateSchema.parse(body)
    const res = await formulariosAxios.post('/respuesta', body)
    return respuestaResponseSchema.parse(res.data)
  },

  /**
   * Multipart submission for photo evidence. The current backend returns 501
   * (MULTIPART_UPLOAD_NOT_IMPLEMENTED) because the storage backend wired in
   * PR-6 is not yet connected to the multipart handler. We still send the
   * multipart envelope so the call signature is correct, and we surface the
   * 501 as a user-friendly Error so the UI can present it as a known
   * limitation rather than a 500.
   *
   * TODO(PR-after-7): the storage wiring is owned by the backend; once the
   * multipart handler is plumbed, remove the 501 → friendly error mapping
   * and let the success path return a RespuestaResponse.
   */
  async submitRespuestaMultipart(form: FormData): Promise<RespuestaResponse> {
    try {
      const res = await formulariosAxios.post('/respuesta', form, {
        headers: { 'Content-Type': 'multipart/form-data' },
      })
      return respuestaResponseSchema.parse(res.data)
    } catch (err: unknown) {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const status = (err as any)?.response?.status
      if (status === 501) {
        throw new Error(
          'Photo upload via mobile camera is not yet enabled on the server. ' +
            'The text portion of your response can still be submitted.',
          { cause: err },
        )
      }
      throw err
    }
  },

  /**
   * Download a PDF report for a specific pregunta's respuestas. Returns the
   * raw Blob — the caller is responsible for triggering a browser download
   * (e.g. via `URL.createObjectURL(blob)` + an anchor click).
   */
  async downloadReportePDF(preguntaId: string): Promise<Blob> {
    const res = await formulariosAxios.get(
      `/respuesta/reporte/pregunta/${preguntaId}/pdf`,
      { responseType: 'blob' },
    )
    return res.data as Blob
  },
}

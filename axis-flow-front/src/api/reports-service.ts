import axios from 'axios'
import { useAuthStore } from '@/store/authStore'
import type {
  EvidenciasQueryParams,
  AsistenciasQueryParams,
  ReverseGeocodingInput,
} from '@/schemas/reports-schemas'

// ─── Types ───────────────────────────────────────────────────────────────────

export interface Evidencia {
  asignacion_id: string
  actividad_id: string
  empleado_nombre: string
  actividad_descripcion: string
  fecha_ejecucion: string
  latitud: number
  longitud: number
  evidencias: string[]
}

export interface AttendanceRecord {
  id: string
  empleado_nombre: string
  empleado_codigo: string
  clock_in: string
  clock_out: string | null
  status: 'IN_TIME' | 'LATE' | 'ABSENT' | 'EXCUSED'
  delay_minutes: number
  latitud: number
  longitud: number
}

export interface GraficaStat {
  week: string
  total: number
  compliant: number
}

export interface IncidenteCount {
  status: string
  count: number
}

export interface GeocodingResult {
  latitud: number
  longitud: number
  direccion: string
  cached: boolean
}

export interface PaginatedMeta {
  page: number
  limit: number
  total_records: number
  total_pages: number
}

// ─── Axios client ─────────────────────────────────────────────────────────────

// Token is read from Zustand in-memory store (XSS-safe, never persisted to disk).
const reportsClient = axios.create({
  baseURL: (import.meta.env.VITE_API_BASE_URL as string | undefined)
    ? `${import.meta.env.VITE_API_BASE_URL as string}/api/v1/reports`
    : '/api/v1/reports',
  timeout: 15000,
})

reportsClient.interceptors.request.use((config) => {
  const token = useAuthStore.getState().accessToken
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

// ─── Helpers ──────────────────────────────────────────────────────────────────

function buildParams(raw: Record<string, unknown>): URLSearchParams {
  const params = new URLSearchParams()
  for (const [key, value] of Object.entries(raw)) {
    if (value === undefined || value === null) continue
    if (typeof value === 'object') {
      // Flatten nested objects (e.g. dateRange)
      for (const [nestedKey, nestedValue] of Object.entries(
        value as Record<string, unknown>,
      )) {
        if (nestedValue !== undefined && nestedValue !== null) {
          params.set(`${key}.${nestedKey}`, String(nestedValue))
        }
      }
    } else {
      params.set(key, String(value))
    }
  }
  return params
}

// ─── API methods ──────────────────────────────────────────────────────────────

async function getEvidencias(
  params: EvidenciasQueryParams,
): Promise<{ data: Evidencia[]; meta: PaginatedMeta }> {
  const response = await reportsClient.get<{ data: Evidencia[]; meta: PaginatedMeta }>(
    '/evidencias',
    { params: buildParams(params as unknown as Record<string, unknown>) },
  )
  return response.data
}

async function getAsistencias(
  params: AsistenciasQueryParams,
): Promise<{ data: AttendanceRecord[]; meta: PaginatedMeta }> {
  const response = await reportsClient.get<{ data: AttendanceRecord[]; meta: PaginatedMeta }>(
    '/asistencias',
    { params: buildParams(params as unknown as Record<string, unknown>) },
  )
  return response.data
}

async function getGraficaEvidencia(
  clientId?: string,
): Promise<{ data: GraficaStat[] }> {
  const response = await reportsClient.get<{ data: GraficaStat[] }>('/grafica-evidencia', {
    params: clientId ? { clientId } : undefined,
  })
  return response.data
}

async function getCountIncidentes(): Promise<{ data: IncidenteCount[] }> {
  const response = await reportsClient.get<{ data: IncidenteCount[] }>('/count-incidentes')
  return response.data
}

async function reverseGeocode(
  input: ReverseGeocodingInput,
): Promise<{ data: GeocodingResult }> {
  const response = await reportsClient.get<{ data: GeocodingResult }>('/reverse-geocode', {
    params: { latitud: input.latitud, longitud: input.longitud },
  })
  return response.data
}

export const ReportsService = {
  getEvidencias,
  getAsistencias,
  getGraficaEvidencia,
  getCountIncidentes,
  reverseGeocode,
}

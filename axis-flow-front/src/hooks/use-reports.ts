import { keepPreviousData, useMutation, useQuery } from '@tanstack/react-query'
import { ReportsService } from '@/api/reports-service'
import type {
  EvidenciasQueryParams,
  AsistenciasQueryParams,
  IncidentDashboardQueryParams,
  ReverseGeocodingInput,
} from '@/schemas/reports-schemas'
import type { AxiosError } from 'axios'

// ─── Constants ────────────────────────────────────────────────────────────────

const MIN_5 = 1000 * 60 * 5
const MIN_3 = 1000 * 60 * 3
const MIN_10 = 1000 * 60 * 10

function noRetryOnAuthError(failureCount: number, error: unknown): boolean {
  const status = (error as AxiosError)?.response?.status
  if (status === 401 || status === 403) return false
  return failureCount < 3
}

// ─── Hooks ────────────────────────────────────────────────────────────────────

export function useEvidencias(params: EvidenciasQueryParams, enabled = true) {
  return useQuery({
    queryKey: ['reports', 'evidencias', params],
    queryFn: () => ReportsService.getEvidencias(params),
    enabled,
    staleTime: MIN_5,
    placeholderData: keepPreviousData,
    retry: noRetryOnAuthError,
    refetchOnWindowFocus: false,
  })
}

export function useAsistencias(params: AsistenciasQueryParams) {
  return useQuery({
    queryKey: ['reports', 'asistencias', params],
    queryFn: () => ReportsService.getAsistencias(params),
    staleTime: MIN_3,
    placeholderData: keepPreviousData,
    retry: noRetryOnAuthError,
    refetchOnWindowFocus: false,
  })
}

export function useIncidentMetrics(params: IncidentDashboardQueryParams) {
  return useQuery({
    queryKey: ['reports', 'incident-metrics', params],
    queryFn: () => ReportsService.getCountIncidentes(),
    staleTime: MIN_10,
    retry: noRetryOnAuthError,
    refetchOnWindowFocus: false,
  })
}

export function useIncidentWeeklyTrend(params: IncidentDashboardQueryParams) {
  return useQuery({
    queryKey: ['reports', 'incident-trend', params],
    queryFn: () => ReportsService.getGraficaEvidencia(params.clientId),
    staleTime: MIN_10,
    retry: noRetryOnAuthError,
    refetchOnWindowFocus: false,
  })
}

export function useReverseGeocoding() {
  return useMutation({
    mutationFn: (input: ReverseGeocodingInput) => ReportsService.reverseGeocode(input),
  })
}

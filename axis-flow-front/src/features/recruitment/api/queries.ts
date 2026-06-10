import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { recruitmentClient } from '@/api/recruitmentClient'
import type {
  Trabajo,
  Postulacion,
  Evaluacion,
  PaginatedResponse,
  PipelineStats,
} from '../types'

// — Key factory —

export const recruitmentKeys = {
  allJobs: ['recruitment', 'jobs'] as const,
  companyJobs: (empresaId: string) => ['recruitment', 'jobs', 'company', empresaId] as const,
  jobStats: (trabajoId: string) => ['recruitment', 'jobs', 'stats', trabajoId] as const,
  evaluation: (postulacionId: string) =>
    ['recruitment', 'evaluation', postulacionId] as const,
}

const QUERY_OPTIONS = {
  staleTime: 5 * 60 * 1000,
  gcTime: 15 * 60 * 1000,
  refetchOnWindowFocus: false,
}

// — Query hooks —

export function useActiveJobs(search?: string, page = 1) {
  return useQuery({
    queryKey: [...recruitmentKeys.allJobs, 'active', search, page],
    queryFn: () =>
      recruitmentClient
        .get<PaginatedResponse<Trabajo>>('/trabajo/activeJobs', {
          params: { search, page },
        })
        .then((r) => r.data),
    ...QUERY_OPTIONS,
  })
}

export function useRecentJobs(page = 1) {
  return useQuery({
    queryKey: [...recruitmentKeys.allJobs, 'recent', page],
    queryFn: () =>
      recruitmentClient
        .get<PaginatedResponse<Trabajo>>('/trabajo/recent', { params: { page } })
        .then((r) => r.data),
    ...QUERY_OPTIONS,
  })
}

export function useJobsByEmpresa(empresaId: string, filter?: string, page = 1) {
  return useQuery({
    queryKey: [...recruitmentKeys.companyJobs(empresaId), filter, page],
    queryFn: () =>
      recruitmentClient
        .get<PaginatedResponse<Trabajo>>(`/trabajo/by-empresa/${empresaId}`, {
          params: { filter, page },
        })
        .then((r) => r.data),
    enabled: !!empresaId,
    ...QUERY_OPTIONS,
  })
}

export function usePostulacionStats(trabajoId: string) {
  return useQuery({
    queryKey: recruitmentKeys.jobStats(trabajoId),
    queryFn: () =>
      recruitmentClient
        .get<PipelineStats>(`/postulacion/by-trabajo-stats/${trabajoId}`)
        .then((r) => r.data),
    enabled: !!trabajoId,
    ...QUERY_OPTIONS,
  })
}

export function useEvaluacion(postulacionId: string) {
  return useQuery({
    queryKey: recruitmentKeys.evaluation(postulacionId),
    queryFn: () =>
      recruitmentClient
        .get<Evaluacion>(`/evaluacion/by-postulacion/${postulacionId}`)
        .then((r) => r.data),
    enabled: !!postulacionId,
    ...QUERY_OPTIONS,
  })
}

// — Mutation hooks —

export function useCreateTrabajo() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: Omit<Trabajo, 'id' | 'created_at' | 'updated_at'>) =>
      recruitmentClient.post<Trabajo>('/trabajo', data).then((r) => r.data),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: recruitmentKeys.allJobs })
    },
  })
}

export function useSwitchEstatus() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, empresaId }: { id: string; empresaId: string }) =>
      recruitmentClient
        .patch<Trabajo>(`/trabajo/switch/${id}`)
        .then((r) => r.data),
    onSuccess: (_data, { empresaId }) => {
      void qc.invalidateQueries({ queryKey: recruitmentKeys.allJobs })
      void qc.invalidateQueries({ queryKey: recruitmentKeys.companyJobs(empresaId) })
    },
  })
}

export function useApplyToJob() {
  return useMutation({
    mutationFn: (formData: FormData) =>
      recruitmentClient
        .post<Postulacion>('/postulacion/apply', formData, {
          headers: { 'Content-Type': 'multipart/form-data' },
        })
        .then((r) => r.data),
  })
}

export function useUpdateCandidateStatus() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, estatus }: { id: string; estatus: number }) =>
      recruitmentClient
        .patch<Postulacion>(`/postulacion/status/${id}`, { estatus })
        .then((r) => r.data),
    onSuccess: (data) => {
      void qc.invalidateQueries({
        queryKey: recruitmentKeys.jobStats(data.trabajo_id),
      })
    },
  })
}

export function useSubmitEvaluation() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: {
      postulacion_id: string
      puntualidad: number
      cortesia: number
      soft_skills: number
      comentarios?: string
    }) =>
      recruitmentClient.post<Evaluacion>('/evaluacion', data).then((r) => r.data),
    onSuccess: (data) => {
      void qc.invalidateQueries({
        queryKey: recruitmentKeys.evaluation(data.postulacion_id),
      })
    },
  })
}

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { parametrizacionClient } from './parametrizacionClient'
import type {
  CompanyInactiveDays,
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

export const parametrizacionKeys = {
  all: ['parametrizacion'] as const,
  serviceEvaluations: (empresaId: number) =>
    [...parametrizacionKeys.all, 'service-evaluations', empresaId] as const,
  personalEvaluations: (empresaId: number) =>
    [...parametrizacionKeys.all, 'personal-evaluations', empresaId] as const,
  inactiveDays: (empresaId: number) => [...parametrizacionKeys.all, 'inactive-days', empresaId] as const,
  systemSettings: () => [...parametrizacionKeys.all, 'system-settings'] as const,
}

const QUERY_OPTIONS = {
  staleTime: 5 * 60 * 1000,
  gcTime: 15 * 60 * 1000,
  refetchOnWindowFocus: false,
}

export function useServiceEvaluations(empresaId: number) {
  return useQuery<ServiceEvaluation[]>({
    queryKey: parametrizacionKeys.serviceEvaluations(empresaId),
    queryFn: () => parametrizacionClient.listServiceEvaluations(empresaId),
    enabled: empresaId > 0,
    ...QUERY_OPTIONS,
  })
}

export function useCreateServiceEvaluation() {
  const queryClient = useQueryClient()
  return useMutation<ServiceEvaluation, Error, ServiceEvaluationForm>({
    mutationFn: parametrizacionClient.createServiceEvaluation,
    onSuccess: (_data, variables) => {
      void queryClient.invalidateQueries({ queryKey: parametrizacionKeys.serviceEvaluations(variables.empresa_id) })
    },
  })
}

export function usePersonalEvaluations(empresaId: number) {
  return useQuery<PersonalEvaluation[]>({
    queryKey: parametrizacionKeys.personalEvaluations(empresaId),
    queryFn: () => parametrizacionClient.listPersonalEvaluations(empresaId),
    enabled: empresaId > 0,
    ...QUERY_OPTIONS,
  })
}

export function useCreatePersonalEvaluation() {
  const queryClient = useQueryClient()
  return useMutation<PersonalEvaluation, Error, PersonalEvaluationForm>({
    mutationFn: parametrizacionClient.createPersonalEvaluation,
    onSuccess: (_data, variables) => {
      void queryClient.invalidateQueries({ queryKey: parametrizacionKeys.personalEvaluations(variables.empresa_id) })
    },
  })
}

export function useInactiveDays(empresaId: number) {
  return useQuery<CompanyInactiveDays>({
    queryKey: parametrizacionKeys.inactiveDays(empresaId),
    queryFn: () => parametrizacionClient.getInactiveDays(empresaId),
    enabled: empresaId > 0,
    ...QUERY_OPTIONS,
  })
}

export function useCreateInactiveDay() {
  const queryClient = useQueryClient()
  return useMutation<InactiveDay, Error, InactiveDayForm>({
    mutationFn: parametrizacionClient.createInactiveDay,
    onSuccess: (_data, variables) => {
      void queryClient.invalidateQueries({ queryKey: parametrizacionKeys.inactiveDays(variables.empresa_id) })
    },
  })
}

export function useDeleteInactiveDay(empresaId: number) {
  const queryClient = useQueryClient()
  return useMutation<{ message: string }, Error, number>({
    mutationFn: parametrizacionClient.deleteInactiveDay,
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: parametrizacionKeys.inactiveDays(empresaId) })
    },
  })
}

export function useUpdateInactiveDaysThreshold() {
  const queryClient = useQueryClient()
  return useMutation<InactiveDaysThresholdResponse, Error, InactiveDaysThreshold>({
    mutationFn: parametrizacionClient.updateInactiveDaysThreshold,
    onSuccess: (_data, variables) => {
      void queryClient.invalidateQueries({ queryKey: parametrizacionKeys.inactiveDays(variables.empresa_id) })
    },
  })
}

export function useSystemSettings() {
  return useQuery<SystemSetting[]>({
    queryKey: parametrizacionKeys.systemSettings(),
    queryFn: parametrizacionClient.listSystemSettings,
    ...QUERY_OPTIONS,
  })
}

export function useUpdateSystemSetting() {
  const queryClient = useQueryClient()
  return useMutation<SystemSetting, Error, { clave: string; payload: Pick<SystemSettingForm, 'valor'> }>({
    mutationFn: ({ clave, payload }) => parametrizacionClient.updateSystemSetting(clave, payload),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: parametrizacionKeys.systemSettings() })
    },
  })
}

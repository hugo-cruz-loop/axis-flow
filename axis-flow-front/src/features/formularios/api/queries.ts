import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { formulariosClient } from './formulariosClient'
import type {
  FormularioCreate,
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

// — Key factory (mirror atencionKeys convention) —

export const formulariosKeys = {
  all: ['formularios'] as const,
  formularios: (empresaId: string) =>
    [...formulariosKeys.all, 'formularios', empresaId] as const,
  formulariosList: (empresaId: string, params?: { activo?: boolean; page?: number; limit?: number }) =>
    [...formulariosKeys.formularios(empresaId), params] as const,
  eventos: (empId: string, cteId: string) =>
    [...formulariosKeys.all, 'eventos', empId, cteId] as const,
  eventosList: (
    empId: string,
    cteId: string,
    params?: { estatus?: string; page?: number; limit?: number },
  ) => [...formulariosKeys.eventos(empId, cteId), params] as const,
}

const QUERY_OPTIONS = {
  staleTime: 5 * 60 * 1000,
  gcTime: 15 * 60 * 1000,
  refetchOnWindowFocus: false,
}

// — Form hooks —

export function useFormularios(
  empresaId: string,
  params?: { activo?: boolean; page?: number; limit?: number },
) {
  return useQuery<FormularioListResponse>({
    queryKey: formulariosKeys.formulariosList(empresaId, params),
    queryFn: () => formulariosClient.getFormulariosByEmpresa(empresaId, params),
    enabled: !!empresaId,
    ...QUERY_OPTIONS,
  })
}

export function useCreateFormulario() {
  const qc = useQueryClient()
  return useMutation<FormularioResponse, Error, FormularioCreate>({
    mutationFn: formulariosClient.createFormulario,
    onSuccess: (_, vars) => {
      void qc.invalidateQueries({ queryKey: formulariosKeys.formularios(vars.empresa_id) })
    },
  })
}

export function useCreatePregunta() {
  const qc = useQueryClient()
  return useMutation<PreguntaResponse, Error, PreguntaCreate>({
    mutationFn: formulariosClient.createPregunta,
    onSuccess: () => {
      // No specific list key to invalidate — formularios list is already
      // invalidated when the form is created. Pregunta reads happen via the
      // builder page local state.
      void qc.invalidateQueries({ queryKey: formulariosKeys.all })
    },
  })
}

// — Evento hooks —

export function useEventosByEmpCte(
  empId: string,
  cteId: string,
  params?: { estatus?: string; page?: number; limit?: number },
) {
  return useQuery<EventoListResponse>({
    queryKey: formulariosKeys.eventosList(empId, cteId, params),
    queryFn: () => formulariosClient.getEventosByEmpCte(empId, cteId, params),
    enabled: !!empId && !!cteId,
    ...QUERY_OPTIONS,
  })
}

export function useCreateEvento(empId: string, cteId: string) {
  const qc = useQueryClient()
  return useMutation<EventoResponse, Error, EventoCreate>({
    mutationFn: formulariosClient.createEvento,
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: formulariosKeys.eventos(empId, cteId) })
    },
  })
}

export function useIniciarEvento() {
  return useMutation<EventoIniciadoResponse, Error, EventoIniciadoCreate>({
    mutationFn: formulariosClient.iniciarEvento,
  })
}

// — Respuesta hooks —

export function useSubmitRespuesta() {
  return useMutation<RespuestaResponse, Error, RespuestaCreate>({
    mutationFn: formulariosClient.submitRespuestaJSON,
  })
}

export function useSubmitRespuestaMultipart() {
  return useMutation<RespuestaResponse, Error, FormData>({
    mutationFn: formulariosClient.submitRespuestaMultipart,
  })
}

export function useDownloadReporte() {
  return useMutation<Blob, Error, string>({
    mutationFn: formulariosClient.downloadReportePDF,
  })
}

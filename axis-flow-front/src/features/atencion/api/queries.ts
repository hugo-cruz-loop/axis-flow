import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { atencionClient } from '@/api/atencionClient'
import type {
  SolicitudQueja,
  RespuestaQueja,
  TicketServicio,
  RespuestaServicio,
  IncidenciaSupervisor,
  TicketStats,
  PaginatedAtencion,
} from '../types'
import type { QuejaInput, MensajeInput, TicketInput, IncidenciaInput } from '../schemas/validation'

// — Key factory —

export const atencionKeys = {
  quejas: (empresaId: string) => ['atencion', 'quejas', empresaId] as const,
  queja: (id: string) => ['atencion', 'queja', id] as const,
  mensajesQueja: (id: string) => ['atencion', 'mensajes-queja', id] as const,
  tickets: (clienteId: string) => ['atencion', 'tickets', clienteId] as const,
  mensajesTicket: (id: string) => ['atencion', 'mensajes-ticket', id] as const,
  ticketStats: (empresaId: string) => ['atencion', 'ticket-stats', empresaId] as const,
  incidencias: (empresaId: string) => ['atencion', 'incidencias', empresaId] as const,
}

const QUERY_OPTIONS = {
  staleTime: 5 * 60 * 1000,
  gcTime: 15 * 60 * 1000,
  refetchOnWindowFocus: false,
}

// — Query hooks —

export function useQuejasByEmpresa(empresaId: string, estatus?: number, page = 1) {
  return useQuery({
    queryKey: [...atencionKeys.quejas(empresaId), estatus, page],
    queryFn: () =>
      atencionClient
        .get<PaginatedAtencion<SolicitudQueja>>(`/queja/by-empresa/${empresaId}`, {
          params: { estatus, page },
        })
        .then((r) => r.data),
    enabled: !!empresaId,
    ...QUERY_OPTIONS,
  })
}

export function useQueja(id: string) {
  return useQuery({
    queryKey: atencionKeys.queja(id),
    queryFn: () =>
      atencionClient.get<SolicitudQueja>(`/queja/${id}`).then((r) => r.data),
    enabled: !!id,
    ...QUERY_OPTIONS,
  })
}

export function useMensajesQueja(solicitudId: string) {
  return useQuery({
    queryKey: atencionKeys.mensajesQueja(solicitudId),
    queryFn: () =>
      atencionClient
        .get<RespuestaQueja[]>(`/queja/${solicitudId}/mensajes`)
        .then((r) => r.data),
    enabled: !!solicitudId,
    ...QUERY_OPTIONS,
  })
}

export function useCreateQueja() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: QuejaInput) =>
      atencionClient.post<SolicitudQueja>('/queja', data).then((r) => r.data),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['atencion', 'quejas'] })
    },
  })
}

export function useCreateMensajeQueja(solicitudId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: MensajeInput) =>
      atencionClient
        .post<RespuestaQueja>(`/queja/${solicitudId}/mensajes`, data)
        .then((r) => r.data),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: atencionKeys.mensajesQueja(solicitudId) })
    },
  })
}

export function useTicketsByCliente(clienteId: string, page = 1) {
  return useQuery({
    queryKey: [...atencionKeys.tickets(clienteId), page],
    queryFn: () =>
      atencionClient
        .get<PaginatedAtencion<TicketServicio>>(`/ticket/by-cliente/${clienteId}`, {
          params: { page },
        })
        .then((r) => r.data),
    enabled: !!clienteId,
    ...QUERY_OPTIONS,
  })
}

export function useMensajesTicket(ticketId: string) {
  return useQuery({
    queryKey: atencionKeys.mensajesTicket(ticketId),
    queryFn: () =>
      atencionClient
        .get<RespuestaServicio[]>(`/ticket/${ticketId}/mensajes`)
        .then((r) => r.data),
    enabled: !!ticketId,
    ...QUERY_OPTIONS,
  })
}

export function useCreateTicket() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: TicketInput) =>
      atencionClient.post<TicketServicio>('/ticket', data).then((r) => r.data),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['atencion', 'tickets'] })
    },
  })
}

export function useCreateMensajeTicket(ticketId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: MensajeInput) =>
      atencionClient
        .post<RespuestaServicio>(`/ticket/${ticketId}/mensajes`, data)
        .then((r) => r.data),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: atencionKeys.mensajesTicket(ticketId) })
    },
  })
}

export function useUpdateTicketEstatus(ticketId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (estatus: 1 | 2 | 3) =>
      atencionClient
        .patch<TicketServicio>(`/ticket/${ticketId}/estatus`, { estatus })
        .then((r) => r.data),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['atencion', 'tickets'] })
    },
  })
}

export function useTicketStats(empresaId: string) {
  return useQuery({
    queryKey: atencionKeys.ticketStats(empresaId),
    queryFn: () =>
      atencionClient
        .get<TicketStats>(`/ticket/stats/${empresaId}`)
        .then((r) => r.data),
    enabled: !!empresaId,
    ...QUERY_OPTIONS,
  })
}

export function useIncidenciasByEmpresa(empresaId: string, page = 1) {
  return useQuery({
    queryKey: [...atencionKeys.incidencias(empresaId), page],
    queryFn: () =>
      atencionClient
        .get<PaginatedAtencion<IncidenciaSupervisor>>(
          `/incidencia/by-empresa/${empresaId}`,
          { params: { page } },
        )
        .then((r) => r.data),
    enabled: !!empresaId,
    ...QUERY_OPTIONS,
  })
}

export function useCreateIncidencia() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: IncidenciaInput) =>
      atencionClient
        .post<IncidenciaSupervisor>('/incidencia', data)
        .then((r) => r.data),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['atencion', 'incidencias'] })
    },
  })
}

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  createCliente,
  getCliente,
  listClientes,
  getClienteStats,
  patchEstatus,
  createFactura,
  getFactura,
  createPresupuesto,
  getPresupuesto,
  createCalendario,
  getCalendario,
  createLocalidad,
  listLocalidades,
  getLocalidad,
  listServicios,
  listHorarios,
  listHerramientas,
  listActividades,
  listEvaluaciones,
  createEvaluacion,
} from '@/api/clientesClient'
import type {
  CreateClienteRequest,
  CreateFacturaRequest,
  CreatePresupuestoRequest,
  CreateCalendarioRequest,
  CreateLocalidadRequest,
  CreateEvaluacionRequest,
} from '@/types/clientes'

const QUERY_OPTIONS = {
  staleTime: 10 * 60 * 1000,
  gcTime: 30 * 60 * 1000,
  refetchOnWindowFocus: false,
}

// — Queries —

export const useCliente = (id: string) =>
  useQuery({ queryKey: ['cliente', id], queryFn: () => getCliente(id), ...QUERY_OPTIONS })

export const useClienteList = (empresaId: number, page?: number, size?: number) =>
  useQuery({
    queryKey: ['clientes', empresaId, page, size],
    queryFn: () => listClientes(empresaId, page, size),
    enabled: !!empresaId,
    ...QUERY_OPTIONS,
  })

export const useClienteStats = (empresaId: number) =>
  useQuery({
    queryKey: ['clienteStats', empresaId],
    queryFn: () => getClienteStats(empresaId),
    enabled: !!empresaId,
    ...QUERY_OPTIONS,
  })

export const useFactura = (clienteId: string) =>
  useQuery({
    queryKey: ['factura', clienteId],
    queryFn: () => getFactura(clienteId),
    enabled: !!clienteId,
    ...QUERY_OPTIONS,
  })

export const usePresupuesto = (clienteId: string) =>
  useQuery({
    queryKey: ['presupuesto', clienteId],
    queryFn: () => getPresupuesto(clienteId),
    enabled: !!clienteId,
    ...QUERY_OPTIONS,
  })

export const useCalendario = (clienteId: string) =>
  useQuery({
    queryKey: ['calendario', clienteId],
    queryFn: () => getCalendario(clienteId),
    enabled: !!clienteId,
    ...QUERY_OPTIONS,
  })

export const useLocalidades = (clienteId: string) =>
  useQuery({
    queryKey: ['localidades', clienteId],
    queryFn: () => listLocalidades(clienteId),
    enabled: !!clienteId,
    ...QUERY_OPTIONS,
  })

export const useLocalidad = (localidadId: string) =>
  useQuery({
    queryKey: ['localidad', localidadId],
    queryFn: () => getLocalidad(localidadId),
    enabled: !!localidadId,
    ...QUERY_OPTIONS,
  })

export const useServicios = (localidadId: string) =>
  useQuery({
    queryKey: ['servicios', localidadId],
    queryFn: () => listServicios(localidadId),
    enabled: !!localidadId,
    ...QUERY_OPTIONS,
  })

export const useHorarios = (localidadId: string) =>
  useQuery({
    queryKey: ['horarios', localidadId],
    queryFn: () => listHorarios(localidadId),
    enabled: !!localidadId,
    ...QUERY_OPTIONS,
  })

export const useHerramientas = (localidadId: string) =>
  useQuery({
    queryKey: ['herramientas', localidadId],
    queryFn: () => listHerramientas(localidadId),
    enabled: !!localidadId,
    ...QUERY_OPTIONS,
  })

export const useActividades = (localidadId: string) =>
  useQuery({
    queryKey: ['actividades', localidadId],
    queryFn: () => listActividades(localidadId),
    enabled: !!localidadId,
    ...QUERY_OPTIONS,
  })

export const useEvaluaciones = (clienteId: string) =>
  useQuery({
    queryKey: ['evaluaciones', clienteId],
    queryFn: () => listEvaluaciones(clienteId),
    enabled: !!clienteId,
    ...QUERY_OPTIONS,
  })

// — Mutations —

export const useCreateCliente = () => {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (req: CreateClienteRequest) => createCliente(req),
    onSuccess: (data) => {
      queryClient.invalidateQueries({ queryKey: ['clientes', data.empresa_id] })
    },
  })
}

export const usePatchEstatus = () => {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ id, estatus }: { id: string; estatus: number }) => patchEstatus(id, estatus),
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({ queryKey: ['cliente', variables.id] })
    },
  })
}

export const useCreateFactura = () => {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ clienteId, req }: { clienteId: string; req: CreateFacturaRequest }) =>
      createFactura(clienteId, req),
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({ queryKey: ['factura', variables.clienteId] })
    },
  })
}

export const useCreatePresupuesto = () => {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ clienteId, req }: { clienteId: string; req: CreatePresupuestoRequest }) =>
      createPresupuesto(clienteId, req),
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({ queryKey: ['presupuesto', variables.clienteId] })
    },
  })
}

export const useCreateCalendario = () => {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ clienteId, req }: { clienteId: string; req: CreateCalendarioRequest }) =>
      createCalendario(clienteId, req),
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({ queryKey: ['calendario', variables.clienteId] })
    },
  })
}

export const useCreateLocalidad = () => {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ clienteId, req }: { clienteId: string; req: CreateLocalidadRequest }) =>
      createLocalidad(clienteId, req),
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({ queryKey: ['localidades', variables.clienteId] })
    },
  })
}

export const useCreateEvaluacion = () => {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ clienteId, req }: { clienteId: string; req: CreateEvaluacionRequest }) =>
      createEvaluacion(clienteId, req),
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({ queryKey: ['evaluaciones', variables.clienteId] })
    },
  })
}

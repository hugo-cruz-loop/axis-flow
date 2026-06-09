import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  createEmpleado,
  getEmpleado,
  updateEmpleado,
  softDeleteEmpleado,
  listEmpleados,
  getUbicacion,
  upsertUbicacion,
  getAdicionales,
  upsertAdicionales,
  getDocumentos,
  uploadDocumento,
  getKPIComplete,
  getKPILack,
  getKPIPendiente,
  getAsistencias,
  getInasistencias,
  aprobarInasistencia,
  getFotologin,
  uploadFotologin,
  getDevices,
} from '@/api/empleadosClient'
import type {
  CreateEmpleadoRequest,
  TipoDocumento,
  EmpleadoKPIs,
  Ubicacion,
  Adicionales,
} from '@/types/empleados'

const QUERY_OPTIONS = {
  staleTime: 5 * 60 * 1000,
  gcTime: 30 * 60 * 1000,
  refetchOnWindowFocus: false,
}

// — Queries —

export const useEmpleados = (empresaId: number, page?: number, size?: number) =>
  useQuery({
    queryKey: ['empleados', empresaId, page, size],
    queryFn: () => listEmpleados(empresaId, page, size),
    enabled: !!empresaId,
    ...QUERY_OPTIONS,
  })

export const useEmpleado = (numEmpleado: number) =>
  useQuery({
    queryKey: ['empleado', numEmpleado],
    queryFn: () => getEmpleado(numEmpleado),
    enabled: !!numEmpleado,
    ...QUERY_OPTIONS,
  })

export const useUbicacion = (numEmpleado: number) =>
  useQuery({
    queryKey: ['ubicacion', numEmpleado],
    queryFn: () => getUbicacion(numEmpleado),
    enabled: !!numEmpleado,
    ...QUERY_OPTIONS,
  })

export const useAdicionales = (numEmpleado: number) =>
  useQuery({
    queryKey: ['adicionales', numEmpleado],
    queryFn: () => getAdicionales(numEmpleado),
    enabled: !!numEmpleado,
    ...QUERY_OPTIONS,
  })

export const useDocumentos = (numEmpleado: number) =>
  useQuery({
    queryKey: ['documentos', numEmpleado],
    queryFn: () => getDocumentos(numEmpleado),
    enabled: !!numEmpleado,
    ...QUERY_OPTIONS,
  })

export const useAsistencias = (empresaId: number, empleadoId?: number) =>
  useQuery({
    queryKey: ['asistencias', empresaId, empleadoId],
    queryFn: () => getAsistencias(empresaId, empleadoId),
    enabled: !!empresaId,
    ...QUERY_OPTIONS,
  })

export const useInasistencias = (empleadoId: number, empresaId: number) =>
  useQuery({
    queryKey: ['inasistencias', empleadoId, empresaId],
    queryFn: () => getInasistencias(empleadoId, empresaId),
    enabled: !!empleadoId && !!empresaId,
    ...QUERY_OPTIONS,
  })

export const useFotologin = (numEmpleado: number, empresaId: number) =>
  useQuery({
    queryKey: ['fotologin', numEmpleado, empresaId],
    queryFn: () => getFotologin(numEmpleado, empresaId),
    enabled: !!numEmpleado && !!empresaId,
    ...QUERY_OPTIONS,
  })

export const useDevices = (numEmpleado: number, empresaId: number) =>
  useQuery({
    queryKey: ['devices', numEmpleado, empresaId],
    queryFn: () => getDevices(numEmpleado, empresaId),
    enabled: !!numEmpleado && !!empresaId,
    ...QUERY_OPTIONS,
  })

export const useEmpleadoKPIs = (empresaId: number) => {
  const complete = useQuery({
    queryKey: ['kpi-complete', empresaId],
    queryFn: () => getKPIComplete(empresaId),
    enabled: !!empresaId,
    ...QUERY_OPTIONS,
  })
  const lack = useQuery({
    queryKey: ['kpi-lack', empresaId],
    queryFn: () => getKPILack(empresaId),
    enabled: !!empresaId,
    ...QUERY_OPTIONS,
  })
  const pendiente = useQuery({
    queryKey: ['kpi-pendiente', empresaId],
    queryFn: () => getKPIPendiente(empresaId),
    enabled: !!empresaId,
    ...QUERY_OPTIONS,
  })

  const data: EmpleadoKPIs = {
    complete: complete.data?.count ?? 0,
    lack: lack.data?.count ?? 0,
    pendiente: pendiente.data?.count ?? 0,
  }

  return {
    data,
    isLoading: complete.isLoading || lack.isLoading || pendiente.isLoading,
    isError: complete.isError || lack.isError || pendiente.isError,
  }
}

// — Mutations —

export const useCreateEmpleado = () => {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (req: CreateEmpleadoRequest) => createEmpleado(req),
    onSuccess: (data) => {
      queryClient.invalidateQueries({ queryKey: ['empleados', data.empresa_id] })
    },
  })
}

export const useUpdateEmpleado = () => {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({
      numEmpleado,
      req,
    }: {
      numEmpleado: number
      req: Partial<CreateEmpleadoRequest>
    }) => updateEmpleado(numEmpleado, req),
    onSuccess: (data) => {
      queryClient.invalidateQueries({ queryKey: ['empleado', data.num_empleado] })
    },
  })
}

export const useSoftDeleteEmpleado = () => {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ numEmpleado }: { numEmpleado: number; empresaId: number }) =>
      softDeleteEmpleado(numEmpleado),
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({ queryKey: ['empleados', variables.empresaId] })
      queryClient.invalidateQueries({ queryKey: ['empleado', variables.numEmpleado] })
    },
  })
}

export const useUpsertUbicacion = () => {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({
      numEmpleado,
      req,
    }: {
      numEmpleado: number
      req: Partial<Ubicacion>
    }) => upsertUbicacion(numEmpleado, req),
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({ queryKey: ['ubicacion', variables.numEmpleado] })
    },
  })
}

export const useUpsertAdicionales = () => {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({
      numEmpleado,
      req,
    }: {
      numEmpleado: number
      req: Partial<Adicionales>
    }) => upsertAdicionales(numEmpleado, req),
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({ queryKey: ['adicionales', variables.numEmpleado] })
    },
  })
}

export const useUploadDocumento = () => {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({
      numEmpleado,
      tipo,
      file,
    }: {
      numEmpleado: number
      tipo: TipoDocumento
      file: File
    }) => uploadDocumento(numEmpleado, tipo, file),
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({ queryKey: ['documentos', variables.numEmpleado] })
    },
  })
}

export const useUploadFotologin = () => {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ numEmpleado, empresaId, file }: { numEmpleado: number; empresaId: number; file: File }) =>
      uploadFotologin(numEmpleado, empresaId, file),
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({ queryKey: ['fotologin', variables.numEmpleado] })
    },
  })
}

export const useAprobarInasistencia = () => {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ id, aprobado }: { id: string; aprobado: boolean }) =>
      aprobarInasistencia(id, aprobado),
    onSuccess: (data) => {
      queryClient.invalidateQueries({ queryKey: ['inasistencias', data.empleado_id] })
    },
  })
}

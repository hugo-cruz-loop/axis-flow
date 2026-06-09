import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  cursosListQueryOptions,
  cursoDetailQueryOptions,
  cursoContenidoQueryOptions,
  enrollmentsQueryOptions,
  notaQueryOptions,
  examenQueryOptions,
  resultadosQueryOptions,
  cursosKeys,
} from '../api/queries'
import {
  enroll,
  markLeccionCompleta,
  upsertNota,
  resolverExamen,
} from '../api/cursosClient'
import type { ResolverExamenRequest } from '../types'

// — Queries —

export const useCursos = (includePrivate?: boolean) =>
  useQuery(cursosListQueryOptions(includePrivate))

export const useCurso = (id: number) =>
  useQuery(cursoDetailQueryOptions(id))

export const useCursoContenido = (id: number) =>
  useQuery(cursoContenidoQueryOptions(id))

export const useEnrollments = (empleadoId: number) =>
  useQuery(enrollmentsQueryOptions(empleadoId))

export const useNota = (leccionId: number) =>
  useQuery(notaQueryOptions(leccionId))

export const useExamen = (cursoId: number) =>
  useQuery(examenQueryOptions(cursoId))

export const useResultados = (examenId: number, empleadoId: number) =>
  useQuery(resultadosQueryOptions(examenId, empleadoId))

// — Mutations —

export const useEnroll = () => {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (cursoId: number) => enroll(cursoId),
    onSuccess: (data) => {
      queryClient.invalidateQueries({ queryKey: cursosKeys.enrollments(data.empleado_id) })
    },
  })
}

export const useMarkLeccionCompleta = () => {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ leccionId, cursoId }: { leccionId: number; cursoId: number }) =>
      markLeccionCompleta(leccionId, cursoId),
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({ queryKey: cursosKeys.contenido(variables.cursoId) })
    },
  })
}

export const useUpsertNota = () => {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ leccionId, contenido }: { leccionId: number; contenido: string }) =>
      upsertNota(leccionId, contenido),
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({ queryKey: cursosKeys.notas(variables.leccionId) })
    },
  })
}

export const useResolverExamen = () => {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (req: ResolverExamenRequest) => resolverExamen(req),
    onSuccess: (data) => {
      queryClient.invalidateQueries({
        queryKey: cursosKeys.resultados(data.examen_id, data.empleado_id),
      })
    },
  })
}

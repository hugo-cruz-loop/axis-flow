import { queryOptions } from '@tanstack/react-query'
import {
  getCursos,
  getCurso,
  getCursoContenido,
  getEnrollments,
  getNota,
  getExamen,
  getResultados,
} from './cursosClient'

// — Key factory —

export const cursosKeys = {
  all: ['cursos'] as const,
  list: (includePrivate?: boolean) => ['cursos', 'list', includePrivate] as const,
  detail: (id: number) => ['cursos', 'detail', id] as const,
  contenido: (id: number) => ['cursos', 'contenido', id] as const,
  examen: (cursoId: number) => ['cursos', 'examen', cursoId] as const,
  resultados: (examenId: number, empleadoId: number) =>
    ['cursos', 'resultados', examenId, empleadoId] as const,
  notas: (leccionId: number) => ['cursos', 'notas', leccionId] as const,
  enrollments: (empleadoId: number) => ['cursos', 'enrollments', empleadoId] as const,
}

// — Query option objects —

const QUERY_OPTIONS = {
  staleTime: 10 * 60 * 1000,
  gcTime: 30 * 60 * 1000,
  refetchOnWindowFocus: false,
}

export const cursosListQueryOptions = (includePrivate?: boolean) =>
  queryOptions({
    queryKey: cursosKeys.list(includePrivate),
    queryFn: () => getCursos(includePrivate),
    ...QUERY_OPTIONS,
  })

export const cursoDetailQueryOptions = (id: number) =>
  queryOptions({
    queryKey: cursosKeys.detail(id),
    queryFn: () => getCurso(id),
    enabled: !!id,
    ...QUERY_OPTIONS,
  })

export const cursoContenidoQueryOptions = (id: number) =>
  queryOptions({
    queryKey: cursosKeys.contenido(id),
    queryFn: () => getCursoContenido(id),
    enabled: !!id,
    ...QUERY_OPTIONS,
  })

export const enrollmentsQueryOptions = (empleadoId: number) =>
  queryOptions({
    queryKey: cursosKeys.enrollments(empleadoId),
    queryFn: () => getEnrollments(empleadoId),
    enabled: !!empleadoId,
    ...QUERY_OPTIONS,
  })

export const notaQueryOptions = (leccionId: number) =>
  queryOptions({
    queryKey: cursosKeys.notas(leccionId),
    queryFn: () => getNota(leccionId),
    enabled: !!leccionId,
    ...QUERY_OPTIONS,
  })

export const examenQueryOptions = (cursoId: number) =>
  queryOptions({
    queryKey: cursosKeys.examen(cursoId),
    queryFn: () => getExamen(cursoId),
    enabled: !!cursoId,
    ...QUERY_OPTIONS,
  })

export const resultadosQueryOptions = (examenId: number, empleadoId: number) =>
  queryOptions({
    queryKey: cursosKeys.resultados(examenId, empleadoId),
    queryFn: () => getResultados(examenId, empleadoId),
    enabled: !!examenId && !!empleadoId,
    ...QUERY_OPTIONS,
  })

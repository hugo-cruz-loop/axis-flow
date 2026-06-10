import axiosInstance from '@/api/axiosInstance'
import type {
  Categoria,
  Curso,
  Enrollment,
  Examen,
  Nota,
  ResultadoExamen,
  ResolverExamenRequest,
  Unidad,
} from '../types'

// — Categorias —

export const getCategorias = (empresaId: string): Promise<Categoria[]> =>
  axiosInstance.get(`/v1/empresa/${empresaId}/cursos/categorias`).then((r) => r.data)

export const createCategoria = (data: Omit<Categoria, 'id'>): Promise<Categoria> =>
  axiosInstance.post(`/v1/empresa/${data.empresa_id}/cursos/categorias`, data).then((r) => r.data)

// — Cursos —

export const getCursos = (includePrivate?: boolean): Promise<Curso[]> =>
  axiosInstance
    .get('/v1/cursos', { params: includePrivate ? { include_private: true } : undefined })
    .then((r) => r.data)

export const getCurso = (id: number): Promise<Curso> =>
  axiosInstance.get(`/v1/cursos/${id}`).then((r) => r.data)

export const getCursoContenido = (id: number): Promise<Unidad[]> =>
  axiosInstance.get(`/v1/cursos/${id}/contenido`).then((r) => r.data)

export const createCurso = (data: Omit<Curso, 'id' | 'created_at'>): Promise<Curso> =>
  axiosInstance.post('/v1/cursos', data).then((r) => r.data)

// — Enrollment —

export const enroll = (cursoId: number): Promise<Enrollment> =>
  axiosInstance.post(`/v1/cursos/${cursoId}/enroll`).then((r) => r.data)

export const getEnrollments = (empleadoId: number): Promise<Enrollment[]> =>
  axiosInstance.get(`/v1/empleado/${empleadoId}/enrollments`).then((r) => r.data)

// — Progress —

export const markLeccionCompleta = (leccionId: number, cursoId: number): Promise<void> =>
  axiosInstance.post(`/v1/cursos/${cursoId}/lecciones/${leccionId}/completa`)

// — Notas —

export const getNota = (leccionId: number): Promise<Nota> =>
  axiosInstance.get(`/v1/lecciones/${leccionId}/nota`).then((r) => r.data)

export const upsertNota = (leccionId: number, contenido: string): Promise<Nota> =>
  axiosInstance.put(`/v1/lecciones/${leccionId}/nota`, { contenido }).then((r) => r.data)

// — Examenes —

export const getExamen = (cursoId: number): Promise<Examen> =>
  axiosInstance.get(`/v1/cursos/${cursoId}/examen`).then((r) => r.data)

export const resolverExamen = (req: ResolverExamenRequest, cursoId: number): Promise<ResultadoExamen> =>
  axiosInstance
    .post(`/v1/examenes/${req.examen_id}/resolver`, req, { params: { curso_id: cursoId } })
    .then((r) => r.data)

export const getResultados = (examenId: number, empleadoId: number): Promise<ResultadoExamen[]> =>
  axiosInstance
    .get(`/v1/examenes/${examenId}/resultados`, { params: { empleado_id: empleadoId } })
    .then((r) => r.data)

// — Certificate —

export const downloadCertificate = (examenId: number): Promise<Blob> =>
  axiosInstance
    .get(`/v1/examenes/${examenId}/certificado`, { responseType: 'blob' })
    .then((r) => r.data)

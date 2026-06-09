import axiosInstance from './axiosInstance'
import { useAuthStore } from '@/store/authStore'
import type {
  Empleado,
  Ubicacion,
  Adicionales,
  Documentos,
  Asistencia,
  Inasistencia,
  Fotologin,
  UserDevice,
  CreateEmpleadoRequest,
  TipoDocumento,
} from '@/types/empleados'

// — Empleado CRUD —

export const createEmpleado = (req: CreateEmpleadoRequest): Promise<Empleado> =>
  axiosInstance.post('/v1/empleado', req).then((r) => r.data)

export const getEmpleado = (numEmpleado: number): Promise<Empleado> =>
  axiosInstance.get(`/v1/empleado/${numEmpleado}`).then((r) => r.data)

export const updateEmpleado = (
  numEmpleado: number,
  req: Partial<CreateEmpleadoRequest>,
): Promise<Empleado> =>
  axiosInstance.put(`/v1/empleado/${numEmpleado}`, req).then((r) => r.data)

export const softDeleteEmpleado = (numEmpleado: number): Promise<void> =>
  axiosInstance.delete(`/v1/empleado/${numEmpleado}`)

export const listEmpleados = (
  empresaId: number,
  page = 1,
  size = 20,
): Promise<{ data: Empleado[]; total: number }> =>
  axiosInstance
    .get(`/v1/empresa/${empresaId}/empleados`, { params: { page, size } })
    .then((r) => r.data)

export const getEmpleadoByUser = (userId: string): Promise<Empleado> =>
  axiosInstance.get(`/v1/empleado/by-user/${userId}`).then((r) => r.data)

// — Expediente —

export const getUbicacion = (numEmpleado: number): Promise<Ubicacion> =>
  axiosInstance.get(`/v1/empleado/${numEmpleado}/ubicacion`).then((r) => r.data)

export const upsertUbicacion = (
  numEmpleado: number,
  req: Partial<Ubicacion>,
): Promise<Ubicacion> =>
  axiosInstance.put(`/v1/empleado/${numEmpleado}/ubicacion`, req).then((r) => r.data)

export const getAdicionales = (numEmpleado: number): Promise<Adicionales> =>
  axiosInstance.get(`/v1/empleado/${numEmpleado}/adicionales`).then((r) => r.data)

export const upsertAdicionales = (
  numEmpleado: number,
  req: Partial<Adicionales>,
): Promise<Adicionales> =>
  axiosInstance.put(`/v1/empleado/${numEmpleado}/adicionales`, req).then((r) => r.data)

export const getDocumentos = (numEmpleado: number): Promise<Documentos> =>
  axiosInstance.get(`/v1/empleado/${numEmpleado}/documentos`).then((r) => r.data)

export const uploadDocumento = (
  numEmpleado: number,
  tipo: TipoDocumento,
  file: File,
): Promise<Documentos> => {
  const form = new FormData()
  form.append('tipo_documento', tipo)
  form.append('archivo', file)
  return axiosInstance
    .post(`/v1/empleado/${numEmpleado}/documentos`, form, {
      headers: { 'Content-Type': 'multipart/form-data' },
    })
    .then((r) => r.data)
}

// — KPIs —

export const getKPIComplete = (empresaId: number): Promise<{ count: number }> =>
  axiosInstance
    .get(`/v1/empresa/${empresaId}/empleados/documentos/complete`)
    .then((r) => r.data)

export const getKPILack = (empresaId: number): Promise<{ count: number }> =>
  axiosInstance
    .get(`/v1/empresa/${empresaId}/empleados/documentos/lack`)
    .then((r) => r.data)

export const getKPIPendiente = (empresaId: number): Promise<{ count: number }> =>
  axiosInstance
    .get(`/v1/empresa/${empresaId}/empleados/documentos/pendiente`)
    .then((r) => r.data)

// — Asistencias —

export const getAsistencias = (
  empresaId: number,
  empleadoId?: number,
): Promise<Asistencia[]> =>
  axiosInstance
    .get('/v1/empleado/asistencias', {
      params: empleadoId ? { empresa_id: empresaId, empleado_id: empleadoId } : { empresa_id: empresaId },
    })
    .then((r) => r.data)

export const getInasistencias = (empleadoId: number, empresaId: number): Promise<Inasistencia[]> =>
  axiosInstance
    .get('/v1/empleado/inasistencia', { params: { empresa_id: empresaId, empleado_id: empleadoId } })
    .then((r) => r.data)

export const aprobarInasistencia = (id: string, aprobado: boolean): Promise<Inasistencia> =>
  axiosInstance
    .put(`/v1/empleado/inasistencia/${id}`, { aprobado })
    .then((r) => r.data)

// — Biometric —

export const getFotologin = (numEmpleado: number, empresaId: number): Promise<Fotologin[]> =>
  axiosInstance
    .get(`/v1/empleado/${numEmpleado}/fotologin`, { params: { empresa_id: empresaId } })
    .then((r) => r.data)

export const uploadFotologin = (numEmpleado: number, empresaId: number, file: File): Promise<Fotologin> => {
  const form = new FormData()
  form.append('foto', file)
  return axiosInstance
    .post(`/v1/empleado/${numEmpleado}/fotologin`, form, {
      params: { empresa_id: empresaId },
      headers: { 'Content-Type': 'multipart/form-data' },
    })
    .then((r) => r.data)
}

// — Devices —

export const getDevices = (numEmpleado: number, empresaId: number): Promise<UserDevice[]> =>
  axiosInstance
    .get('/v1/empleado/devices', { params: { empresa_id: empresaId, empleado_id: numEmpleado } })
    .then((r) => r.data)

// — WebSocket —

export const connectAttendanceWS = (empresaId: number, token: string): WebSocket => {
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  const host = window.location.host
  const url = `${protocol}//${host}/api/v1/ws/empleados/asistencias?empresa_id=${empresaId}&token=${encodeURIComponent(token)}`
  return new WebSocket(url)
}

// Helper to get current token outside of React render
export const getAccessToken = (): string | null => useAuthStore.getState().accessToken

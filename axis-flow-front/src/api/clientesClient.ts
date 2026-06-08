import axiosInstance from './axiosInstance'
import type {
  Cliente,
  Factura,
  Presupuesto,
  CalendarioLaboral,
  Localidad,
  ServicioLocalidad,
  Horario,
  Herramienta,
  Actividad,
  EvaluacionServicio,
  PatchEstatusResponse,
  ClienteStats,
  CreateClienteRequest,
  CreateFacturaRequest,
  CreatePresupuestoRequest,
  CreateCalendarioRequest,
  CreateLocalidadRequest,
  CreateEvaluacionRequest,
} from '@/types/clientes'

// — Cliente —

export const createCliente = (req: CreateClienteRequest): Promise<Cliente> =>
  axiosInstance.post('/v1/cliente', req).then((r) => r.data)

export const getCliente = (id: string): Promise<Cliente> =>
  axiosInstance.get(`/v1/cliente/${id}`).then((r) => r.data)

export const updateCliente = (id: string, req: Partial<CreateClienteRequest>): Promise<Cliente> =>
  axiosInstance.put(`/v1/cliente/${id}`, req).then((r) => r.data)

export const patchEstatus = (id: string, estatus: number): Promise<PatchEstatusResponse> =>
  axiosInstance
    .patch(`/v1/cliente/${id}/estatus`, { estatus }, {
      validateStatus: (s) => (s >= 200 && s < 300) || s === 206,
    })
    .then((r) => r.data)

export const deleteCliente = (id: string): Promise<void> =>
  axiosInstance.delete(`/v1/cliente/${id}`)

export const getClienteByUser = (userId: string): Promise<Cliente> =>
  axiosInstance.get(`/v1/cliente/by-user/${userId}`).then((r) => r.data)

export const listClientes = (
  empresaId: number,
  page = 1,
  size = 20,
): Promise<{ data: Cliente[]; total: number }> =>
  axiosInstance
    .get(`/v1/empresa/${empresaId}/clientes`, { params: { page, size } })
    .then((r) => r.data)

export const getClienteStats = (empresaId: number): Promise<ClienteStats> =>
  axiosInstance.get(`/v1/empresa/${empresaId}/clientes/stats`).then((r) => r.data)

// — Satellite: Factura —

export const createFactura = (clienteId: string, req: CreateFacturaRequest): Promise<Factura> =>
  axiosInstance.post(`/v1/cliente/${clienteId}/factura`, req).then((r) => r.data)

export const getFactura = (clienteId: string): Promise<Factura> =>
  axiosInstance.get(`/v1/cliente/${clienteId}/factura`).then((r) => r.data)

export const updateFactura = (
  clienteId: string,
  req: Partial<CreateFacturaRequest>,
): Promise<Factura> =>
  axiosInstance.put(`/v1/cliente/${clienteId}/factura`, req).then((r) => r.data)

// — Satellite: Presupuesto —

export const createPresupuesto = (
  clienteId: string,
  req: CreatePresupuestoRequest,
): Promise<Presupuesto> =>
  axiosInstance.post(`/v1/cliente/${clienteId}/presupuesto`, req).then((r) => r.data)

export const getPresupuesto = (clienteId: string): Promise<Presupuesto> =>
  axiosInstance.get(`/v1/cliente/${clienteId}/presupuesto`).then((r) => r.data)

export const updatePresupuesto = (
  clienteId: string,
  req: Partial<CreatePresupuestoRequest>,
): Promise<Presupuesto> =>
  axiosInstance.put(`/v1/cliente/${clienteId}/presupuesto`, req).then((r) => r.data)

// — Satellite: Calendario —

export const createCalendario = (
  clienteId: string,
  req: CreateCalendarioRequest,
): Promise<CalendarioLaboral> =>
  axiosInstance.post(`/v1/cliente/${clienteId}/calendario`, req).then((r) => r.data)

export const getCalendario = (clienteId: string): Promise<CalendarioLaboral> =>
  axiosInstance.get(`/v1/cliente/${clienteId}/calendario`).then((r) => r.data)

export const updateCalendario = (
  clienteId: string,
  req: Partial<CreateCalendarioRequest>,
): Promise<CalendarioLaboral> =>
  axiosInstance.put(`/v1/cliente/${clienteId}/calendario`, req).then((r) => r.data)

// — Localidades —

export const createLocalidad = (
  clienteId: string,
  req: CreateLocalidadRequest,
): Promise<Localidad> =>
  axiosInstance.post(`/v1/cliente/${clienteId}/localidades`, req).then((r) => r.data)

export const listLocalidades = (clienteId: string): Promise<Localidad[]> =>
  axiosInstance.get(`/v1/cliente/${clienteId}/localidades`).then((r) => r.data)

export const getLocalidad = (localidadId: string): Promise<Localidad> =>
  axiosInstance.get(`/v1/localidad/${localidadId}`).then((r) => r.data)

export const updateLocalidad = (
  localidadId: string,
  req: Partial<CreateLocalidadRequest>,
): Promise<Localidad> =>
  axiosInstance.put(`/v1/localidad/${localidadId}`, req).then((r) => r.data)

export const deleteLocalidad = (localidadId: string): Promise<void> =>
  axiosInstance.delete(`/v1/localidad/${localidadId}`)

// — Site config: Servicios —

export const listServicios = (localidadId: string): Promise<ServicioLocalidad[]> =>
  axiosInstance.get(`/v1/localidad/${localidadId}/servicios`).then((r) => r.data)

export const addServicio = (localidadId: string, servicioId: number): Promise<void> =>
  axiosInstance.post(`/v1/localidad/${localidadId}/servicios`, { servicio_id: servicioId })

export const removeServicio = (localidadId: string, servicioId: number): Promise<void> =>
  axiosInstance.delete(`/v1/localidad/${localidadId}/servicios/${servicioId}`)

// — Site config: Horarios —

export const listHorarios = (localidadId: string): Promise<Horario[]> =>
  axiosInstance.get(`/v1/localidad/${localidadId}/horarios`).then((r) => r.data)

export const createHorario = (
  localidadId: string,
  req: Omit<Horario, 'id' | 'localidad_id' | 'created_at' | 'updated_at'>,
): Promise<Horario> =>
  axiosInstance.post(`/v1/localidad/${localidadId}/horarios`, req).then((r) => r.data)

// — Site config: Herramientas —

export const listHerramientas = (localidadId: string): Promise<Herramienta[]> =>
  axiosInstance.get(`/v1/localidad/${localidadId}/herramientas`).then((r) => r.data)

export const createHerramienta = (
  localidadId: string,
  req: Omit<Herramienta, 'id' | 'localidad_id' | 'created_at' | 'updated_at'>,
): Promise<Herramienta> =>
  axiosInstance.post(`/v1/localidad/${localidadId}/herramientas`, req).then((r) => r.data)

// — Site config: Actividades —

export const listActividades = (localidadId: string): Promise<Actividad[]> =>
  axiosInstance.get(`/v1/localidad/${localidadId}/actividades`).then((r) => r.data)

export const createActividad = (
  localidadId: string,
  req: Omit<Actividad, 'id' | 'localidad_id' | 'created_at' | 'updated_at'>,
): Promise<Actividad> =>
  axiosInstance.post(`/v1/localidad/${localidadId}/actividades`, req).then((r) => r.data)

// — Evaluaciones —

export const listEvaluaciones = (clienteId: string): Promise<EvaluacionServicio[]> =>
  axiosInstance.get(`/v1/cliente/${clienteId}/evaluaciones`).then((r) => r.data)

export const createEvaluacion = (
  clienteId: string,
  req: CreateEvaluacionRequest,
): Promise<EvaluacionServicio> =>
  axiosInstance.post(`/v1/cliente/${clienteId}/evaluaciones`, req).then((r) => r.data)

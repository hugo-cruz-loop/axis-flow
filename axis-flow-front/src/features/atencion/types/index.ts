export interface SolicitudQueja {
  id: string
  empresa_id: string
  empleado_id: number
  tipo_queja_id: string
  titulo: string
  descripcion: string
  estatus: 1 | 2 | 3
  fecha_vigencia?: string
  ultima_resp: 1 | 2 | 3
  created_at: string
  updated_at: string
}

export interface RespuestaQueja {
  id: string
  solicitud_id: string
  remitente_id: string
  rol_respuesta: 1 | 2 | 3
  mensaje: string
  archivo_adjunto_url?: string
  leido: boolean
  created_at: string
}

export interface TicketServicio {
  id: string
  empresa_id: string
  cliente_id: string
  localidad_id: string
  asunto: string
  descripcion: string
  estatus: 1 | 2 | 3
  ultima_resp: 1 | 2
  created_at: string
  updated_at: string
}

export interface RespuestaServicio {
  id: string
  ticket_id: string
  remitente_id: string
  rol_respuesta: 1 | 2
  mensaje: string
  archivo_adjunto_url?: string
  leido: boolean
  created_at: string
}

export interface IncidenciaSupervisor {
  id: string
  empresa_id: string
  supervisor_id: string
  empleado_id: number
  localidad_id: string
  tipo_incidencia_id: string
  descripcion: string
  sancion_sugerida?: string
  evidencia_url?: string
  created_at: string
}

export interface TicketStats {
  empresa_id: string
  pendiente: number
  en_proceso: number
  finalizado: number
}

export interface PaginatedAtencion<T> {
  success: boolean
  data: T[]
  meta: { page: number; limit: number; total_records: number; total_pages: number }
}

export type EstatusLabel = 'Pendiente' | 'En Proceso' | 'Finalizado'
export const ESTATUS_LABELS: Record<number, EstatusLabel> = {
  1: 'Pendiente',
  2: 'En Proceso',
  3: 'Finalizado',
}

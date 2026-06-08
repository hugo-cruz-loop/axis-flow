export const ClienteStatus = {
  ACTIVO: 1,
  INCOMPLETO: 2,
} as const
export type ClienteStatusType = typeof ClienteStatus[keyof typeof ClienteStatus]

export interface Cliente {
  id: string
  empresa_id: number
  representante_id: string
  nombre_comercial: string
  razon_social?: string
  fecha_inicio_contrato?: string
  estatus: ClienteStatusType
  created_at: string
  updated_at: string
}

export interface Factura {
  id: string
  cliente_id: string
  rfc: string
  razon_social: string
  domicilio_fiscal: string
  created_at: string
  updated_at: string
}

export interface Presupuesto {
  id: string
  cliente_id: string
  personal_requerido?: number
  material_estimado?: string
  costo_mensual?: number
  created_at: string
  updated_at: string
}

export interface CalendarioLaboral {
  cliente_id: string
  semana_laboral: Record<string, boolean>
  dias_inhabiles: string[]
  created_at: string
  updated_at: string
}

export interface Localidad {
  id: string
  cliente_id: string
  nombre: string
  direccion: string
  supervisor_id: string
  tipo_localidad_id: number
  latitud?: number
  longitud?: number
  created_at: string
  updated_at: string
}

export interface ServicioLocalidad {
  id: string
  localidad_id: string
  servicio_id: number
  status_activo: boolean
  created_at: string
  updated_at: string
}

export interface Horario {
  id: string
  localidad_id: string
  hora_entrada: string
  hora_salida: string
  hora_comida_inicio?: string
  hora_comida_fin?: string
  created_at: string
  updated_at: string
}

export interface Herramienta {
  id: string
  localidad_id: string
  nombre: string
  cantidad: number
  especificaciones?: string
  created_at: string
  updated_at: string
}

export interface Actividad {
  id: string
  localidad_id: string
  descripcion: string
  frecuencia: string
  orden: number
  created_at: string
  updated_at: string
}

export interface EvaluacionServicio {
  id: string
  cliente_id: string
  puntuacion: number
  comentarios?: string
  de_usuario_id: string
  fecha: string
  created_at: string
  updated_at: string
}

export interface QualityGateWarning {
  message: string
  missing_sections: Array<'factura' | 'presupuesto' | 'calendario'>
}

export interface PatchEstatusResponse {
  data: Cliente | null
  warnings?: QualityGateWarning
}

export interface ClienteStats {
  active: number
  inactive: number
}

export interface CreateClienteRequest {
  empresa_id: number
  representante_id: string
  nombre_comercial: string
  razon_social?: string
  fecha_inicio_contrato?: string
}

export interface CreateFacturaRequest {
  rfc: string
  razon_social: string
  domicilio_fiscal: string
}

export interface CreatePresupuestoRequest {
  personal_requerido?: number
  material_estimado?: string
  costo_mensual?: number
}

export interface CreateCalendarioRequest {
  semana_laboral: Record<string, boolean>
  dias_inhabiles: string[]
}

export interface CreateLocalidadRequest {
  nombre: string
  direccion: string
  supervisor_id: string
  tipo_localidad_id: number
  latitud?: number
  longitud?: number
}

export interface CreateEvaluacionRequest {
  puntuacion: number
  comentarios?: string
}

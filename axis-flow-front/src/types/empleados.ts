export const EmpleadoStatus = {
  ACTIVO: 1,
  INCOMPLETO: 2,
  BAJA: 4,
} as const
export type EmpleadoStatusType = typeof EmpleadoStatus[keyof typeof EmpleadoStatus]

export const ObservacionStatus = {
  PENDIENTE: 1,
  VALIDADA: 2,
  RECHAZADA: 3,
} as const
export type ObservacionStatusType = typeof ObservacionStatus[keyof typeof ObservacionStatus]

export const EstatusRango = {
  EN_RANGO: 1,
  FUERA_RANGO: 2,
} as const
export type EstatusRangoType = typeof EstatusRango[keyof typeof EstatusRango]

export const TipoRegistro = {
  ENTRADA_LABORAL: 'ENTRADA_LABORAL',
  SALIDA_LABORAL: 'SALIDA_LABORAL',
  ENTRADA_COMIDA: 'ENTRADA_COMIDA',
  SALIDA_COMIDA: 'SALIDA_COMIDA',
} as const
export type TipoRegistroType = typeof TipoRegistro[keyof typeof TipoRegistro]

export interface Empleado {
  num_empleado: number
  id_empleado: string
  usuario_id: string
  empresa_id: number
  nombre: string
  apellido_paterno: string
  apellido_materno?: string
  status: number
  created_at: string
  updated_at: string
}

export interface Ubicacion {
  empleado_id: number
  curp: string
  nss: string
  calle: string
  numero_exterior: string
  numero_interior?: string
  colonia: string
  codigo_postal: string
  ciudad_id: number
  estado_id: number
  pais_id: number
}

export interface Adicionales {
  empleado_id: number
  contacto_emergencia_nombre: string
  contacto_emergencia_telefono: string
  contacto_emergencia_parentesco: string
  beneficiarios: Array<{ nombre: string; parentesco: string; porcentaje: number }>
}

export interface Documentos {
  empleado_id: number
  acta_url?: string
  ine_url?: string
  comprobante_domicilio_url?: string
  curp_pdf_url?: string
  nss_pdf_url?: string
  contrato_url?: string
  estatus_validacion: number
}

export interface Asistencia {
  id: string
  empleado_id: number
  geolocalizacion: string
  estatus_rango: number
  foto_entrada_url: string
  estatus_observacion_entrada: number
  similitud_facial?: number
  tipo_registro: string
  fecha: string
  hora_entrada: string
  hora_salida?: string
  created_at: string
  updated_at: string
}

export interface Inasistencia {
  id: string
  empleado_id: number
  tipo_incidencia: string
  fecha_inicio: string
  fecha_fin: string
  justificante_url?: string
  aprobado: boolean
  observaciones?: string
  created_at: string
}

export interface Fotologin {
  id: string
  empleado_id: number
  foto_base_url: string
  created_at: string
}

export interface UserDevice {
  id: string
  empleado_id: number
  device_uuid: string
  device_model?: string
  os_version?: string
  is_active: boolean
  created_at: string
}

export interface EmpleadoKPIs {
  complete: number
  lack: number
  pendiente: number
}

export interface AsistenciaVerificadaEvent {
  asistencia_id: string
  empleado_id: number
  empresa_id: number
  similitud_facial: number
  estatus_observacion: number
  foto_entrada_url: string
  timestamp: string
}

export interface CreateEmpleadoRequest {
  email: string
  nombre: string
  apellido_paterno: string
  apellido_materno?: string
  id_empleado: string
  empresa_id: number
}

export type TipoDocumento =
  | 'ACTA_NACIMIENTO'
  | 'INE'
  | 'COMPROBANTE_DOMICILIO'
  | 'CURP_PDF'
  | 'NSS_PDF'
  | 'CONTRATO'

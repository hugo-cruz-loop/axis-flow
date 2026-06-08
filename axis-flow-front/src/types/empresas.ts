export type EmpresaStatus = 'PENDING_PAYMENT' | 'ACTIVE' | 'INACTIVE' | 'SUSPENDED'
export type PagoStatus = 'PENDING' | 'PAID' | 'FAILED' | 'REFUNDED'

export interface Empresa {
  id: number
  nombre: string
  direccion: string
  telefono: string
  representante_id: string // UUID
  plan_id: number
  status: EmpresaStatus
  vigencia?: string // ISO date
  created_at: string
  updated_at: string
}

export interface DatosFiscales {
  id: number
  empresa_id: number
  rfc: string
  razon_social: string
  logo_url?: string
  imss_patronal?: string
  repse?: string
}

export interface Apoderado {
  id: number
  empresa_id: number
  nombre: string
  curp: string
  rfc: string
  email: string
  telefono?: string
}

export interface Servicio {
  id: number
  empresa_id: number
  nombre: string
  descripcion?: string
  precio: number
  status_activo: boolean
}

export interface OnboardingRequest {
  nombre: string
  direccion: string
  telefono: string
  plan_id: number
  representante: {
    email: string
    nombre: string
    apellido_paterno: string
    apellido_materno?: string
  }
}

export interface OnboardingResult {
  empresa_id: number
  clave_pago: string
}

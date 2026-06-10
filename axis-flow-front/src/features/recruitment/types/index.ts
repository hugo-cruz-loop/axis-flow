export interface Trabajo {
  id: string
  empresa_id: string
  titulo: string
  descripcion: string
  requisitos: string[]
  fecha_caducar: string
  estatus_vacante: 1 | 2 | 3 // 1=Activo, 2=Pausa, 3=Fin
  created_at: string
  updated_at: string
}

export interface Postulacion {
  id: string
  trabajo_id: string
  nombre_completo: string
  email: string
  telefono?: string
  cv_url: string
  estatus: 1 | 2 | 3 | 4 | 5
  created_at: string
  updated_at: string
}

export interface Evaluacion {
  id: string
  postulacion_id: string
  puntualidad: number
  cortesia: number
  soft_skills: number
  comentarios?: string
  evaluator_id: string
  created_at: string
}

export interface StageStat {
  stage: number
  stage_name: string
  count: number
}

export interface PipelineStats {
  trabajo_id: string
  stats: StageStat[]
}

export interface PaginatedResponse<T> {
  data: T[]
  meta: {
    page: number
    page_size: number
    total: number
    request_id: string
  }
}

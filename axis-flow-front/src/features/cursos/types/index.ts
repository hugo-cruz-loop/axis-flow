export interface Categoria {
  id: number
  nombre: string
  descripcion?: string
  empresa_id: string
}

export interface Modulo {
  id: number
  nombre: string
  empresa_id: string
}

export type CursoEstatus = 1 | 2 // 1=Public, 2=Private
export type EnrollEstatus = 1 | 2 // 1=Completado, 2=Incompleto
export type LeccionTipo = 1 | 2 | 3 // 1=Video, 2=Texto, 3=Documento

export interface Curso {
  id: number
  titulo: string
  descripcion?: string
  empresa_id: string
  modulo_id?: number
  categoria_id: number
  estatus: CursoEstatus
  imagen_url?: string
  duracion_minutos: number
  prerequisito_id?: number
  created_at: string
}

export interface Unidad {
  id: number
  curso_id: number
  titulo: string
  orden: number
  lecciones?: Leccion[]
}

export interface Leccion {
  id: number
  unidad_id: number
  titulo: string
  tipo: LeccionTipo
  contenido_url?: string
  orden: number
}

export interface Nota {
  id: number
  leccion_id: number
  empleado_id: number
  contenido: string
  updated_at: string
}

export interface Enrollment {
  id: number
  curso_id: number
  empleado_id: number
  empresa_id: string
  avance_porcentaje: number
  estatus: EnrollEstatus
  enrolled_at: string
  completed_at?: string
}

export interface AvanceLeccion {
  empleado_id: number
  leccion_id: number
  completado_at: string
}

// Anti-cheat: es_correcta is never included in client-side types
export interface ExamenOpcion {
  id: number
  pregunta_id: number
  texto: string
}

export interface ExamenPregunta {
  id: number
  examen_id: number
  enunciado: string
  orden: number
  opciones: ExamenOpcion[]
}

export interface Examen {
  id: number
  curso_id: number
  titulo: string
  note_min: number
  num_intentos: number
  tiempo_limite_min?: number
  preguntas: ExamenPregunta[]
}

export interface ResultadoExamen {
  id: number
  examen_id: number
  empleado_id: number
  calificacion: number
  aprobado: boolean
  intento: number
  created_at: string
}

export interface ResolverExamenRequest {
  examen_id: number
  respuestas: { pregunta_id: number; opcion_id: number }[]
}

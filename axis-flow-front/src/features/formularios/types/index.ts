import type { z } from 'zod'
import {
  formularioCreateSchema,
  formularioResponseSchema,
  formularioListResponseSchema,
  preguntaCreateSchema,
  preguntaResponseSchema,
  eventoCreateSchema,
  eventoResponseSchema,
  eventoListResponseSchema,
  eventoIniciadoCreateSchema,
  eventoIniciadoResponseSchema,
  respuestaCreateSchema,
  respuestaResponseSchema,
  geolocalizacionSchema,
  matrixConfigSchema,
  choiceConfigSchema,
  cameraConfigSchema,
} from '../schemas/validation'

// Re-export the schema constant + value type
export { TIPO_PREGUNTA } from '../schemas/validation'
export type { TipoPreguntaValue } from '../schemas/validation'

// — Inferred input types (request bodies) —
export type FormularioCreate = z.infer<typeof formularioCreateSchema>
export type FormularioList = z.infer<typeof formularioListResponseSchema>
export type FormularioRecord = FormularioList['data'][number]
export type PreguntaCreate = z.infer<typeof preguntaCreateSchema>
export type EventoCreate = z.infer<typeof eventoCreateSchema>
export type EventoIniciadoCreate = z.infer<typeof eventoIniciadoCreateSchema>
export type RespuestaCreate = z.infer<typeof respuestaCreateSchema>

// — Inferred response types —
export type FormularioResponse = z.infer<typeof formularioResponseSchema>
export type FormularioListResponse = z.infer<typeof formularioListResponseSchema>
export type PreguntaResponse = z.infer<typeof preguntaResponseSchema>
export type EventoResponse = z.infer<typeof eventoResponseSchema>
export type EventoListResponse = z.infer<typeof eventoListResponseSchema>
export type EventoIniciadoResponse = z.infer<typeof eventoIniciadoResponseSchema>
export type RespuestaResponse = z.infer<typeof respuestaResponseSchema>

// — Sub-shapes —
export type Geolocalizacion = z.infer<typeof geolocalizacionSchema>
export type MatrixConfig = z.infer<typeof matrixConfigSchema>
export type ChoiceConfig = z.infer<typeof choiceConfigSchema>
export type CameraConfig = z.infer<typeof cameraConfigSchema>

// — Helper shapes for the builder UI (draft state) —

/** Pregunta as it lives in the builder before persistence (id is local, not a server UUID). */
export interface PreguntaDraft {
  id: string
  orden: number
  texto_pregunta: string
  tipo_pregunta: TipoPreguntaValue
  obligatoria: boolean
  respuesta_predefinida?: MatrixConfig | ChoiceConfig | CameraConfig | Record<string, unknown>
}

/** Form-level draft — not yet persisted. */
export interface FormularioDraft {
  id?: string
  empresa_id: string
  nombre: string
  descripcion?: string
  activo: boolean
  preguntas: PreguntaDraft[]
}

// — Estatus label maps for UI rendering —

export const EVENTO_ESTATUS_LABELS: Record<string, string> = {
  pendiente: 'Pendiente',
  iniciado: 'Iniciado',
  completado: 'Completado',
  cancelado: 'Cancelado',
}

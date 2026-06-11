import { z } from 'zod'

// — Tipo de pregunta enum (subset supported by formularios backend) —

export const TIPO_PREGUNTA = {
  SHORT_TEXT: 1,
  CHECKBOX: 2,
  RATING: 3,
  MATRIX: 5,
  CAMERA: 8,
  SIGNATURE: 11,
} as const

export type TipoPreguntaValue = (typeof TIPO_PREGUNTA)[keyof typeof TIPO_PREGUNTA]

export const tipoPreguntaSchema = z.union([
  z.literal(1),
  z.literal(2),
  z.literal(3),
  z.literal(5),
  z.literal(8),
  z.literal(11),
])

// — Shared sub-schemas —

export const geolocalizacionSchema = z.object({
  latitud: z.number().min(-90).max(90),
  longitud: z.number().min(-180).max(180),
})

export const matrixConfigSchema = z.object({
  rows: z.array(z.string().min(1)).min(1),
  columns: z.array(z.string().min(1)).min(1),
})

export const choiceConfigSchema = z.object({
  options: z.array(z.string().min(1)).min(2),
})

export const cameraConfigSchema = z.object({
  min_photos: z.number().int().min(0).max(3).default(0),
  max_photos: z.number().int().min(1).max(3).default(1),
})

// — Form builder (design time) —

export const formularioCreateSchema = z.object({
  empresa_id: z.string().uuid(),
  nombre: z.string().min(1).max(150),
  descripcion: z.string().max(500).optional(),
  activo: z.boolean().default(true),
})

export type FormularioCreate = z.infer<typeof formularioCreateSchema>

export const preguntaCreateSchema = z.object({
  formulario_id: z.string().uuid(),
  orden: z.number().int().min(1),
  texto_pregunta: z.string().min(1).max(1000),
  tipo_pregunta: tipoPreguntaSchema,
  obligatoria: z.boolean().default(false),
  respuesta_predefinida: z.unknown().optional(),
})

export type PreguntaCreate = z.infer<typeof preguntaCreateSchema>

// — Evento —

export const eventoCreateSchema = z.object({
  empresa_id: z.string().uuid(),
  cliente_id: z.string().uuid(),
  nombre: z.string().min(1).max(150),
  descripcion: z.string().max(1000).optional(),
  fecha_programada: z.string().datetime(),
  formularios_asociados: z.array(z.string().uuid()).optional(),
})

export type EventoCreate = z.infer<typeof eventoCreateSchema>

export const eventoIniciadoCreateSchema = z.object({
  evento_id: z.string().uuid(),
  empleado_id: z.number().int().positive(),
  geolocalizacion_inicio: geolocalizacionSchema.optional(),
})

export type EventoIniciadoCreate = z.infer<typeof eventoIniciadoCreateSchema>

// — Respuesta —

export const respuestaCreateSchema = z.object({
  evento_iniciado_id: z.string().uuid(),
  formulario_id: z.string().uuid(),
  pregunta_id: z.string().uuid(),
  respuesta_lista: z.unknown(),
  evidencia_urls: z.array(z.string().url()).optional(),
  documento_url: z.string().url().nullable().optional(),
  geolocalizacion_respuesta: geolocalizacionSchema.optional(),
})

export type RespuestaCreate = z.infer<typeof respuestaCreateSchema>

// — Response envelopes (success-only parsed path) —

const formularioDataSchema = z.object({
  id: z.string().uuid(),
  empresa_id: z.string().uuid(),
  nombre: z.string(),
  descripcion: z.string().optional(),
  activo: z.boolean(),
  created_at: z.string().datetime().optional(),
  updated_at: z.string().datetime().optional(),
})

export const formularioResponseSchema = z.object({
  success: z.literal(true),
  data: formularioDataSchema,
})

export type FormularioResponse = z.infer<typeof formularioResponseSchema>

export const formularioListResponseSchema = z.object({
  success: z.literal(true),
  data: z.array(formularioDataSchema.extend({
    preguntas_count: z.number().int().optional(),
  })),
  meta: z.object({
    page: z.number().int(),
    limit: z.number().int(),
    total_records: z.number().int(),
    total_pages: z.number().int(),
  }).optional(),
})

export type FormularioListResponse = z.infer<typeof formularioListResponseSchema>

const preguntaDataSchema = z.object({
  id: z.string().uuid(),
  formulario_id: z.string().uuid(),
  orden: z.number().int(),
  texto_pregunta: z.string(),
  tipo_pregunta: z.number().int(),
  obligatoria: z.boolean(),
  respuesta_predefinida: z.unknown().optional(),
  created_at: z.string().datetime().optional(),
})

export const preguntaResponseSchema = z.object({
  success: z.literal(true),
  data: preguntaDataSchema,
})

export type PreguntaResponse = z.infer<typeof preguntaResponseSchema>

const eventoDataSchema = z.object({
  id: z.string().uuid(),
  empresa_id: z.string().uuid(),
  cliente_id: z.string().uuid(),
  nombre: z.string(),
  descripcion: z.string().optional(),
  fecha_programada: z.string().datetime(),
  estatus: z.string(),
  formularios_asociados: z.array(z.string().uuid()).optional(),
  created_at: z.string().datetime().optional(),
})

export const eventoResponseSchema = z.object({
  success: z.literal(true),
  data: eventoDataSchema,
})

export type EventoResponse = z.infer<typeof eventoResponseSchema>

export const eventoListResponseSchema = z.object({
  success: z.literal(true),
  data: z.array(eventoDataSchema),
  meta: z.object({
    page: z.number().int(),
    limit: z.number().int(),
    total_records: z.number().int(),
    total_pages: z.number().int(),
  }).optional(),
})

export type EventoListResponse = z.infer<typeof eventoListResponseSchema>

const eventoIniciadoDataSchema = z.object({
  id: z.string().uuid(),
  evento_id: z.string().uuid(),
  empleado_id: z.number().int(),
  fecha_inicio: z.string().datetime(),
  geolocalizacion_inicio: geolocalizacionSchema.optional(),
  estatus: z.string(),
})

export const eventoIniciadoResponseSchema = z.object({
  success: z.literal(true),
  data: eventoIniciadoDataSchema,
})

export type EventoIniciadoResponse = z.infer<typeof eventoIniciadoResponseSchema>

const respuestaDataSchema = z.object({
  id: z.string().uuid(),
  evento_iniciado_id: z.string().uuid(),
  formulario_id: z.string().uuid(),
  pregunta_id: z.string().uuid(),
  respuesta_lista: z.unknown(),
  evidencia_urls: z.array(z.string()).optional(),
  documento_url: z.string().nullable().optional(),
  created_at: z.string().datetime().optional(),
})

export const respuestaResponseSchema = z.object({
  success: z.literal(true),
  data: respuestaDataSchema,
})

export type RespuestaResponse = z.infer<typeof respuestaResponseSchema>

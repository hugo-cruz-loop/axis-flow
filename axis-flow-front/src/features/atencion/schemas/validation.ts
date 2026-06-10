import { z } from 'zod'

// Complaint creation (employee)
export const QuejaSchema = z.object({
  tipo_queja_id: z.string().uuid(),
  titulo: z.string().min(5).max(255),
  descripcion: z.string().min(10).max(1000),
  archivo_adjunto_url: z.string().url().optional(),
})

// Client ticket creation
export const TicketSchema = z.object({
  localidad_id: z.string().uuid(),
  asunto: z.string().min(5).max(255),
  descripcion: z.string().min(15).max(1500),
  archivo_adjunto_url: z.string().url().optional(),
})

// Reply message
export const MensajeSchema = z.object({
  mensaje: z.string().min(1).max(500),
  archivo_adjunto_url: z.string().url().optional(),
})

// Supervisor incident
export const IncidenciaSchema = z.object({
  empleado_id: z.number().int().positive(),
  tipo_incidencia_id: z.string().uuid(),
  descripcion: z.string().min(10).max(1000),
  fecha_incidencia: z.string().datetime(),
  sancion_sugerida: z.string().max(250).optional(),
  evidencia_url: z.string().url().optional(),
})

export type QuejaInput = z.infer<typeof QuejaSchema>
export type TicketInput = z.infer<typeof TicketSchema>
export type MensajeInput = z.infer<typeof MensajeSchema>
export type IncidenciaInput = z.infer<typeof IncidenciaSchema>

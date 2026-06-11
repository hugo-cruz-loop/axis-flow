import { z } from 'zod'

// --- SHARED COMMONS ---
export const errorDetailsSchema = z.object({
  code: z.string(),
  message: z.string(),
  details: z.any().nullable().optional(),
})

// --- ADMIN EMPRESA METRIC MODELS ---
export const clientEvaluationItemSchema = z.object({
  cliente_id: z.string().uuid('Invalid Client UUID format'),
  cliente_nombre: z.string().min(1, 'Client name is required'),
  promedio_puntuacion: z.coerce.number().min(0).max(10, 'Evaluation score must be between 0 and 10'),
  total_evaluaciones: z.number().int().nonnegative(),
})

export const evaluationsResponseSchema = z.object({
  data: z.object({
    empresa_id: z.coerce.number(),
    evaluaciones: z.array(clientEvaluationItemSchema),
  }),
})

export const employeesCountResponseSchema = z.object({
  data: z.object({
    empresa_id: z.coerce.number(),
    total_empleados: z.number().int().nonnegative('Count must be a non-negative integer'),
  }),
})

export const activitiesCountResponseSchema = z.object({
  data: z.object({
    empresa_id: z.coerce.number(),
    total_actividades_finalizadas: z.number().int().nonnegative(),
    periodo: z.string(),
  }),
})

export const absencesCountResponseSchema = z.object({
  data: z.object({
    empresa_id: z.coerce.number(),
    total_ausencias: z.number().int().nonnegative(),
    year: z.number().int().nullable().optional(),
  }),
})

// --- CLIENTE METRIC MODELS ---
export const servicioSchema = z.object({
  servicio_id: z.coerce.number(),
  servicio_nombre: z.string().min(1),
  empleados_asignados: z.number().int().nonnegative(),
})

export const localidadServicioSchema = z.object({
  localidad_id: z.string().uuid(),
  localidad_nombre: z.string().min(1),
  servicios: z.array(servicioSchema),
})

export const servicesLocalidadResponseSchema = z.object({
  data: z.object({
    cliente_id: z.string().uuid(),
    localidades: z.array(localidadServicioSchema),
  }),
})

export const ticketBreakdownSchema = z.object({
  pendiente: z.number().int().nonnegative(),
  en_proceso: z.number().int().nonnegative(),
  finalizado: z.number().int().nonnegative(),
})

export const ticketStatusBreakdownResponseSchema = z.object({
  data: z.object({
    cliente_id: z.string().uuid(),
    tickets: ticketBreakdownSchema,
    total_tickets: z.number().int().nonnegative(),
  }),
})

// --- RH METRIC MODELS ---
export const vacanteSchema = z.object({
  vacante_id: z.string(),
  titulo: z.string(),
  departamento: z.string(),
  fecha_publicacion: z.string(),
})

export const totalTrabajosActivosResponseSchema = z.object({
  data: z.object({
    empresa_id: z.coerce.number(),
    total_vacantes_activas: z.number().int().nonnegative(),
    vacantes: z.array(vacanteSchema).optional().default([]),
  }),
})

export const totalAusenciaTrabajadoresResponseSchema = z.object({
  data: z.object({
    empresa_id: z.coerce.number(),
    tasa_absentismo: z.coerce.number().min(0).max(100),
    dias_laborables_totales: z.number().int().nonnegative(),
    total_inasistencias: z.number().int().nonnegative(),
    empleados_afectados: z.number().int().nonnegative(),
  }),
})

// Types for components (matching the fields they actually consume)
export type ClientEvaluationItem = z.infer<typeof clientEvaluationItemSchema>

export type ServicesLocalidadItem = {
  localidad_id: string
  localidad_nombre: string
  servicio_id: string
  servicio_nombre: string
  empleados_asignados: number
}

export type TicketStatusBreakdown = {
  cliente_id: string
  pendiente: number
  en_proceso: number
  finalizado: number
  total_tickets: number
  // support the plural forms expected by the spec
  pendientes: number
  finalizados: number
}

export type AbsenteeismMetrics = {
  empresa_id: number
  tasa_absentismo: number
  dias_laborables_totales: number
  total_inasistencias: number
  empleados_afectados: number
  total_ausencias_actuales: number
  comparativa_mensual?: Array<{
    mes: string
    inasistencias: number
  }>
}

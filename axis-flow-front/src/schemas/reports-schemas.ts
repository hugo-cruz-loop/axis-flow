import { z } from 'zod'

export const paginationSchema = z.object({
  page: z.coerce.number().int().min(1).default(1),
  limit: z.coerce.number().int().min(1).max(100).default(20),
})

export const coordinateSchema = z.object({
  latitude: z.coerce.number().min(-90).max(90),
  longitude: z.coerce.number().min(-180).max(180),
})

export const dateRangeSchema = z
  .object({
    startDate: z.string().regex(/^\d{4}-\d{2}-\d{2}$/),
    endDate: z.string().regex(/^\d{4}-\d{2}-\d{2}$/),
  })
  .refine((d) => new Date(d.startDate) <= new Date(d.endDate), {
    message: 'End date must be >= start date',
    path: ['endDate'],
  })

export const evidenciasQuerySchema = z.intersection(
  paginationSchema,
  z.object({
    clientId: z.string().uuid().optional(),
    fecha: z
      .string()
      .regex(/^\d{4}-\d{2}-\d{2}$/)
      .optional(),
    searchQuery: z.string().trim().max(100).optional(),
  }),
)

export const asistenciasQuerySchema = z.intersection(
  paginationSchema,
  z.object({
    employeeId: z.string().uuid().optional(),
    status: z.enum(['IN_TIME', 'LATE', 'ABSENT', 'EXCUSED']).optional(),
    dateRange: dateRangeSchema.optional(),
  }),
)

export const incidentDashboardQuerySchema = z.object({
  clientId: z.string().uuid().optional(),
  dateRange: dateRangeSchema.optional(),
  groupBy: z.enum(['day', 'week', 'month']).default('week'),
})

export const reverseGeocodingSchema = z.object({
  latitud: z.number().min(-90).max(90),
  longitud: z.number().min(-180).max(180),
})

export type EvidenciasQueryParams = z.infer<typeof evidenciasQuerySchema>
export type AsistenciasQueryParams = z.infer<typeof asistenciasQuerySchema>
export type IncidentDashboardQueryParams = z.infer<typeof incidentDashboardQuerySchema>
export type ReverseGeocodingInput = z.infer<typeof reverseGeocodingSchema>

import { z } from 'zod'

export const reportFormatSchema = z.enum(['pdf', 'xlsx'])
export type ReportFormat = z.infer<typeof reportFormatSchema>

export const reportFilterSchema = z
  .object({
    date_from: z.string().regex(/^\d{4}-\d{2}-\d{2}$/, 'Must be YYYY-MM-DD'),
    date_to: z.string().regex(/^\d{4}-\d{2}-\d{2}$/, 'Must be YYYY-MM-DD'),
    client_id: z.number().int().positive().optional(),
    employee_id: z.string().min(1).optional(),
  })
  .refine((data) => new Date(data.date_from) <= new Date(data.date_to), {
    message: 'Start date must be before or equal to End date',
    path: ['date_to'],
  })
export type ReportFilters = z.infer<typeof reportFilterSchema>

export const reportRunPayloadSchema = z.object({
  filters: reportFilterSchema,
  outputFormat: reportFormatSchema.default('pdf'),
})
export type ReportRunPayload = z.infer<typeof reportRunPayloadSchema>

export const reportRunResponseSchema = z.object({
  data: z.object({
    key: z.string().uuid(),
    download_url: z.string(),
    expiresAt: z.string(),
  }),
})
export type ReportRunResponse = z.infer<typeof reportRunResponseSchema>

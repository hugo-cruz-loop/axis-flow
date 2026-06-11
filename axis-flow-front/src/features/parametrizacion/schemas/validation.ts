import { z } from 'zod'

const int64IdSchema = z.number().int().positive()
const timestampSchema = z.string().datetime()
const dateSchema = z
  .string()
  .regex(/^\d{4}-\d{2}-\d{2}$/, 'Must select a valid date in YYYY-MM-DD format')
  .refine((value) => !Number.isNaN(Date.parse(`${value}T00:00:00`)), {
    message: 'Must select a valid date in YYYY-MM-DD format',
  })

export const errorEnvelopeSchema = z.object({
  success: z.literal(false),
  error: z.object({
    code: z.string(),
    message: z.string(),
    details: z.record(z.string(), z.array(z.string())).optional(),
  }),
})

export const serviceEvaluationFormSchema = z.object({
  empresa_id: int64IdSchema,
  servicio_id: int64IdSchema,
  periodicidad_id: int64IdSchema,
  activa: z.boolean().default(true),
})

export const personalEvaluationFormSchema = z.object({
  empresa_id: int64IdSchema,
  periodicidad_id: int64IdSchema,
  activa: z.boolean().default(true),
})

export const inactiveDayFormSchema = z.object({
  empresa_id: int64IdSchema,
  fecha: dateSchema,
  descripcion: z.string().max(255, 'Description cannot exceed 255 characters'),
})

export const inactiveDaysThresholdSchema = z.object({
  empresa_id: int64IdSchema,
  umbral_dias: z.number().int().min(0),
})

export const systemSettingFormSchema = z.object({
  clave_parametro: z
    .string()
    .min(1, 'Parameter key cannot be empty')
    .regex(/^[A-Z0-9_]+$/, 'Parameter key must be uppercase alphanumeric and underscores only'),
  valor: z.string().min(1, 'Parameter value cannot be empty'),
})

export const serviceEvaluationSchema = serviceEvaluationFormSchema.extend({
  id: int64IdSchema,
  created_at: timestampSchema,
  updated_at: timestampSchema,
})

export const personalEvaluationSchema = personalEvaluationFormSchema.extend({
  id: int64IdSchema,
  created_at: timestampSchema,
  updated_at: timestampSchema,
})

export const inactiveDaySchema = inactiveDayFormSchema.extend({
  id: int64IdSchema,
  created_at: timestampSchema,
  updated_at: timestampSchema,
})

export const companyInactiveDaysSchema = z.object({
  empresa_id: int64IdSchema,
  umbral_dias: z.number().int().min(0),
  dias_inactivos: z.array(inactiveDaySchema),
})

export const inactiveDaysThresholdResponseSchema = inactiveDaysThresholdSchema.extend({
  updated_at: timestampSchema,
})

export const systemSettingSchema = systemSettingFormSchema.extend({
  descripcion: z.string(),
  created_at: timestampSchema,
  updated_at: timestampSchema,
})

export function successEnvelopeSchema<TSchema extends z.ZodType>(schema: TSchema) {
  return z.object({ success: z.literal(true), data: schema })
}

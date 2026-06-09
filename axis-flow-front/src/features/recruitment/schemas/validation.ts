import { z } from 'zod'

const ACCEPTED_TYPES = [
  'application/pdf',
  'application/vnd.openxmlformats-officedocument.wordprocessingml.document',
]
const MAX_SIZE = 5 * 1024 * 1024

export const ApplicationSchema = z.object({
  trabajo_id: z.string().uuid(),
  nombre_completo: z.string().min(3).max(150),
  email: z.string().email(),
  telefono: z
    .string()
    .regex(/^\+?[0-9]{8,15}$/)
    .optional(),
  cv: z
    .any()
    .refine((f: FileList) => f?.length === 1, 'CV requerido')
    .refine((f: FileList) => f?.[0]?.size <= MAX_SIZE, 'Máximo 5MB')
    .refine(
      (f: FileList) => ACCEPTED_TYPES.includes(f?.[0]?.type),
      'Solo PDF o DOCX',
    ),
  turnstile_token: z.string().min(1, 'Captcha requerido'),
})

export const EvaluationSchema = z.object({
  postulacion_id: z.string().uuid(),
  puntualidad: z.number().int().min(1).max(5),
  cortesia: z.number().int().min(1).max(5),
  soft_skills: z.number().int().min(1).max(5),
  comentarios: z.string().max(1000).optional(),
})

export type ApplicationInput = z.infer<typeof ApplicationSchema>
export type EvaluationInput = z.infer<typeof EvaluationSchema>

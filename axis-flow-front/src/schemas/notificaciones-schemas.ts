import { z } from 'zod'

export const deviceTypeSchema = z.enum(['WEB', 'ANDROID', 'IOS'])
export const tokenRegistrationSchema = z.object({
  token: z.string().min(10),
  device_type: deviceTypeSchema,
})
export type TokenRegistrationInput = z.infer<typeof tokenRegistrationSchema>

export const wsEventEnum = z.enum([
  'notifica-send-gestor',
  'message.send.queja',
  'message.send.service',
])
export const wsEventPayloadSchema = z.object({
  event: wsEventEnum,
  payload: z.record(z.string(), z.unknown()),
})
export type WsEventEnvelope = z.infer<typeof wsEventPayloadSchema>

export const chatMessageSchema = z.object({
  id: z.string().uuid().optional(),
  ticket_id: z.number().int().min(1).optional(),
  queja_id: z.number().int().min(1).optional(),
  sender_id: z.string().uuid(),
  sender_name: z.string().min(1).max(100),
  sender_role: z.enum(['Cliente', 'Gestor', 'Empleado']),
  message: z.string().min(1).max(1000),
  timestamp: z.string().datetime(),
})
export type ChatMessageInput = z.infer<typeof chatMessageSchema>

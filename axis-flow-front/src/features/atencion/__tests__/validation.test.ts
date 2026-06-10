import { describe, it, expect } from 'vitest'
import { QuejaSchema, TicketSchema, MensajeSchema } from '../schemas/validation'

// — QuejaSchema —

describe('QuejaSchema', () => {
  const valid = {
    tipo_queja_id: 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
    titulo: 'Valid complaint title',
    descripcion: 'This is a valid description with enough characters.',
  }

  it('rejects when titulo is missing', () => {
    const result = QuejaSchema.safeParse({ ...valid, titulo: '' })
    expect(result.success).toBe(false)
  })

  it('rejects when titulo is too short (< 5 chars)', () => {
    const result = QuejaSchema.safeParse({ ...valid, titulo: 'Hi' })
    expect(result.success).toBe(false)
  })

  it('rejects when descripcion is less than 10 chars', () => {
    const result = QuejaSchema.safeParse({ ...valid, descripcion: 'Short' })
    expect(result.success).toBe(false)
  })

  it('rejects when tipo_queja_id is not a UUID', () => {
    const result = QuejaSchema.safeParse({ ...valid, tipo_queja_id: 'not-a-uuid' })
    expect(result.success).toBe(false)
  })

  it('accepts a valid queja', () => {
    const result = QuejaSchema.safeParse(valid)
    expect(result.success).toBe(true)
  })
})

// — TicketSchema —

describe('TicketSchema', () => {
  const valid = {
    localidad_id: 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
    asunto: 'Valid ticket subject',
    descripcion: 'This description is long enough to meet the fifteen character minimum.',
  }

  it('rejects when descripcion is less than 15 chars', () => {
    const result = TicketSchema.safeParse({ ...valid, descripcion: 'Too short.' })
    expect(result.success).toBe(false)
  })

  it('rejects when asunto is missing', () => {
    const result = TicketSchema.safeParse({ ...valid, asunto: '' })
    expect(result.success).toBe(false)
  })

  it('accepts a valid ticket', () => {
    const result = TicketSchema.safeParse(valid)
    expect(result.success).toBe(true)
  })
})

// — MensajeSchema —

describe('MensajeSchema', () => {
  it('rejects when mensaje is empty', () => {
    const result = MensajeSchema.safeParse({ mensaje: '' })
    expect(result.success).toBe(false)
  })

  it('accepts a valid message', () => {
    const result = MensajeSchema.safeParse({ mensaje: 'Hello there' })
    expect(result.success).toBe(true)
  })
})

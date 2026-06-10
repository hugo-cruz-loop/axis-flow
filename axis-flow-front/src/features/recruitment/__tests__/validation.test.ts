import { describe, it, expect } from 'vitest'
import { ApplicationSchema, EvaluationSchema } from '../schemas/validation'

// — ApplicationSchema tests (task 6.9) —

describe('ApplicationSchema', () => {
  const validBase = {
    trabajo_id: 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
    nombre_completo: 'Jane Doe',
    email: 'jane@example.com',
    turnstile_token: 'some-turnstile-token',
  }

  function makeFile(overrides?: Partial<{ size: number; type: string }>) {
    const defaults = { size: 1024, type: 'application/pdf' }
    const opts = { ...defaults, ...overrides }
    // FileList cannot be constructed directly in Node; use a mock object
    const file = { size: opts.size, type: opts.type } as File
    const fileList = { 0: file, length: 1 } as unknown as FileList
    return fileList
  }

  it('rejects when nombre_completo is missing', () => {
    const result = ApplicationSchema.safeParse({
      ...validBase,
      nombre_completo: '',
      cv: makeFile(),
    })
    expect(result.success).toBe(false)
  })

  it('rejects when email is invalid', () => {
    const result = ApplicationSchema.safeParse({
      ...validBase,
      email: 'not-an-email',
      cv: makeFile(),
    })
    expect(result.success).toBe(false)
  })

  it('rejects when trabajo_id is not a UUID', () => {
    const result = ApplicationSchema.safeParse({
      ...validBase,
      trabajo_id: 'not-a-uuid',
      cv: makeFile(),
    })
    expect(result.success).toBe(false)
  })

  it('rejects when cv exceeds 5MB', () => {
    const result = ApplicationSchema.safeParse({
      ...validBase,
      cv: makeFile({ size: 6 * 1024 * 1024 }),
    })
    expect(result.success).toBe(false)
  })

  it('rejects when cv is not PDF or DOCX', () => {
    const result = ApplicationSchema.safeParse({
      ...validBase,
      cv: makeFile({ type: 'image/png' }),
    })
    expect(result.success).toBe(false)
  })

  it('rejects when turnstile_token is empty', () => {
    const result = ApplicationSchema.safeParse({
      ...validBase,
      cv: makeFile(),
      turnstile_token: '',
    })
    expect(result.success).toBe(false)
  })

  it('accepts a valid application with DOCX file', () => {
    const result = ApplicationSchema.safeParse({
      ...validBase,
      cv: makeFile({
        type: 'application/vnd.openxmlformats-officedocument.wordprocessingml.document',
      }),
    })
    expect(result.success).toBe(true)
  })
})

// — EvaluationSchema tests (task 6.10) —

describe('EvaluationSchema', () => {
  const validBase = {
    postulacion_id: 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
    puntualidad: 3,
    cortesia: 4,
    soft_skills: 5,
  }

  it('rejects when puntualidad is 0', () => {
    const result = EvaluationSchema.safeParse({ ...validBase, puntualidad: 0 })
    expect(result.success).toBe(false)
  })

  it('rejects when cortesia is 6', () => {
    const result = EvaluationSchema.safeParse({ ...validBase, cortesia: 6 })
    expect(result.success).toBe(false)
  })

  it('rejects when soft_skills is 0', () => {
    const result = EvaluationSchema.safeParse({ ...validBase, soft_skills: 0 })
    expect(result.success).toBe(false)
  })

  it('rejects when soft_skills is 6', () => {
    const result = EvaluationSchema.safeParse({ ...validBase, soft_skills: 6 })
    expect(result.success).toBe(false)
  })

  it('accepts all scores at boundary values (1 and 5)', () => {
    const result = EvaluationSchema.safeParse({
      ...validBase,
      puntualidad: 1,
      cortesia: 5,
      soft_skills: 1,
    })
    expect(result.success).toBe(true)
  })

  it('rejects when postulacion_id is not a UUID', () => {
    const result = EvaluationSchema.safeParse({ ...validBase, postulacion_id: 'bad' })
    expect(result.success).toBe(false)
  })
})

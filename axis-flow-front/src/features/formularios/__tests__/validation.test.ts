import { describe, it, expect } from 'vitest'
import {
  formularioCreateSchema,
  formularioResponseSchema,
  preguntaCreateSchema,
  preguntaResponseSchema,
  eventoCreateSchema,
  eventoResponseSchema,
  eventoIniciadoCreateSchema,
  eventoIniciadoResponseSchema,
  respuestaCreateSchema,
  respuestaResponseSchema,
  geolocalizacionSchema,
  matrixConfigSchema,
  choiceConfigSchema,
  cameraConfigSchema,
  tipoPreguntaSchema,
} from '../schemas/validation'

const UUID = '8c2a7d0e-5412-4c22-b5e1-88f6c5bbde93'
const UUID2 = 'd041e2a8-0e1b-4d43-85f6-cbb18a4a5119'

// — formularioCreateSchema —

describe('formularioCreateSchema', () => {
  it('accepts a minimal valid form (only required fields)', () => {
    const r = formularioCreateSchema.safeParse({
      empresa_id: UUID,
      nombre: 'Limpieza de Baños',
    })
    expect(r.success).toBe(true)
    if (r.success) {
      // activo defaults to true
      expect(r.data.activo).toBe(true)
    }
  })

  it('rejects when empresa_id is not a UUID', () => {
    const r = formularioCreateSchema.safeParse({
      empresa_id: 'not-a-uuid',
      nombre: 'Test',
    })
    expect(r.success).toBe(false)
  })

  it('rejects when nombre is empty', () => {
    const r = formularioCreateSchema.safeParse({
      empresa_id: UUID,
      nombre: '',
    })
    expect(r.success).toBe(false)
  })

  it('rejects when nombre exceeds 150 chars', () => {
    const r = formularioCreateSchema.safeParse({
      empresa_id: UUID,
      nombre: 'x'.repeat(151),
    })
    expect(r.success).toBe(false)
  })
})

// — formularioResponseSchema —

describe('formularioResponseSchema', () => {
  it('parses a full envelope', () => {
    const r = formularioResponseSchema.safeParse({
      success: true,
      data: {
        id: UUID,
        empresa_id: UUID,
        nombre: 'Limpieza de Baños',
        descripcion: 'Verificacion diaria',
        activo: true,
        created_at: '2026-06-10T22:00:00Z',
        updated_at: '2026-06-10T22:00:00Z',
      },
    })
    expect(r.success).toBe(true)
  })

  it('rejects when success flag is false', () => {
    const r = formularioResponseSchema.safeParse({
      success: false,
      data: { id: UUID, empresa_id: UUID, nombre: 'x', activo: true },
    })
    expect(r.success).toBe(false)
  })
})

// — preguntaCreateSchema —

describe('preguntaCreateSchema', () => {
  const valid = {
    formulario_id: UUID,
    orden: 1,
    texto_pregunta: 'Limpieza de inodoros',
    tipo_pregunta: 1 as const,
  }

  it('accepts a minimal valid pregunta', () => {
    const r = preguntaCreateSchema.safeParse(valid)
    expect(r.success).toBe(true)
    if (r.success) {
      expect(r.data.obligatoria).toBe(false) // default
    }
  })

  it('rejects when tipo_pregunta is out of enum (99)', () => {
    const r = preguntaCreateSchema.safeParse({ ...valid, tipo_pregunta: 99 })
    expect(r.success).toBe(false)
  })

  it('accepts every tipo_pregunta in the supported set (1, 2, 3, 5, 8, 11)', () => {
    for (const tp of [1, 2, 3, 5, 8, 11]) {
      const r = preguntaCreateSchema.safeParse({ ...valid, tipo_pregunta: tp })
      expect(r.success).toBe(true)
    }
  })

  it('rejects when texto_pregunta is empty', () => {
    const r = preguntaCreateSchema.safeParse({ ...valid, texto_pregunta: '' })
    expect(r.success).toBe(false)
  })

  it('rejects when orden < 1', () => {
    const r = preguntaCreateSchema.safeParse({ ...valid, orden: 0 })
    expect(r.success).toBe(false)
  })

  it('rejects when orden is not an integer', () => {
    const r = preguntaCreateSchema.safeParse({ ...valid, orden: 1.5 })
    expect(r.success).toBe(false)
  })

  it('rejects when formulario_id is not a UUID', () => {
    const r = preguntaCreateSchema.safeParse({ ...valid, formulario_id: 'bad' })
    expect(r.success).toBe(false)
  })
})

// — preguntaResponseSchema —

describe('preguntaResponseSchema', () => {
  it('parses a full pregunta envelope', () => {
    const r = preguntaResponseSchema.safeParse({
      success: true,
      data: {
        id: UUID,
        formulario_id: UUID,
        orden: 1,
        texto_pregunta: 'Pregunta',
        tipo_pregunta: 1,
        obligatoria: false,
        created_at: '2026-06-10T22:00:00Z',
      },
    })
    expect(r.success).toBe(true)
  })
})

// — eventoCreateSchema —

describe('eventoCreateSchema', () => {
  it('accepts a minimal valid evento', () => {
    const r = eventoCreateSchema.safeParse({
      empresa_id: UUID,
      cliente_id: UUID2,
      nombre: 'Ronda Nocturna',
      fecha_programada: '2026-06-10T22:00:00Z',
    })
    expect(r.success).toBe(true)
  })

  it('rejects when fecha_programada is not ISO datetime', () => {
    const r = eventoCreateSchema.safeParse({
      empresa_id: UUID,
      cliente_id: UUID2,
      nombre: 'X',
      fecha_programada: 'not-a-date',
    })
    expect(r.success).toBe(false)
  })

  it('rejects when formularios_asociados contains a non-uuid', () => {
    const r = eventoCreateSchema.safeParse({
      empresa_id: UUID,
      cliente_id: UUID2,
      nombre: 'X',
      fecha_programada: '2026-06-10T22:00:00Z',
      formularios_asociados: ['bad-id'],
    })
    expect(r.success).toBe(false)
  })
})

// — eventoIniciadoCreateSchema —

describe('eventoIniciadoCreateSchema', () => {
  it('accepts without geolocation (optional)', () => {
    const r = eventoIniciadoCreateSchema.safeParse({
      evento_id: UUID,
      empleado_id: 5,
    })
    expect(r.success).toBe(true)
  })

  it('accepts with valid geolocation', () => {
    const r = eventoIniciadoCreateSchema.safeParse({
      evento_id: UUID,
      empleado_id: 5,
      geolocalizacion_inicio: { latitud: -34.6037, longitud: -58.3816 },
    })
    expect(r.success).toBe(true)
  })

  it('rejects when latitud > 90', () => {
    const r = eventoIniciadoCreateSchema.safeParse({
      evento_id: UUID,
      empleado_id: 5,
      geolocalizacion_inicio: { latitud: 91, longitud: 0 },
    })
    expect(r.success).toBe(false)
  })

  it('rejects when latitud < -90', () => {
    const r = eventoIniciadoCreateSchema.safeParse({
      evento_id: UUID,
      empleado_id: 5,
      geolocalizacion_inicio: { latitud: -91, longitud: 0 },
    })
    expect(r.success).toBe(false)
  })

  it('rejects when longitud > 180', () => {
    const r = eventoIniciadoCreateSchema.safeParse({
      evento_id: UUID,
      empleado_id: 5,
      geolocalizacion_inicio: { latitud: 0, longitud: 181 },
    })
    expect(r.success).toBe(false)
  })
})

// — respuestaCreateSchema —

describe('respuestaCreateSchema', () => {
  it('accepts a minimal valid respuesta', () => {
    const r = respuestaCreateSchema.safeParse({
      evento_iniciado_id: UUID,
      formulario_id: UUID,
      pregunta_id: UUID,
      respuesta_lista: { seleccion: [] },
    })
    expect(r.success).toBe(true)
  })

  it('rejects when respuesta_lista is missing', () => {
    const r = respuestaCreateSchema.safeParse({
      evento_iniciado_id: UUID,
      formulario_id: UUID,
      pregunta_id: UUID,
    })
    expect(r.success).toBe(false)
  })
})

// — geolocalizacionSchema —

describe('geolocalizacionSchema', () => {
  it('accepts valid lat/lng within range', () => {
    const r = geolocalizacionSchema.safeParse({ latitud: 45.0, longitud: -120.0 })
    expect(r.success).toBe(true)
  })

  it('rejects latitud = 91', () => {
    const r = geolocalizacionSchema.safeParse({ latitud: 91, longitud: 0 })
    expect(r.success).toBe(false)
  })

  it('rejects longitud = 181', () => {
    const r = geolocalizacionSchema.safeParse({ latitud: 0, longitud: 181 })
    expect(r.success).toBe(false)
  })
})

// — matrixConfigSchema —

describe('matrixConfigSchema', () => {
  it('accepts when rows and columns are non-empty arrays', () => {
    const r = matrixConfigSchema.safeParse({
      rows: ['Inodoros', 'Lavabos'],
      columns: ['Bueno', 'Regular', 'Malo'],
    })
    expect(r.success).toBe(true)
  })

  it('rejects when rows is empty', () => {
    const r = matrixConfigSchema.safeParse({
      rows: [],
      columns: ['Bueno'],
    })
    expect(r.success).toBe(false)
  })

  it('rejects when a row string is empty', () => {
    const r = matrixConfigSchema.safeParse({
      rows: [''],
      columns: ['Bueno'],
    })
    expect(r.success).toBe(false)
  })
})

// — choiceConfigSchema —

describe('choiceConfigSchema', () => {
  it('accepts when there are at least 2 non-empty options', () => {
    const r = choiceConfigSchema.safeParse({ options: ['A', 'B'] })
    expect(r.success).toBe(true)
  })

  it('rejects when only 1 option', () => {
    const r = choiceConfigSchema.safeParse({ options: ['A'] })
    expect(r.success).toBe(false)
  })

  it('rejects when an option is empty', () => {
    const r = choiceConfigSchema.safeParse({ options: ['A', ''] })
    expect(r.success).toBe(false)
  })
})

// — cameraConfigSchema —

describe('cameraConfigSchema', () => {
  it('accepts min_photos=0, max_photos=1', () => {
    const r = cameraConfigSchema.safeParse({ min_photos: 0, max_photos: 1 })
    expect(r.success).toBe(true)
  })

  it('rejects when max_photos > 3', () => {
    const r = cameraConfigSchema.safeParse({ min_photos: 0, max_photos: 4 })
    expect(r.success).toBe(false)
  })

  it('rejects when max_photos < 1', () => {
    const r = cameraConfigSchema.safeParse({ min_photos: 0, max_photos: 0 })
    expect(r.success).toBe(false)
  })
})

// — tipoPreguntaSchema —

describe('tipoPreguntaSchema', () => {
  it('accepts 1, 2, 3, 5, 8, 11', () => {
    for (const tp of [1, 2, 3, 5, 8, 11]) {
      const r = tipoPreguntaSchema.safeParse(tp)
      expect(r.success).toBe(true)
    }
  })

  it('rejects 4, 6, 7, 9, 10, 12, 99', () => {
    for (const tp of [4, 6, 7, 9, 10, 12, 99]) {
      const r = tipoPreguntaSchema.safeParse(tp)
      expect(r.success).toBe(false)
    }
  })
})

// — eventoResponseSchema / eventoIniciadoResponseSchema / respuestaResponseSchema —

describe('envelope schemas', () => {
  it('eventoResponseSchema accepts a valid evento', () => {
    const r = eventoResponseSchema.safeParse({
      success: true,
      data: {
        id: UUID,
        empresa_id: UUID,
        cliente_id: UUID2,
        nombre: 'Ronda',
        fecha_programada: '2026-06-10T22:00:00Z',
        estatus: 'pendiente',
      },
    })
    expect(r.success).toBe(true)
  })

  it('eventoIniciadoResponseSchema accepts a valid iniciado', () => {
    const r = eventoIniciadoResponseSchema.safeParse({
      success: true,
      data: {
        id: UUID,
        evento_id: UUID,
        empleado_id: 5,
        fecha_inicio: '2026-06-10T22:00:00Z',
        estatus: 'iniciado',
      },
    })
    expect(r.success).toBe(true)
  })

  it('respuestaResponseSchema accepts a valid respuesta', () => {
    const r = respuestaResponseSchema.safeParse({
      success: true,
      data: {
        id: UUID,
        evento_iniciado_id: UUID,
        formulario_id: UUID,
        pregunta_id: UUID,
        respuesta_lista: {},
      },
    })
    expect(r.success).toBe(true)
  })
})

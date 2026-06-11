import { describe, it, expect, vi, beforeAll, beforeEach, afterEach } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import { useFormularioCapture } from '../hooks/useFormularioCapture'
import type { PreguntaDraft } from '../types'

const INICIADO_ID = '11111111-2222-3333-4444-555555555555'
const DRAFT_KEY = `formularios:draft:${INICIADO_ID}`

// jsdom's localStorage is available in vitest's jsdom env but defensive polyfill
// keeps the test self-contained if the env ever changes.
const memoryStorage = (() => {
  const map = new Map<string, string>()
  return {
    getItem: (k: string) => (map.has(k) ? (map.get(k) as string) : null),
    setItem: (k: string, v: string) => {
      map.set(k, String(v))
    },
    removeItem: (k: string) => {
      map.delete(k)
    },
    clear: () => {
      map.clear()
    },
    key: (i: number) => Array.from(map.keys())[i] ?? null,
    get length() {
      return map.size
    },
  }
})()

beforeAll(() => {
  Object.defineProperty(window, 'localStorage', {
    value: memoryStorage,
    configurable: true,
    writable: true,
  })
})

const preguntas: PreguntaDraft[] = [
  {
    id: 'q1',
    orden: 1,
    texto_pregunta: 'Pregunta 1',
    tipo_pregunta: 1, // SHORT_TEXT
    obligatoria: true,
  },
  {
    id: 'q2',
    orden: 2,
    texto_pregunta: 'Pregunta 2',
    tipo_pregunta: 1, // SHORT_TEXT
    obligatoria: false,
  },
  {
    id: 'q3',
    orden: 3,
    texto_pregunta: 'Pregunta 3',
    tipo_pregunta: 1, // SHORT_TEXT
    obligatoria: false,
  },
]

beforeEach(() => {
  window.localStorage.clear()
})

afterEach(() => {
  window.localStorage.clear()
})

describe('useFormularioCapture', () => {
  it('starts at index 0 with empty answers and writes the draft to localStorage on mount', () => {
    const { result } = renderHook(() =>
      useFormularioCapture({ eventoIniciadoId: INICIADO_ID, preguntas }),
    )
    expect(result.current.index).toBe(0)
    expect(result.current.answers).toEqual({})
    expect(result.current.total).toBe(3)
    expect(window.localStorage.getItem(DRAFT_KEY)).not.toBeNull()
  })

  it('setAnswer stores a value and persists to localStorage', () => {
    const { result } = renderHook(() =>
      useFormularioCapture({ eventoIniciadoId: INICIADO_ID, preguntas }),
    )
    act(() => result.current.setAnswer('q1', 'Respuesta 1'))
    expect(result.current.answers.q1).toBe('Respuesta 1')
    const stored = JSON.parse(window.localStorage.getItem(DRAFT_KEY) ?? '{}')
    expect(stored.answers.q1).toBe('Respuesta 1')
  })

  it('restores answers from localStorage on mount', () => {
    window.localStorage.setItem(
      DRAFT_KEY,
      JSON.stringify({ answers: { q1: 'A', q2: 'B' }, index: 1 }),
    )
    const { result } = renderHook(() =>
      useFormularioCapture({ eventoIniciadoId: INICIADO_ID, preguntas }),
    )
    expect(result.current.answers).toEqual({ q1: 'A', q2: 'B' })
    expect(result.current.index).toBe(1)
  })

  it('next() advances index within bounds; does not advance past the last pregunta', () => {
    const { result } = renderHook(() =>
      useFormularioCapture({ eventoIniciadoId: INICIADO_ID, preguntas }),
    )
    act(() => result.current.next())
    act(() => result.current.next())
    expect(result.current.index).toBe(2)
    act(() => result.current.next())
    // Still at the last
    expect(result.current.index).toBe(2)
  })

  it('prev() goes back within bounds; does not go below 0', () => {
    const { result } = renderHook(() =>
      useFormularioCapture({ eventoIniciadoId: INICIADO_ID, preguntas }),
    )
    act(() => result.current.prev())
    expect(result.current.index).toBe(0)
  })

  it('clearDraft removes the localStorage entry and resets state', () => {
    const { result } = renderHook(() =>
      useFormularioCapture({ eventoIniciadoId: INICIADO_ID, preguntas }),
    )
    act(() => result.current.setAnswer('q1', 'X'))
    expect(window.localStorage.getItem(DRAFT_KEY)).not.toBeNull()
    act(() => result.current.clearDraft())
    expect(window.localStorage.getItem(DRAFT_KEY)).toBeNull()
    expect(result.current.answers).toEqual({})
    expect(result.current.index).toBe(0)
  })

  it('validateAll reports the pregunta IDs of required preguntas that are missing', () => {
    const { result } = renderHook(() =>
      useFormularioCapture({ eventoIniciadoId: INICIADO_ID, preguntas }),
    )
    // q1 is required and not yet answered
    const missing = result.current.validateAll()
    expect(missing).toContain('q1')
    expect(missing).not.toContain('q2')
    act(() => result.current.setAnswer('q1', 'ok'))
    expect(result.current.validateAll()).toEqual([])
  })

  it('submitAll calls the submit fn for each answered pregunta and clears the draft on success', async () => {
    const submit = vi.fn().mockResolvedValue(undefined)
    const { result } = renderHook(() =>
      useFormularioCapture({ eventoIniciadoId: INICIADO_ID, preguntas }),
    )
    act(() => result.current.setAnswer('q1', 'A'))
    act(() => result.current.setAnswer('q2', 'B'))
    await act(async () => {
      await result.current.submitAll(submit)
    })
    expect(submit).toHaveBeenCalledTimes(2)
    expect(submit).toHaveBeenNthCalledWith(1, preguntas[0], 'A')
    expect(submit).toHaveBeenNthCalledWith(2, preguntas[1], 'B')
    expect(window.localStorage.getItem(DRAFT_KEY)).toBeNull()
  })
})

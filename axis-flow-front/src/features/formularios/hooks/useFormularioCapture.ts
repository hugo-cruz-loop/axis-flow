import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import type { PreguntaDraft } from '../types'

interface UseFormularioCaptureParams {
  eventoIniciadoId: string
  preguntas: PreguntaDraft[]
}

interface UseFormularioCaptureResult {
  index: number
  total: number
  current: PreguntaDraft | null
  answers: Record<string, unknown>
  setAnswer: (preguntaId: string, value: unknown) => void
  next: () => void
  prev: () => void
  clearDraft: () => void
  validateAll: () => string[]
  submitAll: (submit: (p: PreguntaDraft, value: unknown) => Promise<void>) => Promise<void>
}

function draftKey(eventoIniciadoId: string): string {
  return `formularios:draft:${eventoIniciadoId}`
}

interface PersistedDraft {
  answers: Record<string, unknown>
  index: number
}

function loadDraft(eventoIniciadoId: string): PersistedDraft {
  if (typeof window === 'undefined') return { answers: {}, index: 0 }
  try {
    const raw = window.localStorage.getItem(draftKey(eventoIniciadoId))
    if (!raw) return { answers: {}, index: 0 }
    const parsed = JSON.parse(raw) as Partial<PersistedDraft>
    return {
      answers: (parsed.answers as Record<string, unknown>) ?? {},
      index: typeof parsed.index === 'number' ? parsed.index : 0,
    }
  } catch {
    return { answers: {}, index: 0 }
  }
}

function saveDraft(
  eventoIniciadoId: string,
  answers: Record<string, unknown>,
  index: number,
): void {
  if (typeof window === 'undefined') return
  try {
    window.localStorage.setItem(
      draftKey(eventoIniciadoId),
      JSON.stringify({ answers, index }),
    )
  } catch {
    // localStorage may be full or disabled — fail silently, the in-memory
    // state is the source of truth for the running session.
  }
}

function clearPersistedDraft(eventoIniciadoId: string): void {
  if (typeof window === 'undefined') return
  try {
    window.localStorage.removeItem(draftKey(eventoIniciadoId))
  } catch {
    // ignore
  }
}

/**
 * State machine for the mobile capture flow.
 *
 * - Tracks the current pregunta index and a map of preguntaId → answer.
 * - Persists the draft to localStorage so a partial capture survives a
 *   page refresh (keyed by evento_iniciado_id).
 * - `submitAll` calls the provided submit fn sequentially and clears
 *   the draft on success.
 */
export function useFormularioCapture(
  params: UseFormularioCaptureParams,
): UseFormularioCaptureResult {
  const { eventoIniciadoId, preguntas } = params

  // Initialize from localStorage exactly once on mount.
  const [answers, setAnswers] = useState<Record<string, unknown>>(() =>
    loadDraft(eventoIniciadoId).answers,
  )
  const [index, setIndex] = useState<number>(() => {
    const loaded = loadDraft(eventoIniciadoId)
    // Clamp to current preguntas length in case the form changed since last visit.
    return Math.min(loaded.index, Math.max(0, preguntas.length - 1))
  })

  // Track whether we just cleared so the persistence effect doesn't re-save
  // the empty state right after clearDraft() deletes the entry.
  const justClearedRef = useRef(false)

  // Persist on every state change.
  useEffect(() => {
    if (justClearedRef.current) {
      justClearedRef.current = false
      return
    }
    saveDraft(eventoIniciadoId, answers, index)
  }, [eventoIniciadoId, answers, index])

  const total = preguntas.length
  const current = index >= 0 && index < total ? (preguntas[index] ?? null) : null

  const setAnswer = useCallback((preguntaId: string, value: unknown) => {
    setAnswers((prev) => ({ ...prev, [preguntaId]: value }))
  }, [])

  const next = useCallback(() => {
    setIndex((i) => Math.min(i + 1, Math.max(0, total - 1)))
  }, [total])

  const prev = useCallback(() => {
    setIndex((i) => Math.max(i - 1, 0))
  }, [])

  const clearDraft = useCallback(() => {
    justClearedRef.current = true
    setAnswers({})
    setIndex(0)
    clearPersistedDraft(eventoIniciadoId)
  }, [eventoIniciadoId])

  const validateAll = useCallback((): string[] => {
    return preguntas
      .filter((p) => p.obligatoria)
      .filter((p) => {
        const v = answers[p.id]
        if (v === null || v === undefined) return true
        if (typeof v === 'string' && v.trim() === '') return true
        if (Array.isArray(v) && v.length === 0) return true
        return false
      })
      .map((p) => p.id)
  }, [preguntas, answers])

  const submitAll = useCallback(
    async (submit: (p: PreguntaDraft, value: unknown) => Promise<void>) => {
      for (const p of preguntas) {
        if (!(p.id in answers)) continue
        await submit(p, answers[p.id])
      }
      clearDraft()
    },
    [preguntas, answers, clearDraft],
  )

  return useMemo(
    () => ({
      index,
      total,
      current,
      answers,
      setAnswer,
      next,
      prev,
      clearDraft,
      validateAll,
      submitAll,
    }),
    [index, total, current, answers, setAnswer, next, prev, clearDraft, validateAll, submitAll],
  )
}

import { useId, useState } from 'react'
import { useLocation } from 'react-router-dom'
import { useCreateFormulario, useCreatePregunta } from '../api/queries'
import { TIPO_PREGUNTA, type FormularioDraft, type PreguntaDraft, type TipoPreguntaValue } from '../types'

interface ToolboxEntry {
  type: TipoPreguntaValue
  label: string
  ariaLabel: string
}

const TOOLBOX: ToolboxEntry[] = [
  { type: TIPO_PREGUNTA.SHORT_TEXT, label: 'Short text', ariaLabel: 'Add short text question' },
  { type: TIPO_PREGUNTA.CHECKBOX, label: 'Checkbox', ariaLabel: 'Add checkbox question' },
  { type: TIPO_PREGUNTA.RATING, label: 'Rating', ariaLabel: 'Add rating question' },
  { type: TIPO_PREGUNTA.MATRIX, label: 'Matrix', ariaLabel: 'Add matrix question' },
  { type: TIPO_PREGUNTA.CAMERA, label: 'Camera', ariaLabel: 'Add camera question' },
  { type: TIPO_PREGUNTA.SIGNATURE, label: 'Signature', ariaLabel: 'Add signature question' },
]

function makeDraft(type: TipoPreguntaValue, orden: number): PreguntaDraft {
  const base: PreguntaDraft = {
    id: crypto.randomUUID(),
    orden,
    texto_pregunta: `Question #${orden}`,
    tipo_pregunta: type,
    obligatoria: false,
  }
  if (type === TIPO_PREGUNTA.MATRIX) {
    base.respuesta_predefinida = { rows: ['Task 1'], columns: ['Good', 'Bad'] }
  } else if (type === TIPO_PREGUNTA.CAMERA) {
    base.respuesta_predefinida = { min_photos: 0, max_photos: 1 }
  }
  return base
}

export function FormBuilderPage() {
  const location = useLocation()
  const empresaId =
    (location.state as { empresaId?: string; empresa_id?: string } | null)?.empresa_id ??
    (location.state as { empresaId?: string; empresa_id?: string } | null)?.empresaId ??
    ''

  // Initialize empresa_id synchronously from location state (no effect needed).
  const [draft, setDraft] = useState<FormularioDraft>(() => ({
    empresa_id: empresaId,
    nombre: '',
    descripcion: '',
    activo: true,
    preguntas: [],
  }))
  const [selectedIdx, setSelectedIdx] = useState<number | null>(null)
  const [status, setStatus] = useState<{ kind: 'idle' | 'saving' | 'saved' | 'error'; msg?: string }>({
    kind: 'idle',
  })

  const createForm = useCreateFormulario()
  const createPregunta = useCreatePregunta()

  const nameId = useId()
  const descId = useId()

  function addQuestion(type: TipoPreguntaValue) {
    setDraft((d) => {
      const next = [...d.preguntas, makeDraft(type, d.preguntas.length + 1)]
      setSelectedIdx(next.length - 1)
      return { ...d, preguntas: next }
    })
  }

  function removeQuestion(idx: number) {
    setDraft((d) => {
      const next = d.preguntas.filter((_, i) => i !== idx)
      // Re-number to keep orden sequential
      const renum = next.map((p, i) => ({ ...p, orden: i + 1 }))
      return { ...d, preguntas: renum }
    })
    setSelectedIdx(null)
  }

  function moveQuestion(idx: number, dir: -1 | 1) {
    setDraft((d) => {
      const target = idx + dir
      if (target < 0 || target >= d.preguntas.length) return d
      const next = [...d.preguntas]
      const tmp = next[idx]
      if (!tmp) return d
      next[idx] = next[target]!
      next[target] = tmp
      const renum = next.map((p, i) => ({ ...p, orden: i + 1 }))
      return { ...d, preguntas: renum }
    })
  }

  async function handleSave() {
    if (!draft.nombre.trim() || draft.preguntas.length === 0) return
    setStatus({ kind: 'saving', msg: 'Saving form…' })
    createForm.mutate(
      {
        empresa_id: draft.empresa_id,
        nombre: draft.nombre,
        descripcion: draft.descripcion,
        activo: draft.activo,
      },
      {
        onSuccess: (resp) => {
          const newFormId = resp.data.id
          // Persist each pregunta sequentially. We don't await in a loop
          // because the order doesn't matter to the user — the form is
          // already saved.
          for (const p of draft.preguntas) {
            createPregunta.mutate({
              formulario_id: newFormId,
              orden: p.orden,
              texto_pregunta: p.texto_pregunta,
              tipo_pregunta: p.tipo_pregunta,
              obligatoria: p.obligatoria,
              respuesta_predefinida: p.respuesta_predefinida,
            })
          }
          setStatus({ kind: 'saved', msg: 'Saved.' })
        },
        onError: () => setStatus({ kind: 'error', msg: 'Save failed.' }),
      },
    )
  }

  const canSave = draft.nombre.trim().length > 0 && draft.preguntas.length > 0

  return (
    <div className="flex flex-col h-screen bg-slate-50 text-slate-800">
      <header className="flex justify-between items-center px-6 py-4 bg-white border-b border-slate-200">
        <div>
          <h1 className="text-xl font-bold tracking-tight text-slate-900">Form Designer</h1>
          <p className="text-xs text-slate-500">
            Design dynamic checklists and field surveys.
          </p>
        </div>
        <div className="flex items-center gap-3">
          <div role="status" aria-live="polite" className="text-xs text-slate-500 min-w-[6rem] text-right">
            {status.kind === 'saving' && 'Saving…'}
            {status.kind === 'saved' && 'Saved.'}
            {status.kind === 'error' && status.msg}
          </div>
          <button
            type="button"
            onClick={handleSave}
            disabled={!canSave || createForm.isPending}
            className="px-4 py-2 text-sm font-semibold text-white bg-indigo-600 rounded-md hover:bg-indigo-700 disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {createForm.isPending ? 'Saving…' : 'Save form'}
          </button>
        </div>
      </header>

      <div className="flex flex-1 overflow-hidden">
        <aside
          className="w-64 bg-white border-r border-slate-200 p-4 overflow-y-auto"
          aria-label="Toolbox panel"
        >
          <h2 className="text-xs font-bold text-slate-400 uppercase tracking-wider mb-4">
            Field types
          </h2>
          <div className="grid grid-cols-1 gap-2">
            {TOOLBOX.map((entry) => (
              <button
                key={entry.type}
                type="button"
                onClick={() => addQuestion(entry.type)}
                aria-label={entry.ariaLabel}
                className="flex items-center w-full px-3 py-2 text-sm text-left border border-slate-200 rounded-lg hover:bg-slate-50 hover:border-slate-300 font-medium transition"
              >
                <span className="mr-2" aria-hidden="true">🧩</span>
                {entry.label}
              </button>
            ))}
          </div>
        </aside>

        <main className="flex-1 p-6 overflow-y-auto bg-slate-100 flex flex-col items-center">
          <div className="w-full max-w-3xl space-y-6">
            <div className="bg-white p-6 rounded-xl border border-slate-200 shadow-sm space-y-4">
              <div>
                <label
                  htmlFor={nameId}
                  className="block text-xs font-semibold text-slate-500 uppercase"
                >
                  Form name
                </label>
                <input
                  id={nameId}
                  type="text"
                  value={draft.nombre}
                  onChange={(e) => setDraft((d) => ({ ...d, nombre: e.target.value }))}
                  placeholder="e.g. Daily Branch Janitorial Audit"
                  className="mt-1 block w-full text-lg font-bold border-b border-transparent hover:border-slate-200 focus:border-indigo-500 focus:outline-none py-1"
                />
              </div>
              <div>
                <label
                  htmlFor={descId}
                  className="block text-xs font-semibold text-slate-500 uppercase"
                >
                  Description (optional)
                </label>
                <textarea
                  id={descId}
                  rows={2}
                  value={draft.descripcion ?? ''}
                  onChange={(e) => setDraft((d) => ({ ...d, descripcion: e.target.value }))}
                  className="mt-1 block w-full text-sm border-b border-transparent hover:border-slate-200 focus:border-indigo-500 focus:outline-none py-1 resize-none"
                />
              </div>
            </div>

            <div className="space-y-3" aria-label="Form canvas">
              {draft.preguntas.length === 0 ? (
                <div className="flex flex-col items-center justify-center p-12 border-2 border-dashed border-slate-300 rounded-xl bg-white text-slate-400">
                  <span>📥 Your canvas is empty.</span>
                  <span className="text-xs mt-1 text-slate-400">
                    Click elements from the toolbox to build your form.
                  </span>
                </div>
              ) : (
                draft.preguntas.map((p, idx) => {
                  const isActive = selectedIdx === idx
                  return (
                    <div
                      key={p.id}
                      onClick={() => setSelectedIdx(idx)}
                      className={`relative bg-white p-5 rounded-xl border transition-all cursor-pointer ${
                        isActive
                          ? 'border-indigo-500 ring-2 ring-indigo-50/50 shadow-md'
                          : 'border-slate-200 hover:border-slate-300 shadow-sm'
                      }`}
                    >
                      <div className="flex justify-between items-start mb-2">
                        <span className="text-xs font-semibold text-indigo-600 bg-indigo-50 px-2 py-0.5 rounded">
                          Question {idx + 1}
                        </span>
                        <div className="flex gap-2">
                          <button
                            type="button"
                            onClick={(e) => {
                              e.stopPropagation()
                              moveQuestion(idx, -1)
                            }}
                            disabled={idx === 0}
                            aria-label={`Move question ${idx + 1} up`}
                            className="text-slate-400 hover:text-slate-700 disabled:opacity-30"
                          >
                            ▲
                          </button>
                          <button
                            type="button"
                            onClick={(e) => {
                              e.stopPropagation()
                              moveQuestion(idx, 1)
                            }}
                            disabled={idx === draft.preguntas.length - 1}
                            aria-label={`Move question ${idx + 1} down`}
                            className="text-slate-400 hover:text-slate-700 disabled:opacity-30"
                          >
                            ▼
                          </button>
                          <button
                            type="button"
                            onClick={(e) => {
                              e.stopPropagation()
                              removeQuestion(idx)
                            }}
                            aria-label={`Delete question ${idx + 1}`}
                            className="text-rose-400 hover:text-rose-600"
                          >
                            🗑️
                          </button>
                        </div>
                      </div>
                      <div className="font-semibold text-slate-800">
                        {p.texto_pregunta}
                        {p.obligatoria && <span className="text-rose-500 ml-1" aria-hidden="true">*</span>}
                      </div>
                    </div>
                  )
                })
              )}
            </div>
          </div>
        </main>
      </div>
    </div>
  )
}

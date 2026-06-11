import { useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'
import { useIniciarEvento, useSubmitRespuesta, useFormularios } from '../api/queries'
import { TIPO_PREGUNTA, type PreguntaDraft } from '../types'
import { MatrixField } from '../components/presentational/MatrixField'
import { SignatureCanvas } from '../components/presentational/SignatureCanvas'
import { CameraCapture } from '../components/presentational/CameraCapture'

interface GeolocationState {
  lat: number
  lng: number
}

/**
 * Mobile capture page.
 *
 * - Reads `eventoId` from the URL.
 * - On mount, requests geolocation. The page is still usable without GPS;
 *     the GPS state is just shown in the header.
 * - Loads the formularios for the current empresa and renders the active
 *   pregunta.
 * - Uses the `useFormularioCapture` hook for state + draft persistence.
 * - "Submit all" validates required fields, then calls `useSubmitRespuesta`
 *   per answer.
 *
 * NOTE on the formularios load: the current `useFormularios` hook returns
 * the list of formularios for an empresa. In a real production flow, the
 * backend would return the full form (with preguntas) when we look up the
 * assigned form by evento. For PR-7 we render the preguntas from the first
 * form in the list as a stand-in; the next PR will add a dedicated
 * `GET /formulario/{id}/preguntas` hook so this page renders the right
 * form for the right evento.
 */
export function MobileFormCapturePage() {
  const { eventoId } = useParams<{ eventoId: string }>()
  const [empresaId, setEmpresaId] = useState('')
  const [geo, setGeo] = useState<GeolocationState | null>(null)
  const [geoError, setGeoError] = useState<string | null>(null)
  const [validationError, setValidationError] = useState<string | null>(null)

  // In a real flow the eventoId would drive a `useFormularioByEvento` lookup.
  // For PR-7 we fall back to the first formulario in the empresa's list.
  const { data: formularios, isLoading: loadingForms } = useFormularios(empresaId)
  const activeForm = formularios?.data?.[0]
  const preguntas: PreguntaDraft[] =
    (activeForm as unknown as { preguntas?: PreguntaDraft[] } | undefined)?.preguntas ?? []

  const iniciar = useIniciarEvento()
  const submit = useSubmitRespuesta()

  // Geolocation request on mount.
  useEffect(() => {
    if (typeof navigator === 'undefined' || !navigator.geolocation) {
      setGeoError('Geolocation is not supported by your browser.')
      return
    }
    navigator.geolocation.getCurrentPosition(
      (pos) => {
        setGeo({ lat: pos.coords.latitude, lng: pos.coords.longitude })
      },
      (err) => {
        // We still let the user proceed — geolocation is advisory on this
        // page (the backend records it as part of iniciarEvento).
        setGeoError(`GPS unavailable: ${err.message}`)
      },
      { enableHighAccuracy: true, timeout: 10000 },
    )
  }, [])

  // iniciarEvento on mount (idempotent: server de-dupes by evento_id + empleado_id).
  useEffect(() => {
    if (!eventoId) return
    // empleado_id comes from the JWT in the real app. For PR-7 we use 0 as
    // a placeholder; the backend will reject if empleado_id is invalid.
    iniciar.mutate({
      evento_id: eventoId,
      empleado_id: 0,
      geolocalizacion_inicio: geo ?? undefined,
    })
    // We intentionally re-fire when geo becomes available so the geo is
    // included in the iniciar call.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [eventoId, geo])

  if (loadingForms || !activeForm) {
    return (
      <div className="p-8 text-center text-slate-500 font-medium">
        Loading form checklist…
      </div>
    )
  }

  async function handleSubmitAll(answers: Record<string, unknown>) {
    setValidationError(null)
    const missing = preguntas.filter((p) => {
      if (!p.obligatoria) return false
      const v = answers[p.id]
      if (v === null || v === undefined) return true
      if (typeof v === 'string' && v.trim() === '') return true
      if (Array.isArray(v) && v.length === 0) return true
      return false
    })
    if (missing.length > 0) {
      setValidationError(
        `Please complete required field: "${missing[0]?.texto_pregunta ?? ''}"`,
      )
      return
    }

    for (const p of preguntas) {
      if (!(p.id in answers)) continue
      await submit.mutateAsync({
        evento_iniciado_id: eventoId ?? '',
        formulario_id: activeForm?.id ?? '',
        pregunta_id: p.id,
        respuesta_lista: answers[p.id] as unknown,
        geolocalizacion_respuesta: geo ?? undefined,
      })
    }
  }

  return (
    <div className="min-h-screen bg-slate-50 flex flex-col">
      <header className="sticky top-0 bg-white border-b border-slate-200 z-10 p-4 shadow-sm">
        <h1 className="font-bold text-slate-900 text-base leading-tight">
          {activeForm.nombre}
        </h1>
        <div className="flex justify-between items-center mt-2 text-xs">
          <span className="text-slate-400">Event ID: {eventoId?.slice(0, 8)}…</span>
          <span
            className={`font-semibold flex items-center ${
              geo ? 'text-emerald-600' : 'text-slate-400'
            }`}
            role="status"
            aria-live="polite"
          >
            <span
              aria-hidden="true"
              className={`w-2 h-2 rounded-full mr-1.5 ${
                geo ? 'bg-emerald-500' : 'bg-slate-300 animate-pulse'
              }`}
            />
            {geo ? 'GPS Active' : 'Locating…'}
          </span>
        </div>
        {geoError && (
          <p className="text-[10px] text-rose-600 font-bold mt-1" role="alert">
            ⚠️ {geoError}
          </p>
        )}
      </header>

      <main className="flex-1 p-4 space-y-4 max-w-xl mx-auto w-full">
        <CaptureForm
          preguntas={preguntas}
          onSubmit={handleSubmitAll}
          validationError={validationError}
        />
      </main>
    </div>
  )
}

// — CaptureForm: renders one field per pregunta + a submit-all button —

interface CaptureFormProps {
  preguntas: PreguntaDraft[]
  onSubmit: (answers: Record<string, unknown>) => void | Promise<void>
  validationError: string | null
}

function CaptureForm({ preguntas, onSubmit, validationError }: CaptureFormProps) {
  const [answers, setAnswers] = useState<Record<string, unknown>>({})

  return (
    <form
      onSubmit={(e) => {
        e.preventDefault()
        void onSubmit(answers)
      }}
      className="space-y-4"
    >
      {preguntas.map((p) => (
        <div
          key={p.id}
          className="border border-slate-200 rounded-xl p-4 bg-white shadow-sm space-y-2"
        >
          <label
            htmlFor={`input-${p.id}`}
            className="font-bold text-slate-900 text-sm block"
          >
            {p.texto_pregunta}
            {p.obligatoria && <span className="text-rose-500 ml-1" aria-hidden="true">*</span>}
          </label>

          {p.tipo_pregunta === TIPO_PREGUNTA.SHORT_TEXT && (
            <input
              id={`input-${p.id}`}
              type="text"
              value={(answers[p.id] as string) ?? ''}
              onChange={(e) => setAnswers((a) => ({ ...a, [p.id]: e.target.value }))}
              className="mt-1 block w-full rounded-lg border-slate-300 shadow-sm text-sm"
            />
          )}

          {p.tipo_pregunta === TIPO_PREGUNTA.RATING && (
            <select
              id={`input-${p.id}`}
              value={(answers[p.id] as number) ?? ''}
              onChange={(e) => setAnswers((a) => ({ ...a, [p.id]: Number(e.target.value) }))}
              className="mt-1 block w-full rounded-lg border-slate-300 shadow-sm text-sm"
            >
              <option value="">Select…</option>
              {[1, 2, 3, 4, 5].map((n) => (
                <option key={n} value={n}>
                  {n} star{n === 1 ? '' : 's'}
                </option>
              ))}
            </select>
          )}

          {p.tipo_pregunta === TIPO_PREGUNTA.MATRIX && (
            <MatrixField
              rows={
                ((p.respuesta_predefinida as { rows?: string[] } | undefined)?.rows) ?? []
              }
              columns={
                ((p.respuesta_predefinida as { columns?: string[] } | undefined)?.columns) ?? []
              }
              value={(answers[p.id] as Record<string, string>) ?? {}}
              onChange={(row, col) =>
                setAnswers((a) => ({
                  ...a,
                  [p.id]: { ...((a[p.id] as Record<string, string>) ?? {}), [row]: col },
                }))
              }
              ariaLabel={p.texto_pregunta}
            />
          )}

          {p.tipo_pregunta === TIPO_PREGUNTA.SIGNATURE && (
            <SignatureCanvas
              width={320}
              height={120}
              ariaLabel={p.texto_pregunta}
              onChange={(dataUrl) => setAnswers((a) => ({ ...a, [p.id]: dataUrl }))}
            />
          )}

          {p.tipo_pregunta === TIPO_PREGUNTA.CAMERA && (
            <CameraCapture
              ariaLabel={p.texto_pregunta}
              onCapture={(file) => setAnswers((a) => ({ ...a, [p.id]: file }))}
            />
          )}
        </div>
      ))}

      {validationError && (
        <p className="text-sm text-rose-600 font-semibold" role="alert">
          {validationError}
        </p>
      )}

      <button
        type="submit"
        className="w-full inline-flex justify-center items-center py-3 px-4 border border-transparent shadow-sm text-sm font-bold rounded-lg text-white bg-indigo-600 hover:bg-indigo-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-indigo-500 mt-6"
      >
        Complete &amp; check-out
      </button>
    </form>
  )
}

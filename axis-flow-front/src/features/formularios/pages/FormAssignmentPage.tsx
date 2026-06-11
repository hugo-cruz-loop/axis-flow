import { useId, useState } from 'react'
import { useLocation } from 'react-router-dom'
import { useCreateEvento, useEventosByEmpCte, useFormularios } from '../api/queries'
import type { EventoCreate } from '../types'

export function FormAssignmentPage() {
  const location = useLocation()
  const empresaId =
    (location.state as { empresaId?: string } | null)?.empresa_id ??
    (location.state as { empresaId?: string } | null)?.empresaId ??
    ''
  const clienteId =
    (location.state as { clienteId?: string } | null)?.clienteId ??
    ''

  const { data: formularios, isLoading: loadingForms } = useFormularios(empresaId)
  const { data: eventos, isLoading: loadingEventos } = useEventosByEmpCte(empresaId, clienteId)
  const createEvento = useCreateEvento(empresaId, clienteId)

  const [selectedFormIds, setSelectedFormIds] = useState<string[]>([])
  const [fecha, setFecha] = useState('')
  const [status, setStatus] = useState<{ kind: 'idle' | 'saving' | 'saved' | 'error'; msg?: string }>({
    kind: 'idle',
  })

  const dateId = useId()

  function toggleForm(id: string) {
    setSelectedFormIds((prev) =>
      prev.includes(id) ? prev.filter((x) => x !== id) : [...prev, id],
    )
  }

  function handleAssign() {
    if (selectedFormIds.length === 0 || !fecha || !empresaId || !clienteId) return
    setStatus({ kind: 'saving', msg: 'Creating assignment…' })

    // Convert the local datetime to ISO UTC. The browser's `datetime-local`
    // input is naive (no timezone); we treat it as the operator's local time
    // and convert via the Date constructor so the backend receives a
    // proper ISO-8601 string with timezone.
    const iso = new Date(fecha).toISOString()

    const body: EventoCreate = {
      empresa_id: empresaId,
      cliente_id: clienteId,
      nombre: `Assignment ${new Date(iso).toLocaleDateString()}`,
      fecha_programada: iso,
      formularios_asociados: selectedFormIds,
    }

    createEvento.mutate(body, {
      onSuccess: () => {
        setStatus({ kind: 'saved', msg: 'Assigned.' })
        setSelectedFormIds([])
        setFecha('')
      },
      onError: () => setStatus({ kind: 'error', msg: 'Assignment failed.' }),
    })
  }

  const canAssign = selectedFormIds.length > 0 && fecha.length > 0 && !!empresaId && !!clienteId

  return (
    <div className="p-6 max-w-5xl mx-auto space-y-6">
      <header>
        <h1 className="text-2xl font-bold text-slate-900">Form assignment</h1>
        <p className="text-sm text-slate-500">
          Bind form templates to operational events. Field employees will see the assigned forms when they check in.
        </p>
      </header>

      <section aria-labelledby="existing-eventos-heading" className="space-y-3">
        <h2 id="existing-eventos-heading" className="text-sm font-bold text-slate-700">
          Existing eventos
        </h2>
        {loadingEventos ? (
          <p className="text-sm text-slate-500">Loading eventos…</p>
        ) : eventos?.data.length ? (
          <ul className="space-y-2">
            {eventos.data.map((e) => (
              <li
                key={e.id}
                className="bg-white border border-slate-200 rounded-lg p-3 flex justify-between items-center"
              >
                <div>
                  <p className="font-semibold text-slate-800">{e.nombre}</p>
                  <p className="text-xs text-slate-500">
                    Scheduled: {new Date(e.fecha_programada).toLocaleString()} • Status: {e.estatus}
                  </p>
                </div>
              </li>
            ))}
          </ul>
        ) : (
          <p className="text-sm text-slate-500">No eventos yet for this client.</p>
        )}
      </section>

      <section aria-labelledby="assign-heading" className="bg-white border border-slate-200 rounded-xl p-5 space-y-4">
        <h2 id="assign-heading" className="text-sm font-bold text-slate-700">
          New assignment
        </h2>

        <div>
          <p className="text-xs font-semibold text-slate-600 mb-2">Select forms</p>
          {loadingForms ? (
            <p className="text-sm text-slate-500">Loading forms…</p>
          ) : formularios?.data.length ? (
            <ul className="space-y-2">
              {formularios.data.map((f) => (
                <li key={f.id} className="flex items-center gap-2">
                  <input
                    type="checkbox"
                    id={`form-${f.id}`}
                    checked={selectedFormIds.includes(f.id)}
                    onChange={() => toggleForm(f.id)}
                    className="h-4 w-4 text-indigo-600 border-slate-300 rounded"
                  />
                  <label htmlFor={`form-${f.id}`} className="text-sm text-slate-800">
                    {f.nombre}
                  </label>
                </li>
              ))}
            </ul>
          ) : (
            <p className="text-sm text-slate-500">No forms available. Create one first.</p>
          )}
        </div>

        <div>
          <label htmlFor={dateId} className="block text-xs font-semibold text-slate-600 mb-1">
            Scheduled date
          </label>
          <input
            id={dateId}
            type="datetime-local"
            value={fecha}
            onChange={(e) => setFecha(e.target.value)}
            className="block w-full rounded-md border-slate-300 shadow-sm text-sm"
          />
        </div>

        <div className="flex items-center justify-between">
          <div role="status" aria-live="polite" className="text-xs text-slate-500">
            {status.kind === 'saving' && status.msg}
            {status.kind === 'saved' && status.msg}
            {status.kind === 'error' && status.msg}
          </div>
          <button
            type="button"
            onClick={handleAssign}
            disabled={!canAssign || createEvento.isPending}
            className="px-4 py-2 text-sm font-semibold text-white bg-indigo-600 rounded-md hover:bg-indigo-700 disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {createEvento.isPending ? 'Assigning…' : 'Assign'}
          </button>
        </div>
      </section>
    </div>
  )
}

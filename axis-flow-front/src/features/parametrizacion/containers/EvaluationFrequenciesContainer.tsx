import { useRef, useState } from 'react'
import { useCreatePersonalEvaluation, useCreateServiceEvaluation, usePersonalEvaluations, useServiceEvaluations } from '../api/queries'
import { AccessibleSwitch } from '../components/AccessibleSwitch'
import { ConfirmationModal } from '../components/ConfirmationModal'
import type { ServiceEvaluationForm } from '../types'

interface EvaluationFrequenciesContainerProps {
  empresaId: number
}

const CONFIRMATION_MESSAGE =
  'Warning: Activating or changing this frequency triggers an immediate mass email dispatch of survey questionnaires to all active customers subscribed to this service. Are you sure you want to continue?'

export function EvaluationFrequenciesContainer({ empresaId }: EvaluationFrequenciesContainerProps) {
  const serviceEvaluations = useServiceEvaluations(empresaId)
  const personalEvaluations = usePersonalEvaluations(empresaId)
  const createServiceEvaluation = useCreateServiceEvaluation()
  const createPersonalEvaluation = useCreatePersonalEvaluation()
  const submitButtonRef = useRef<HTMLButtonElement>(null)
  const [serviceForm, setServiceForm] = useState<ServiceEvaluationForm>({
    empresa_id: empresaId,
    servicio_id: 1,
    periodicidad_id: 1,
    activa: true,
  })
  const [personalForm, setPersonalForm] = useState({
    empresa_id: empresaId,
    periodicidad_id: 1,
    activa: true,
  })
  const [pendingServiceForm, setPendingServiceForm] = useState<ServiceEvaluationForm | null>(null)
  const [status, setStatus] = useState('')

  function requestServiceConfirmation(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setPendingServiceForm({ ...serviceForm, empresa_id: empresaId })
  }

  function confirmServiceEvaluation() {
    if (!pendingServiceForm) return
    createServiceEvaluation.mutate(pendingServiceForm, {
      onSuccess: () => setStatus('Service evaluation frequency saved. Cache refreshed.'),
    })
    setPendingServiceForm(null)
  }

  function savePersonalEvaluation(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    createPersonalEvaluation.mutate({ ...personalForm, empresa_id: empresaId }, {
      onSuccess: () => setStatus('Personal evaluation frequency saved. Cache refreshed.'),
    })
  }

  return (
    <section className="space-y-6" aria-labelledby="evaluation-frequencies-title">
      <div>
        <h1 id="evaluation-frequencies-title" className="text-2xl font-bold text-slate-900">
          Evaluation Frequencies
        </h1>
        <p className="mt-1 text-sm text-slate-500">
          Configure service and employee evaluation cadence for this tenant.
        </p>
      </div>
      <div role="status" aria-live="polite" className="min-h-5 text-sm text-slate-600">
        {status}
        {createServiceEvaluation.isPending || createPersonalEvaluation.isPending ? 'Saving and refreshing cache…' : ''}
      </div>
      <div className="grid gap-6 lg:grid-cols-2">
        <form onSubmit={requestServiceConfirmation} className="rounded-xl border border-slate-200 bg-white p-5 shadow-sm">
          <h2 className="text-lg font-semibold text-slate-900">Service Evaluation Form</h2>
          <div className="mt-4 space-y-4">
            <label className="block text-sm font-medium text-slate-700">
              Service ID
              <input
                type="number"
                min={1}
                value={serviceForm.servicio_id}
                onChange={(event) => setServiceForm((current) => ({ ...current, servicio_id: Number(event.target.value) }))}
                className="mt-1 block w-full rounded-md border border-slate-300 px-3 py-2 focus:outline-none focus-visible:ring-2 focus-visible:ring-indigo-500"
              />
            </label>
            <label className="block text-sm font-medium text-slate-700">
              Service periodicity ID
              <input
                type="number"
                min={1}
                value={serviceForm.periodicidad_id}
                onChange={(event) =>
                  setServiceForm((current) => ({ ...current, periodicidad_id: Number(event.target.value) }))
                }
                className="mt-1 block w-full rounded-md border border-slate-300 px-3 py-2 focus:outline-none focus-visible:ring-2 focus-visible:ring-indigo-500"
              />
            </label>
            <div className="flex items-center justify-between rounded-lg bg-slate-50 px-3 py-2">
              <span className="text-sm font-medium text-slate-700">Active</span>
              <AccessibleSwitch
                checked={serviceForm.activa}
                label="Service evaluation active"
                onChange={(checked) => setServiceForm((current) => ({ ...current, activa: checked }))}
              />
            </div>
          </div>
          <button
            ref={submitButtonRef}
            type="submit"
            className="mt-5 w-full rounded-md bg-indigo-600 px-4 py-2 text-sm font-semibold text-white hover:bg-indigo-700 focus:outline-none focus-visible:ring-2 focus-visible:ring-indigo-500 focus-visible:ring-offset-2"
          >
            Save service evaluation frequency
          </button>
          <p className="mt-4 text-xs text-slate-500">
            Configured service evaluations: {serviceEvaluations.data?.length ?? 0}
          </p>
        </form>

        <form onSubmit={savePersonalEvaluation} className="rounded-xl border border-slate-200 bg-white p-5 shadow-sm">
          <h2 className="text-lg font-semibold text-slate-900">Personal Evaluation Form</h2>
          <div className="mt-4 space-y-4">
            <label className="block text-sm font-medium text-slate-700">
              Personal periodicity ID
              <input
                type="number"
                min={1}
                value={personalForm.periodicidad_id}
                onChange={(event) =>
                  setPersonalForm((current) => ({ ...current, periodicidad_id: Number(event.target.value) }))
                }
                className="mt-1 block w-full rounded-md border border-slate-300 px-3 py-2 focus:outline-none focus-visible:ring-2 focus-visible:ring-indigo-500"
              />
            </label>
            <div className="flex items-center justify-between rounded-lg bg-slate-50 px-3 py-2">
              <span className="text-sm font-medium text-slate-700">Active</span>
              <AccessibleSwitch
                checked={personalForm.activa}
                label="Personal evaluation active"
                onChange={(checked) => setPersonalForm((current) => ({ ...current, activa: checked }))}
              />
            </div>
          </div>
          <button
            type="submit"
            className="mt-5 w-full rounded-md border border-indigo-600 px-4 py-2 text-sm font-semibold text-indigo-700 hover:bg-indigo-50 focus:outline-none focus-visible:ring-2 focus-visible:ring-indigo-500 focus-visible:ring-offset-2"
          >
            Save personal evaluation frequency
          </button>
          <p className="mt-4 text-xs text-slate-500">
            Configured personal evaluations: {personalEvaluations.data?.length ?? 0}
          </p>
        </form>
      </div>
      <ConfirmationModal
        open={pendingServiceForm !== null}
        title="Confirm service evaluation change"
        message={CONFIRMATION_MESSAGE}
        confirmLabel="Confirm frequency change"
        cancelLabel="Cancel frequency change"
        onConfirm={confirmServiceEvaluation}
        onCancel={() => setPendingServiceForm(null)}
        returnFocusRef={submitButtonRef}
      />
    </section>
  )
}

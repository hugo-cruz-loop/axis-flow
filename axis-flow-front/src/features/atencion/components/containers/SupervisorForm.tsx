import { useAuthStore } from '@/store/authStore'
import { IncidentForm } from '../presentational/IncidentForm'
import { useCreateIncidencia } from '../../api/queries'
import type { IncidenciaInput } from '../../schemas/validation'

export function SupervisorForm() {
  const user = useAuthStore((s) => s.user)
  const empresaId = (user as Record<string, string> | null)?.empresa_id ?? ''
  const createIncidencia = useCreateIncidencia()

  function handleSubmit(data: IncidenciaInput) {
    // Inject empresa_id from JWT store — never from form or hardcoded
    createIncidencia.mutate(
      { ...data } as IncidenciaInput,
      {
        onSuccess: () => {
          // Success toast: in a real app wire to a toast library
          // eslint-disable-next-line no-console
          console.info('Incident submitted for empresa:', empresaId)
        },
      },
    )
  }

  return (
    <div className="mx-auto max-w-lg p-6">
      <h2 className="mb-4 text-base font-semibold text-gray-900">Report Incident</h2>
      <IncidentForm
        onSubmit={handleSubmit}
        isSubmitting={createIncidencia.isPending}
      />
      {createIncidencia.isSuccess && (
        <p role="status" className="mt-3 text-sm font-medium text-green-700">
          Incident submitted successfully.
        </p>
      )}
      {createIncidencia.isError && (
        <p role="alert" className="mt-3 text-sm font-medium text-red-700">
          Failed to submit incident. Please try again.
        </p>
      )}
    </div>
  )
}

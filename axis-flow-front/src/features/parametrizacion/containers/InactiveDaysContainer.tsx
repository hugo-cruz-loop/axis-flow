import { useState } from 'react'
import { useCreateInactiveDay, useDeleteInactiveDay, useInactiveDays, useUpdateInactiveDaysThreshold } from '../api/queries'
import { InactiveDaysCalendar } from '../components/InactiveDaysCalendar'

interface InactiveDaysContainerProps {
  empresaId: number
  initialMonth?: Date
}

export function InactiveDaysContainer({ empresaId, initialMonth }: InactiveDaysContainerProps) {
  const inactiveDays = useInactiveDays(empresaId)
  const createInactiveDay = useCreateInactiveDay()
  const deleteInactiveDay = useDeleteInactiveDay(empresaId)
  const updateThreshold = useUpdateInactiveDaysThreshold()
  const [threshold, setThreshold] = useState(5)

  const currentThreshold = inactiveDays.data?.umbral_dias ?? threshold

  function saveThreshold(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    updateThreshold.mutate({ empresa_id: empresaId, umbral_dias: threshold })
  }

  return (
    <section className="space-y-6" aria-labelledby="inactive-days-title">
      <div>
        <h1 id="inactive-days-title" className="text-2xl font-bold text-slate-900">
          Operational Calendar
        </h1>
        <p className="mt-1 text-sm text-slate-500">
          Mark holidays, company events, and other inactive operating days.
        </p>
      </div>
      <form onSubmit={saveThreshold} className="flex flex-wrap items-end gap-3 rounded-xl border border-slate-200 bg-white p-4 shadow-sm">
        <label className="text-sm font-medium text-slate-700">
          Consecutive inactive days threshold
          <input
            type="number"
            min={0}
            value={threshold}
            onChange={(event) => setThreshold(Number(event.target.value))}
            className="mt-1 block w-40 rounded-md border border-slate-300 px-3 py-2 focus:outline-none focus-visible:ring-2 focus-visible:ring-indigo-500"
          />
        </label>
        <button
          type="submit"
          className="rounded-md bg-indigo-600 px-4 py-2 text-sm font-semibold text-white hover:bg-indigo-700 focus:outline-none focus-visible:ring-2 focus-visible:ring-indigo-500"
        >
          Save threshold
        </button>
        <span className="text-sm text-slate-500">Current threshold: {currentThreshold}</span>
      </form>
      {inactiveDays.isLoading ? (
        <p className="text-sm text-slate-500">Loading inactive days…</p>
      ) : (
        <InactiveDaysCalendar
          inactiveDays={inactiveDays.data?.dias_inactivos ?? []}
          initialMonth={initialMonth}
          onCreateInactiveDay={(fecha) =>
            createInactiveDay.mutate({ empresa_id: empresaId, fecha, descripcion: 'Inactive day' })
          }
          onDeleteInactiveDay={(id) => deleteInactiveDay.mutate(id)}
        />
      )}
    </section>
  )
}

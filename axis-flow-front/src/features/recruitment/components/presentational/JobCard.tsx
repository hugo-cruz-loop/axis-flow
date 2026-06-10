import type { Trabajo } from '../../types'

interface JobCardProps {
  trabajo: Trabajo
  onApply: () => void
}

const ESTATUS_LABEL: Record<number, string> = {
  1: 'Active',
  2: 'Paused',
  3: 'Closed',
}

export function JobCard({ trabajo, onApply }: JobCardProps) {
  const formattedDate = new Date(trabajo.fecha_caducar).toLocaleDateString()

  return (
    <article className="rounded-lg border border-gray-200 bg-white p-4 shadow-sm">
      <div className="mb-2 flex items-start justify-between gap-2">
        <h3 className="text-base font-semibold text-gray-900">{trabajo.titulo}</h3>
        <span
          className={`shrink-0 rounded-full px-2 py-0.5 text-xs font-medium ${
            trabajo.estatus_vacante === 1
              ? 'bg-green-100 text-green-800'
              : trabajo.estatus_vacante === 2
                ? 'bg-yellow-100 text-yellow-800'
                : 'bg-gray-100 text-gray-600'
          }`}
        >
          {ESTATUS_LABEL[trabajo.estatus_vacante]}
        </span>
      </div>

      <p className="mb-3 text-sm text-gray-600 line-clamp-2">{trabajo.descripcion}</p>

      <div className="mb-3 flex flex-wrap gap-1">
        {trabajo.requisitos.map((req) => (
          <span
            key={req}
            className="rounded-full bg-blue-50 px-2 py-0.5 text-xs text-blue-700"
          >
            {req}
          </span>
        ))}
      </div>

      <div className="flex items-center justify-between">
        <span className="text-xs text-gray-400">Expires: {formattedDate}</span>
        <button
          type="button"
          onClick={onApply}
          disabled={trabajo.estatus_vacante !== 1}
          className="rounded-md bg-blue-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-50"
        >
          Apply
        </button>
      </div>
    </article>
  )
}

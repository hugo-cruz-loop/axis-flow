import { SLABadge } from './SLABadge'
import { ESTATUS_LABELS } from '../../types'
import type { SolicitudQueja } from '../../types'

interface ComplaintCardProps {
  queja: SolicitudQueja
  onClick: () => void
}

const ESTATUS_COLOR: Record<number, string> = {
  1: 'bg-yellow-100 text-yellow-800',
  2: 'bg-blue-100 text-blue-800',
  3: 'bg-gray-100 text-gray-600',
}

export function ComplaintCard({ queja, onClick }: ComplaintCardProps) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="w-full rounded-lg border border-gray-200 bg-white p-4 text-left shadow-sm hover:border-blue-300 hover:shadow-md transition-shadow"
    >
      <div className="mb-2 flex items-start justify-between gap-2">
        <h3 className="text-sm font-semibold text-gray-900 line-clamp-1">{queja.titulo}</h3>
        <span
          className={`shrink-0 rounded-full px-2 py-0.5 text-xs font-medium ${ESTATUS_COLOR[queja.estatus]}`}
        >
          {ESTATUS_LABELS[queja.estatus]}
        </span>
      </div>

      <div className="flex items-center gap-2">
        {queja.fecha_vigencia && (
          <SLABadge fechaVigencia={queja.fecha_vigencia} estatus={queja.estatus} />
        )}
        <span className="text-xs text-gray-400">
          {new Date(queja.created_at).toLocaleDateString()}
        </span>
      </div>
    </button>
  )
}

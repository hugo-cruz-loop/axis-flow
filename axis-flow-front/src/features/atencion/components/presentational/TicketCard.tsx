import { ESTATUS_LABELS } from '../../types'
import type { TicketServicio } from '../../types'

interface TicketCardProps {
  ticket: TicketServicio
  onClick: () => void
}

const ESTATUS_COLOR: Record<number, string> = {
  1: 'bg-yellow-100 text-yellow-800',
  2: 'bg-blue-100 text-blue-800',
  3: 'bg-gray-100 text-gray-600',
}

const ULTIMA_RESP_LABEL: Record<number, string> = {
  1: 'Waiting for client',
  2: 'Waiting for support',
}

export function TicketCard({ ticket, onClick }: TicketCardProps) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="w-full rounded-lg border border-gray-200 bg-white p-4 text-left shadow-sm hover:border-blue-300 hover:shadow-md transition-shadow"
    >
      <div className="mb-2 flex items-start justify-between gap-2">
        <h3 className="text-sm font-semibold text-gray-900 line-clamp-1">{ticket.asunto}</h3>
        <span
          className={`shrink-0 rounded-full px-2 py-0.5 text-xs font-medium ${ESTATUS_COLOR[ticket.estatus]}`}
        >
          {ESTATUS_LABELS[ticket.estatus]}
        </span>
      </div>

      <div className="flex items-center justify-between">
        <span className="text-xs text-gray-500">
          {ULTIMA_RESP_LABEL[ticket.ultima_resp]}
        </span>
        <span className="text-xs text-gray-400">
          {new Date(ticket.created_at).toLocaleDateString()}
        </span>
      </div>
    </button>
  )
}

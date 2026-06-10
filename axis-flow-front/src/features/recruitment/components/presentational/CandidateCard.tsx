import type { Postulacion } from '../../types'

const ESTATUS_LABEL: Record<number, string> = {
  1: 'New',
  2: 'In review',
  3: 'Interview',
  4: 'Offer',
  5: 'Rejected',
}

const ESTATUS_COLOR: Record<number, string> = {
  1: 'bg-gray-100 text-gray-700',
  2: 'bg-blue-100 text-blue-700',
  3: 'bg-yellow-100 text-yellow-700',
  4: 'bg-green-100 text-green-700',
  5: 'bg-red-100 text-red-700',
}

interface CandidateCardProps {
  postulacion: Postulacion
  onEvaluate: () => void
  onMove: (estatus: number) => void
}

export function CandidateCard({ postulacion, onEvaluate, onMove }: CandidateCardProps) {
  const nextEstatus = postulacion.estatus < 5 ? postulacion.estatus + 1 : null

  return (
    <div role="listitem" className="rounded-md border border-gray-200 bg-white p-3 shadow-sm">
      <div className="mb-1 flex items-center justify-between gap-2">
        <span className="text-sm font-medium text-gray-900">{postulacion.nombre_completo}</span>
        <span
          className={`rounded-full px-2 py-0.5 text-xs font-medium ${ESTATUS_COLOR[postulacion.estatus]}`}
        >
          {ESTATUS_LABEL[postulacion.estatus]}
        </span>
      </div>

      <a
        href={postulacion.cv_url}
        target="_blank"
        rel="noopener noreferrer"
        className="mb-2 block text-xs text-blue-600 underline hover:text-blue-800"
      >
        View CV
      </a>

      <div className="flex gap-2">
        <button
          type="button"
          onClick={onEvaluate}
          className="rounded bg-gray-100 px-2 py-1 text-xs font-medium text-gray-700 hover:bg-gray-200"
        >
          Evaluate
        </button>
        {nextEstatus !== null && (
          <button
            type="button"
            onClick={() => onMove(nextEstatus)}
            className="rounded bg-blue-50 px-2 py-1 text-xs font-medium text-blue-700 hover:bg-blue-100"
            aria-label={`Move to ${ESTATUS_LABEL[nextEstatus]}`}
          >
            Move →
          </button>
        )}
      </div>
    </div>
  )
}

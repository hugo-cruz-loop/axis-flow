import type { StageStat, Postulacion } from '../../types'
import { CandidateCard } from './CandidateCard'

interface PipelineColumnProps {
  stage: StageStat
  candidates: Postulacion[]
  onMove: (candidateId: string, newEstatus: number) => void
  onDragStart?: (id: string) => void
  onDragOver?: (e: React.DragEvent) => void
  onDrop?: (e: React.DragEvent, estatus: number) => void
}

export function PipelineColumn({
  stage,
  candidates,
  onMove,
  onDragStart,
  onDragOver,
  onDrop,
}: PipelineColumnProps) {
  return (
    <div
      className="flex min-h-[200px] w-56 shrink-0 flex-col rounded-lg border border-gray-200 bg-gray-50"
      onDragOver={onDragOver}
      onDrop={(e) => onDrop?.(e, stage.stage)}
    >
      <div className="flex items-center justify-between border-b border-gray-200 px-3 py-2">
        <h4 className="text-sm font-semibold text-gray-700">{stage.stage_name}</h4>
        <span className="rounded-full bg-gray-200 px-2 py-0.5 text-xs text-gray-600">
          {stage.count}
        </span>
      </div>

      <ul
        role="list"
        aria-label={`${stage.stage_name} candidates`}
        className="flex flex-col gap-2 p-2"
      >
        {candidates.map((candidate) => (
          <li
            key={candidate.id}
            draggable
            onDragStart={() => onDragStart?.(candidate.id)}
            className="cursor-grab active:cursor-grabbing"
          >
            <CandidateCard
              postulacion={candidate}
              onEvaluate={() => {}}
              onMove={(estatus) => onMove(candidate.id, estatus)}
            />
          </li>
        ))}
      </ul>
    </div>
  )
}

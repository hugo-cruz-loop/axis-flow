import { usePostulacionStats, useUpdateCandidateStatus } from '../../api/queries'
import { PipelineColumn } from '../presentational/PipelineColumn'
import { PipelineColumnSkeleton } from '../presentational/Skeletons'
import { useDragAndDrop } from '../../hooks/useDragAndDrop'
import type { Postulacion } from '../../types'

interface CandidatePipelineProps {
  trabajoId: string
  candidates: Postulacion[]
}

export function CandidatePipeline({ trabajoId, candidates }: CandidatePipelineProps) {
  const { data: pipelineStats, isLoading } = usePostulacionStats(trabajoId)
  const { mutate: updateStatus } = useUpdateCandidateStatus()

  const handleDrop = (candidateId: string, newEstatus: number) => {
    updateStatus({ id: candidateId, estatus: newEstatus })
  }

  const { onDragStart, onDragOver, onDrop } = useDragAndDrop(handleDrop)

  if (isLoading) {
    return (
      <div className="flex gap-4 overflow-x-auto pb-4">
        {Array.from({ length: 5 }, (_, i) => (
          <PipelineColumnSkeleton key={i} />
        ))}
      </div>
    )
  }

  const stats = pipelineStats?.stats ?? []

  return (
    <div className="flex gap-4 overflow-x-auto pb-4">
      {stats.map((stage) => {
        const stageCandidates = candidates.filter((c) => c.estatus === stage.stage)
        return (
          <PipelineColumn
            key={stage.stage}
            stage={stage}
            candidates={stageCandidates}
            onMove={(candidateId, newEstatus) => updateStatus({ id: candidateId, estatus: newEstatus })}
            onDragStart={onDragStart}
            onDragOver={onDragOver}
            onDrop={onDrop}
          />
        )
      })}
    </div>
  )
}

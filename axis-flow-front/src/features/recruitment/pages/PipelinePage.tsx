import { useParams } from 'react-router-dom'
import { CandidatePipeline } from '../components/containers/CandidatePipeline'

export function PipelinePage() {
  const { trabajoId } = useParams<{ trabajoId: string }>()

  if (!trabajoId) return <p className="p-8 text-gray-500">Job not found.</p>

  return (
    <main className="px-4 py-8">
      <h1 className="mb-6 text-xl font-bold text-gray-900">Candidate pipeline</h1>
      {/* Candidates are fetched at page level in a real app via a dedicated query;
          passing an empty array here so the column layout renders from pipeline stats. */}
      <CandidatePipeline trabajoId={trabajoId} candidates={[]} />
    </main>
  )
}

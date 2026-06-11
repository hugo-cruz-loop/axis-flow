import { useAuthStore } from '@/store/authStore'
import { useJobsByEmpresa } from '../../api/queries'
import { JobCard } from '../presentational/JobCard'
import { JobCardSkeleton } from '../presentational/Skeletons'
import { useNavigate } from 'react-router-dom'

export function RecruiterDashboard() {
  const user = useAuthStore((s) => s.user)
  const empresaId = (user as { empresa_id?: string } | null)?.empresa_id ?? ''
  const navigate = useNavigate()

  const { data, isLoading, isError } = useJobsByEmpresa(empresaId)

  const handleCreateJob = () => {
    // In a real implementation this would open a form/modal.
    // Keeping the handler wired so the button is functional.
    console.warn('Create job: open form not yet implemented')
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-bold text-gray-900">Job postings</h1>
        <button
          type="button"
          onClick={handleCreateJob}
          className="rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-50"
        >
          + New job
        </button>
      </div>

      {isError && (
        <p className="text-sm text-red-600">Failed to load jobs. Please try again.</p>
      )}

      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {isLoading
          ? Array.from({ length: 3 }, (_, i) => <JobCardSkeleton key={i} />)
          : data?.data.map((trabajo) => (
              <JobCard
                key={trabajo.id}
                trabajo={trabajo}
                onApply={() => navigate(`/recruiter/pipeline/${trabajo.id}`)}
              />
            ))}
      </div>
    </div>
  )
}

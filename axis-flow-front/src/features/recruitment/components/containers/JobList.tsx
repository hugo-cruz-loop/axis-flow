import { useState } from 'react'
import { useActiveJobs } from '../../api/queries'
import { JobCard } from '../presentational/JobCard'
import { JobCardSkeleton } from '../presentational/Skeletons'
import { JobDetails } from './JobDetails'
import type { Trabajo } from '../../types'

export function JobList() {
  const [search, setSearch] = useState('')
  const [page] = useState(1)
  const [selectedJob, setSelectedJob] = useState<Trabajo | null>(null)

  const { data, isLoading, isError } = useActiveJobs(search || undefined, page)

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-3">
        <input
          type="search"
          placeholder="Search jobs…"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          className="flex-1 rounded-md border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
        />
      </div>

      {isError && (
        <p className="text-sm text-red-600">Failed to load jobs. Please try again.</p>
      )}

      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {isLoading
          ? Array.from({ length: 6 }, (_, i) => <JobCardSkeleton key={i} />)
          : data?.data.map((trabajo) => (
              <JobCard
                key={trabajo.id}
                trabajo={trabajo}
                onApply={() => setSelectedJob(trabajo)}
              />
            ))}
      </div>

      {selectedJob && (
        <JobDetails
          trabajo={selectedJob}
          onClose={() => setSelectedJob(null)}
        />
      )}
    </div>
  )
}

import { JobList } from '../components/containers/JobList'

export function JobBoardPage() {
  return (
    <main className="mx-auto max-w-5xl px-4 py-8">
      <h1 className="mb-6 text-2xl font-bold text-gray-900">Job board</h1>
      <JobList />
    </main>
  )
}

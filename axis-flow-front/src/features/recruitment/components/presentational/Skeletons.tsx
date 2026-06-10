export function JobCardSkeleton() {
  return (
    <div className="animate-pulse rounded-lg border border-gray-200 bg-white p-4 shadow-sm">
      <div className="mb-2 flex items-start justify-between">
        <div className="h-4 w-3/5 rounded bg-gray-200" />
        <div className="h-4 w-16 rounded-full bg-gray-200" />
      </div>
      <div className="mb-3 space-y-1">
        <div className="h-3 w-full rounded bg-gray-200" />
        <div className="h-3 w-4/5 rounded bg-gray-200" />
      </div>
      <div className="mb-3 flex gap-1">
        <div className="h-4 w-12 rounded-full bg-gray-200" />
        <div className="h-4 w-16 rounded-full bg-gray-200" />
        <div className="h-4 w-10 rounded-full bg-gray-200" />
      </div>
      <div className="flex items-center justify-between">
        <div className="h-3 w-24 rounded bg-gray-200" />
        <div className="h-7 w-20 rounded-md bg-gray-200" />
      </div>
    </div>
  )
}

export function PipelineColumnSkeleton() {
  return (
    <div className="animate-pulse w-56 shrink-0 rounded-lg border border-gray-200 bg-gray-50">
      <div className="flex items-center justify-between border-b border-gray-200 px-3 py-2">
        <div className="h-4 w-24 rounded bg-gray-200" />
        <div className="h-4 w-6 rounded-full bg-gray-200" />
      </div>
      <div className="flex flex-col gap-2 p-2">
        {[1, 2, 3].map((i) => (
          <div key={i} className="h-20 rounded-md bg-gray-200" />
        ))}
      </div>
    </div>
  )
}

export function CandidateCardSkeleton() {
  return (
    <div className="animate-pulse rounded-md border border-gray-200 bg-white p-3 shadow-sm">
      <div className="mb-1 flex items-center justify-between">
        <div className="h-4 w-32 rounded bg-gray-200" />
        <div className="h-4 w-16 rounded-full bg-gray-200" />
      </div>
      <div className="mb-2 h-3 w-20 rounded bg-gray-200" />
      <div className="flex gap-2">
        <div className="h-6 w-16 rounded bg-gray-200" />
        <div className="h-6 w-14 rounded bg-gray-200" />
      </div>
    </div>
  )
}

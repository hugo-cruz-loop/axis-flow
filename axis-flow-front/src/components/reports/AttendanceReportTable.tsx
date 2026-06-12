import type { AttendanceRecord } from '@/api/reports-service'

interface AttendanceReportTableProps {
  records: AttendanceRecord[]
  isLoading: boolean
  currentPage: number
  totalPages: number
  onPageChange: (page: number) => void
  onShowCoordinates: (record: AttendanceRecord) => void
}

// ─── Status badge ─────────────────────────────────────────────────────────────

const STATUS_STYLES: Record<AttendanceRecord['status'], string> = {
  IN_TIME: 'text-green-700 bg-green-50 border-green-200',
  LATE: 'text-amber-700 bg-amber-50 border-amber-200',
  ABSENT: 'text-red-700 bg-red-50 border-red-200',
  EXCUSED: 'text-slate-700 bg-slate-50 border-slate-200',
}

const STATUS_LABELS: Record<AttendanceRecord['status'], string> = {
  IN_TIME: 'In Time',
  LATE: 'Late',
  ABSENT: 'Absent',
  EXCUSED: 'Excused',
}

function StatusBadge({ status }: { status: AttendanceRecord['status'] }) {
  return (
    <span
      className={`inline-flex items-center px-2 py-0.5 text-xs font-medium rounded border ${STATUS_STYLES[status]}`}
    >
      {STATUS_LABELS[status]}
    </span>
  )
}

// ─── Skeleton rows ────────────────────────────────────────────────────────────

function SkeletonRows() {
  return (
    <>
      {Array.from({ length: 5 }).map((_, i) => (
        <tr key={i} aria-hidden="true">
          {Array.from({ length: 7 }).map((_, j) => (
            <td key={j} className="px-4 py-3">
              <div className="bg-slate-200 animate-pulse h-4 rounded w-full" />
            </td>
          ))}
        </tr>
      ))}
    </>
  )
}

// ─── Main component ───────────────────────────────────────────────────────────

export function AttendanceReportTable({
  records,
  isLoading,
  currentPage,
  totalPages,
  onPageChange,
  onShowCoordinates,
}: AttendanceReportTableProps) {
  return (
    <div className="space-y-4">
      {/* Accessible live region for pagination status */}
      <div aria-live="polite" className="sr-only" role="status">
        {!isLoading && `Page ${currentPage} of ${totalPages}, ${records.length} records`}
      </div>

      <div className="overflow-x-auto rounded-lg border border-slate-200">
        <table className="min-w-full divide-y divide-slate-200 text-sm">
          <thead className="bg-slate-50">
            <tr>
              <th scope="col" className="px-4 py-3 text-left font-semibold text-slate-700">
                Employee
              </th>
              <th scope="col" className="px-4 py-3 text-left font-semibold text-slate-700">
                Code
              </th>
              <th scope="col" className="px-4 py-3 text-left font-semibold text-slate-700">
                Clock In
              </th>
              <th scope="col" className="px-4 py-3 text-left font-semibold text-slate-700">
                Clock Out
              </th>
              <th scope="col" className="px-4 py-3 text-left font-semibold text-slate-700">
                Status
              </th>
              <th scope="col" className="px-4 py-3 text-left font-semibold text-slate-700">
                Delay (min)
              </th>
              <th scope="col" className="px-4 py-3 text-left font-semibold text-slate-700">
                Location
              </th>
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-100 bg-white">
            {isLoading ? (
              <SkeletonRows />
            ) : records.length === 0 ? (
              <tr>
                <td
                  colSpan={7}
                  className="px-4 py-8 text-center text-slate-500"
                >
                  No attendance records found.
                </td>
              </tr>
            ) : (
              records.map((record) => (
                <tr key={record.id} className="hover:bg-slate-50 transition-colors">
                  <td className="px-4 py-3 text-slate-800">{record.empleado_nombre}</td>
                  <td className="px-4 py-3 text-slate-600 font-mono text-xs">
                    {record.empleado_codigo}
                  </td>
                  <td className="px-4 py-3 text-slate-600">
                    {new Date(record.clock_in).toLocaleString()}
                  </td>
                  <td className="px-4 py-3 text-slate-600">
                    {record.clock_out ? new Date(record.clock_out).toLocaleString() : '—'}
                  </td>
                  <td className="px-4 py-3">
                    <StatusBadge status={record.status} />
                  </td>
                  <td className="px-4 py-3 text-slate-600">
                    {record.delay_minutes > 0 ? `+${record.delay_minutes}` : '—'}
                  </td>
                  <td className="px-4 py-3">
                    <button
                      className="text-blue-600 hover:underline text-xs focus-visible:ring-2 focus-visible:ring-blue-500 rounded"
                      onClick={() => onShowCoordinates(record)}
                      aria-label={`Show ${record.empleado_nombre} coordinates on map`}
                    >
                      View map
                    </button>
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      {/* Pagination */}
      {totalPages > 1 && (
        <nav
          className="flex items-center justify-between"
          aria-label="Attendance table pagination"
        >
          <button
            className="px-3 py-1.5 text-sm rounded border border-slate-300 hover:bg-slate-50 disabled:opacity-40 focus-visible:ring-2 focus-visible:ring-blue-500"
            onClick={() => onPageChange(currentPage - 1)}
            disabled={currentPage <= 1 || isLoading}
            aria-label="Previous page"
          >
            ← Previous
          </button>

          <span className="text-sm text-slate-600">
            Page {currentPage} of {totalPages}
          </span>

          <button
            className="px-3 py-1.5 text-sm rounded border border-slate-300 hover:bg-slate-50 disabled:opacity-40 focus-visible:ring-2 focus-visible:ring-blue-500"
            onClick={() => onPageChange(currentPage + 1)}
            disabled={currentPage >= totalPages || isLoading}
            aria-label="Next page"
          >
            Next →
          </button>
        </nav>
      )}
    </div>
  )
}

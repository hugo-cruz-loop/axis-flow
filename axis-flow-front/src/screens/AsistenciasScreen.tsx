import { useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { AttendanceReportTable } from '@/components/reports/AttendanceReportTable'
import { GeocodingMapInline } from '@/components/reports/GeocodingMapInline'
import { useAsistencias, useReverseGeocoding } from '@/hooks/use-reports'
import { asistenciasQuerySchema } from '@/schemas/reports-schemas'
import type { AttendanceRecord } from '@/api/reports-service'

const STATUS_OPTIONS = ['IN_TIME', 'LATE', 'ABSENT', 'EXCUSED'] as const

export function AsistenciasScreen() {
  const [searchParams, setSearchParams] = useSearchParams()
  const [selectedRecord, setSelectedRecord] = useState<{
    record: AttendanceRecord
    address?: string
  } | null>(null)

  // Parse URL params
  const rawParams = {
    page: searchParams.get('page') ?? undefined,
    limit: searchParams.get('limit') ?? undefined,
    employeeId: searchParams.get('employeeId') ?? undefined,
    status: searchParams.get('status') ?? undefined,
  }

  const parseResult = asistenciasQuerySchema.safeParse(rawParams)
  const params = parseResult.success
    ? parseResult.data
    : { page: 1, limit: 20 }

  const { data, isLoading } = useAsistencias(params)
  const geocodeMutation = useReverseGeocoding()

  const records = data?.data ?? []
  const meta = data?.meta

  function handleShowCoordinates(record: AttendanceRecord) {
    setSelectedRecord({ record })
    geocodeMutation.mutate(
      { latitud: record.latitud, longitud: record.longitud },
      {
        onSuccess: (result) => {
          setSelectedRecord((prev) =>
            prev ? { ...prev, address: result.data.direccion } : prev,
          )
        },
      },
    )
  }

  function handlePageChange(newPage: number) {
    const next = new URLSearchParams(searchParams)
    next.set('page', String(newPage))
    setSearchParams(next)
  }

  return (
    <main className="p-6 space-y-6">
      <h1 className="text-2xl font-bold text-slate-800">Attendance Reports</h1>

      {/* Filters */}
      <div className="flex flex-wrap gap-4 bg-slate-50 p-4 rounded-lg border border-slate-200">
        <div className="flex flex-col gap-1">
          <label htmlFor="status-filter" className="text-xs font-medium text-slate-600">
            Status
          </label>
          <select
            id="status-filter"
            className="border border-slate-300 rounded px-2 py-1 text-sm focus-visible:ring-2 focus-visible:ring-blue-500"
            value={searchParams.get('status') ?? ''}
            onChange={(e) => {
              const next = new URLSearchParams(searchParams)
              if (e.target.value) next.set('status', e.target.value)
              else next.delete('status')
              next.set('page', '1')
              setSearchParams(next)
            }}
          >
            <option value="">All statuses</option>
            {STATUS_OPTIONS.map((s) => (
              <option key={s} value={s}>
                {s.replace('_', ' ')}
              </option>
            ))}
          </select>
        </div>

        <div className="flex flex-col gap-1">
          <label htmlFor="employee-filter" className="text-xs font-medium text-slate-600">
            Employee ID
          </label>
          <input
            id="employee-filter"
            type="text"
            placeholder="UUID…"
            className="border border-slate-300 rounded px-2 py-1 text-sm font-mono focus-visible:ring-2 focus-visible:ring-blue-500"
            defaultValue={searchParams.get('employeeId') ?? ''}
            onBlur={(e) => {
              const next = new URLSearchParams(searchParams)
              if (e.target.value.trim()) next.set('employeeId', e.target.value.trim())
              else next.delete('employeeId')
              next.set('page', '1')
              setSearchParams(next)
            }}
          />
        </div>
      </div>

      <AttendanceReportTable
        records={records}
        isLoading={isLoading}
        currentPage={meta?.page ?? 1}
        totalPages={meta?.total_pages ?? 1}
        onPageChange={handlePageChange}
        onShowCoordinates={handleShowCoordinates}
      />

      {/* Coordinates map panel */}
      {selectedRecord && (
        <section className="mt-4" aria-label="Employee location map">
          <div className="flex items-center justify-between mb-2">
            <h2 className="text-sm font-semibold text-slate-700">
              Location — {selectedRecord.record.empleado_nombre}
            </h2>
            <button
              className="text-xs text-slate-400 hover:text-slate-600 focus-visible:ring-2 focus-visible:ring-blue-500 rounded"
              onClick={() => setSelectedRecord(null)}
              aria-label="Close location map"
            >
              Close
            </button>
          </div>
          <GeocodingMapInline
            latitude={selectedRecord.record.latitud}
            longitude={selectedRecord.record.longitud}
            geocodedAddress={selectedRecord.address}
          />
        </section>
      )}
    </main>
  )
}

import { useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { EvidenceGallery } from '@/components/reports/EvidenceGallery'
import { GeocodingMapInline } from '@/components/reports/GeocodingMapInline'
import { useEvidencias, useReverseGeocoding } from '@/hooks/use-reports'
import { evidenciasQuerySchema } from '@/schemas/reports-schemas'

export function EvidenciasScreen() {
  const [searchParams, setSearchParams] = useSearchParams()
  const [selectedCoords, setSelectedCoords] = useState<{
    lat: number
    lng: number
    address?: string
  } | null>(null)

  // Parse URL params — fall back gracefully on invalid values
  const rawParams = {
    page: searchParams.get('page') ?? undefined,
    limit: searchParams.get('limit') ?? undefined,
    clientId: searchParams.get('clientId') ?? undefined,
    fecha: searchParams.get('fecha') ?? undefined,
    searchQuery: searchParams.get('searchQuery') ?? undefined,
  }

  const parseResult = evidenciasQuerySchema.safeParse(rawParams)
  const params = parseResult.success
    ? parseResult.data
    : { page: 1, limit: 20, clientId: undefined, fecha: undefined, searchQuery: undefined }

  const { data, isLoading } = useEvidencias(params)
  const geocodeMutation = useReverseGeocoding()

  const evidencias = data?.data ?? []
  const meta = data?.meta

  function handleInspectLocation(lat: number, lng: number) {
    setSelectedCoords({ lat, lng })
    geocodeMutation.mutate(
      { latitud: lat, longitud: lng },
      {
        onSuccess: (result) => {
          setSelectedCoords((prev) =>
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
      <h1 className="text-2xl font-bold text-slate-800">Evidence Reports</h1>

      {/* Filters */}
      <div className="flex flex-wrap gap-4 bg-slate-50 p-4 rounded-lg border border-slate-200">
        <div className="flex flex-col gap-1">
          <label htmlFor="fecha-filter" className="text-xs font-medium text-slate-600">
            Date
          </label>
          <input
            id="fecha-filter"
            type="date"
            className="border border-slate-300 rounded px-2 py-1 text-sm focus-visible:ring-2 focus-visible:ring-blue-500"
            value={searchParams.get('fecha') ?? ''}
            onChange={(e) => {
              const next = new URLSearchParams(searchParams)
              if (e.target.value) next.set('fecha', e.target.value)
              else next.delete('fecha')
              next.set('page', '1')
              setSearchParams(next)
            }}
          />
        </div>

        <div className="flex flex-col gap-1">
          <label htmlFor="search-filter" className="text-xs font-medium text-slate-600">
            Search
          </label>
          <input
            id="search-filter"
            type="text"
            placeholder="Activity or employee…"
            className="border border-slate-300 rounded px-2 py-1 text-sm focus-visible:ring-2 focus-visible:ring-blue-500"
            defaultValue={searchParams.get('searchQuery') ?? ''}
            onBlur={(e) => {
              const next = new URLSearchParams(searchParams)
              if (e.target.value.trim()) next.set('searchQuery', e.target.value.trim())
              else next.delete('searchQuery')
              next.set('page', '1')
              setSearchParams(next)
            }}
          />
        </div>
      </div>

      {/* Pagination status (accessible live region) */}
      <div aria-live="polite" className="sr-only" role="status">
        {!isLoading && meta
          ? `Showing page ${meta.page} of ${meta.total_pages}, ${meta.total_records} results`
          : ''}
      </div>

      {meta && (
        <p className="text-sm text-slate-500">
          Showing page {meta.page} of {meta.total_pages} — {meta.total_records} results
        </p>
      )}

      <EvidenceGallery
        evidencias={evidencias}
        isLoading={isLoading}
        onInspectLocation={handleInspectLocation}
      />

      {/* Pagination */}
      {meta && meta.total_pages > 1 && (
        <nav className="flex items-center gap-4" aria-label="Evidence pagination">
          <button
            className="px-3 py-1.5 text-sm rounded border border-slate-300 hover:bg-slate-50 disabled:opacity-40 focus-visible:ring-2 focus-visible:ring-blue-500"
            onClick={() => handlePageChange(params.page - 1)}
            disabled={params.page <= 1 || isLoading}
            aria-label="Previous page"
          >
            ← Previous
          </button>
          <button
            className="px-3 py-1.5 text-sm rounded border border-slate-300 hover:bg-slate-50 disabled:opacity-40 focus-visible:ring-2 focus-visible:ring-blue-500"
            onClick={() => handlePageChange(params.page + 1)}
            disabled={params.page >= meta.total_pages || isLoading}
            aria-label="Next page"
          >
            Next →
          </button>
        </nav>
      )}

      {/* Map inspect panel */}
      {selectedCoords && (
        <section className="mt-4" aria-label="Location map">
          <div className="flex items-center justify-between mb-2">
            <h2 className="text-sm font-semibold text-slate-700">Location Preview</h2>
            <button
              className="text-xs text-slate-400 hover:text-slate-600 focus-visible:ring-2 focus-visible:ring-blue-500 rounded"
              onClick={() => setSelectedCoords(null)}
              aria-label="Close location map"
            >
              Close
            </button>
          </div>
          <GeocodingMapInline
            latitude={selectedCoords.lat}
            longitude={selectedCoords.lng}
            geocodedAddress={selectedCoords.address}
          />
        </section>
      )}
    </main>
  )
}

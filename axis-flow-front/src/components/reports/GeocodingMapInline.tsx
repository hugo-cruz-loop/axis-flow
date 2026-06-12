interface GeocodingMapInlineProps {
  latitude: number
  longitude: number
  geocodedAddress?: string
  zoom?: number
}

const GOOGLE_MAPS_KEY = import.meta.env.VITE_GOOGLE_MAPS_KEY as string | undefined

export function GeocodingMapInline({
  latitude,
  longitude,
  geocodedAddress,
  zoom = 15,
}: GeocodingMapInlineProps) {
  const mapsUrl = `https://www.google.com/maps?q=${latitude},${longitude}`

  const hasKey = Boolean(GOOGLE_MAPS_KEY)

  return (
    <div className="rounded-lg border border-slate-200 overflow-hidden">
      {hasKey ? (
        <StaticMapImage
          latitude={latitude}
          longitude={longitude}
          zoom={zoom}
          apiKey={GOOGLE_MAPS_KEY!}
          fallbackUrl={mapsUrl}
        />
      ) : (
        <MapFallback mapsUrl={mapsUrl} />
      )}

      {/* Address overlay */}
      <div className="p-3 bg-white border-t border-slate-100">
        {geocodedAddress === undefined ? (
          <div
            className="flex items-center gap-2 text-sm text-slate-500"
            aria-busy="true"
          >
            <svg
              className="animate-spin h-4 w-4 text-slate-400"
              viewBox="0 0 24 24"
              fill="none"
              aria-hidden="true"
            >
              <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
              <path
                className="opacity-75"
                fill="currentColor"
                d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"
              />
            </svg>
            <span>Loading address…</span>
          </div>
        ) : geocodedAddress ? (
          <p className="text-sm text-slate-700">
            <span className="font-medium">Address:</span> {geocodedAddress}
          </p>
        ) : (
          <p className="text-sm text-slate-400">Address not available.</p>
        )}

        <p className="text-xs text-slate-400 mt-1">
          {latitude.toFixed(6)}, {longitude.toFixed(6)}
        </p>
      </div>
    </div>
  )
}

// ─── Static map image ─────────────────────────────────────────────────────────

interface StaticMapImageProps {
  latitude: number
  longitude: number
  zoom: number
  apiKey: string
  fallbackUrl: string
}

function StaticMapImage({ latitude, longitude, zoom, apiKey, fallbackUrl }: StaticMapImageProps) {
  const staticMapUrl =
    `https://maps.googleapis.com/maps/api/staticmap` +
    `?center=${latitude},${longitude}` +
    `&zoom=${zoom}` +
    `&size=600x300` +
    `&markers=color:red%7C${latitude},${longitude}` +
    `&key=${apiKey}`

  function handleError(e: React.SyntheticEvent<HTMLImageElement>) {
    // On load error, replace with fallback
    const img = e.currentTarget
    const parent = img.parentElement
    if (!parent) return
    parent.innerHTML = `
      <div class="flex flex-col items-center justify-center h-40 bg-slate-50 p-4 gap-2">
        <p class="text-sm text-slate-500">Unable to load interactive map.</p>
        <a
          href="${fallbackUrl}"
          target="_blank"
          rel="noopener noreferrer"
          class="text-sm text-blue-600 hover:underline"
        >
          View on Google Maps
        </a>
      </div>
    `
  }

  return (
    <div>
      <img
        src={staticMapUrl}
        alt={`Map showing location at coordinates ${latitude.toFixed(4)}, ${longitude.toFixed(4)}`}
        className="w-full h-48 object-cover"
        onError={handleError}
      />
    </div>
  )
}

// ─── Fallback block ───────────────────────────────────────────────────────────

function MapFallback({ mapsUrl }: { mapsUrl: string }) {
  return (
    <div className="flex flex-col items-center justify-center h-40 bg-slate-50 p-4 gap-2">
      <svg
        className="w-8 h-8 text-slate-300"
        fill="none"
        viewBox="0 0 24 24"
        stroke="currentColor"
        aria-hidden="true"
      >
        <path
          strokeLinecap="round"
          strokeLinejoin="round"
          strokeWidth={1.5}
          d="M9 20l-5.447-2.724A1 1 0 013 16.382V5.618a1 1 0 011.447-.894L9 7m0 13l6-3m-6 3V7m6 10l4.553 2.276A1 1 0 0021 18.382V7.618a1 1 0 00-.553-.894L15 4m0 13V4m0 0L9 7"
        />
      </svg>
      <p className="text-sm text-slate-500">Unable to load interactive map.</p>
      <a
        href={mapsUrl}
        target="_blank"
        rel="noopener noreferrer"
        className="text-sm text-blue-600 hover:underline focus-visible:ring-2 focus-visible:ring-blue-500 rounded"
      >
        View on Google Maps
      </a>
    </div>
  )
}

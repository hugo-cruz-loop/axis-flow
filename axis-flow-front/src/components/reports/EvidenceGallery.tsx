import { useCallback, useEffect, useRef, useState } from 'react'
import type { Evidencia } from '@/api/reports-service'

interface EvidenceGalleryProps {
  evidencias: Evidencia[]
  isLoading: boolean
  onInspectLocation: (lat: number, lng: number) => void
}

// ─── Lightbox ─────────────────────────────────────────────────────────────────

interface LightboxProps {
  images: string[]
  initialIndex: number
  altPrefix: string
  onClose: () => void
  triggerRef: React.RefObject<HTMLButtonElement | null>
}

function Lightbox({ images, initialIndex, altPrefix, onClose, triggerRef }: LightboxProps) {
  const [current, setCurrent] = useState(initialIndex)
  const dialogRef = useRef<HTMLDivElement>(null)
  const closeButtonRef = useRef<HTMLButtonElement>(null)

  // Focus trap
  useEffect(() => {
    closeButtonRef.current?.focus()

    function handleKeyDown(e: KeyboardEvent) {
      if (e.key === 'Escape') {
        onClose()
        return
      }
      if (e.key === 'ArrowRight') {
        setCurrent((c) => (c + 1) % images.length)
        return
      }
      if (e.key === 'ArrowLeft') {
        setCurrent((c) => (c - 1 + images.length) % images.length)
        return
      }
      if (e.key === 'Tab') {
        const focusable = dialogRef.current?.querySelectorAll<HTMLElement>(
          'button, [href], input, [tabindex]:not([tabindex="-1"])',
        )
        if (!focusable || focusable.length === 0) return
        const first = focusable[0]
        const last = focusable[focusable.length - 1]
        if (e.shiftKey) {
          if (document.activeElement === first) {
            e.preventDefault()
            last.focus()
          }
        } else {
          if (document.activeElement === last) {
            e.preventDefault()
            first.focus()
          }
        }
      }
    }

    document.addEventListener('keydown', handleKeyDown)
    return () => {
      document.removeEventListener('keydown', handleKeyDown)
      // Return focus to trigger on close
      triggerRef.current?.focus()
    }
  }, [images.length, onClose, triggerRef])

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/80"
      onClick={onClose}
      aria-modal="true"
      role="dialog"
      aria-label="Evidence photo lightbox"
      ref={dialogRef}
    >
      <div
        className="relative max-w-3xl w-full mx-4"
        onClick={(e) => e.stopPropagation()}
      >
        <button
          ref={closeButtonRef}
          className="absolute -top-10 right-0 text-white text-2xl focus-visible:ring-2 focus-visible:ring-white rounded"
          onClick={onClose}
          aria-label="Close lightbox"
        >
          ✕
        </button>

        <img
          src={images[current]}
          alt={`${altPrefix} — photo ${current + 1} of ${images.length}`}
          className="w-full rounded-lg max-h-[80vh] object-contain"
        />

        {images.length > 1 && (
          <div className="flex justify-between mt-4">
            <button
              className="text-white bg-white/20 hover:bg-white/30 px-4 py-2 rounded focus-visible:ring-2 focus-visible:ring-white"
              onClick={() => setCurrent((c) => (c - 1 + images.length) % images.length)}
              aria-label="Previous photo"
            >
              ← Prev
            </button>
            <span className="text-white self-center">
              {current + 1} / {images.length}
            </span>
            <button
              className="text-white bg-white/20 hover:bg-white/30 px-4 py-2 rounded focus-visible:ring-2 focus-visible:ring-white"
              onClick={() => setCurrent((c) => (c + 1) % images.length)}
              aria-label="Next photo"
            >
              Next →
            </button>
          </div>
        )}
      </div>
    </div>
  )
}

// ─── Skeleton card ────────────────────────────────────────────────────────────

function SkeletonCard() {
  return (
    <div className="rounded-lg overflow-hidden border border-slate-200 animate-pulse" aria-hidden="true">
      <div className="bg-slate-200 h-48 w-full" />
      <div className="p-3 space-y-2">
        <div className="bg-slate-200 h-4 w-3/4 rounded" />
        <div className="bg-slate-200 h-3 w-1/2 rounded" />
      </div>
    </div>
  )
}

// ─── Main component ───────────────────────────────────────────────────────────

export function EvidenceGallery({ evidencias, isLoading, onInspectLocation }: EvidenceGalleryProps) {
  const [lightbox, setLightbox] = useState<{
    images: string[]
    initialIndex: number
    altPrefix: string
  } | null>(null)
  const triggerRef = useRef<HTMLButtonElement | null>(null)

  const openLightbox = useCallback(
    (ev: Evidencia, imgIndex: number, buttonEl: HTMLButtonElement) => {
      triggerRef.current = buttonEl
      setLightbox({
        images: ev.evidencias,
        initialIndex: imgIndex,
        altPrefix: `${ev.actividad_descripcion} — ${ev.empleado_nombre}`,
      })
    },
    [],
  )

  if (isLoading) {
    return (
      <div
        className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4"
        aria-label="Loading evidence photos"
      >
        {Array.from({ length: 6 }).map((_, i) => (
          <SkeletonCard key={i} />
        ))}
      </div>
    )
  }

  if (evidencias.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center py-16 text-slate-500">
        <svg
          className="w-16 h-16 mb-4 text-slate-300"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
          aria-hidden="true"
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={1.5}
            d="M4 16l4.586-4.586a2 2 0 012.828 0L16 16m-2-2l1.586-1.586a2 2 0 012.828 0L20 14m-6-6h.01M6 20h12a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v12a2 2 0 002 2z"
          />
        </svg>
        <p className="text-lg font-medium">No evidence photos found for this range.</p>
      </div>
    )
  }

  return (
    <>
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
        {evidencias.map((ev) =>
          ev.evidencias.map((imgUrl, imgIndex) => (
            <div
              key={`${ev.asignacion_id}-${imgIndex}`}
              className="rounded-lg overflow-hidden border border-slate-200 hover:shadow-md transition-shadow"
            >
              <button
                className="block w-full focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-inset"
                onClick={(e) => openLightbox(ev, imgIndex, e.currentTarget)}
                aria-label={`View photo ${imgIndex + 1} for ${ev.actividad_descripcion} — ${ev.empleado_nombre}`}
              >
                <img
                  src={imgUrl}
                  alt={`Evidence photo for ${ev.actividad_descripcion} by ${ev.empleado_nombre} on ${ev.fecha_ejecucion}`}
                  className="w-full h-48 object-cover"
                  loading="lazy"
                />
              </button>
              <div className="p-3">
                <p className="text-sm font-medium text-slate-800 truncate">
                  {ev.actividad_descripcion}
                </p>
                <p className="text-xs text-slate-500 truncate">{ev.empleado_nombre}</p>
                <p className="text-xs text-slate-400">{ev.fecha_ejecucion}</p>
                <button
                  className="mt-2 text-xs text-blue-600 hover:underline focus-visible:ring-2 focus-visible:ring-blue-500 rounded"
                  onClick={() => onInspectLocation(ev.latitud, ev.longitud)}
                  aria-label={`Inspect location for ${ev.actividad_descripcion}`}
                >
                  View location
                </button>
              </div>
            </div>
          )),
        )}
      </div>

      {lightbox && (
        <Lightbox
          images={lightbox.images}
          initialIndex={lightbox.initialIndex}
          altPrefix={lightbox.altPrefix}
          onClose={() => setLightbox(null)}
          triggerRef={triggerRef}
        />
      )}
    </>
  )
}

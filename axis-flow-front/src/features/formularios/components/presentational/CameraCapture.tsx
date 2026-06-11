import { useId, useRef, useState } from 'react'

interface CameraCaptureProps {
  onCapture: (file: File) => void
  maxSizeBytes?: number
  disabled?: boolean
  ariaLabel: string
}

const DEFAULT_MAX_SIZE = 5 * 1024 * 1024 // 5 MB
const ALLOWED_IMAGE_TYPES = ['image/jpeg', 'image/png', 'image/webp', 'image/heic', 'image/heif']

/**
 * Native camera/file capture.
 *
 * - Uses `<input type="file" accept="image/*" capture="environment">` as the
 *   primary path. Mobile browsers route to the rear camera; desktop opens
 *   the file picker. This is the WCAG-friendly approach (no WebRTC streams
 *   that can crash webviews on low-end devices).
 * - Validates file size + MIME type at the boundary.
 * - Renders a thumbnail preview via `URL.createObjectURL` so the user
 *   sees what was captured and can re-capture.
 */
export function CameraCapture({
  onCapture,
  maxSizeBytes = DEFAULT_MAX_SIZE,
  disabled = false,
  ariaLabel,
}: CameraCaptureProps) {
  const inputRef = useRef<HTMLInputElement | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [previewUrl, setPreviewUrl] = useState<string | null>(null)
  const inputId = useId()

  function handleFile(file: File | null | undefined) {
    setError(null)
    if (!file) return
    if (!ALLOWED_IMAGE_TYPES.includes(file.type)) {
      setError(
        `Unsupported image type. Allowed: JPG, PNG, WEBP, HEIC. Got: ${file.type || 'unknown'}.`,
      )
      return
    }
    if (file.size > maxSizeBytes) {
      const mb = (maxSizeBytes / (1024 * 1024)).toFixed(1)
      setError(`Image is too large. Max size is ${mb} MB.`)
      return
    }
    // Create a new preview URL (revoke any previous one)
    if (previewUrl) {
      try {
        URL.revokeObjectURL(previewUrl)
      } catch {
        // ignore
      }
    }
    const url = URL.createObjectURL(file)
    setPreviewUrl(url)
    onCapture(file)
  }

  function handleChange(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0]
    handleFile(file)
    // Reset the input so the same file can be re-selected after a clear.
    if (inputRef.current) inputRef.current.value = ''
  }

  return (
    <div className="space-y-3">
      <div className="flex items-center gap-3">
        <label
          htmlFor={inputId}
          className={`inline-flex items-center gap-2 rounded-md border border-slate-300 bg-white px-3 py-1.5 text-xs font-semibold ${
            disabled
              ? 'text-slate-400 cursor-not-allowed'
              : 'text-slate-700 hover:bg-slate-50 cursor-pointer'
          }`}
        >
          <span aria-hidden="true">📷</span>
          {previewUrl ? 'Re-capture' : 'Capture photo'}
        </label>
        <input
          ref={inputRef}
          id={inputId}
          type="file"
          accept="image/*"
          capture="environment"
          disabled={disabled}
          onChange={handleChange}
          aria-label={ariaLabel}
          className="sr-only"
        />
        {previewUrl && (
          <img
            src={previewUrl}
            alt="Captured photo preview"
            className="h-16 w-16 rounded-md border border-slate-200 object-cover"
          />
        )}
      </div>
      {error && (
        <p className="text-xs text-rose-600 font-semibold" role="alert">
          {error}
        </p>
      )}
    </div>
  )
}

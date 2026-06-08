import { useRef, useState } from 'react'
import { Upload } from 'lucide-react'
import { cn } from '@/lib/utils'

interface LogoUploaderProps {
  currentLogoUrl?: string
  onFileSelect: (file: File) => void
  disabled?: boolean
}

const ACCEPTED_TYPES = ['image/png', 'image/jpg', 'image/jpeg']
const MAX_BYTES = 2 * 1024 * 1024 // 2 MB

export function LogoUploader({
  currentLogoUrl,
  onFileSelect,
  disabled = false,
}: LogoUploaderProps) {
  const inputRef = useRef<HTMLInputElement>(null)
  const [preview, setPreview] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [dragging, setDragging] = useState(false)

  const validate = (file: File): string | null => {
    if (!ACCEPTED_TYPES.includes(file.type)) {
      return 'Invalid file type. Only PNG and JPG files are accepted.'
    }
    if (file.size > MAX_BYTES) {
      return 'File too large. Maximum allowed size is 2 MB.'
    }
    return null
  }

  const handleFile = (file: File) => {
    const validationError = validate(file)
    if (validationError) {
      setError(validationError)
      setPreview(null)
      return
    }
    setError(null)
    const reader = new FileReader()
    reader.onload = (e) => setPreview(e.target?.result as string)
    reader.readAsDataURL(file)
    onFileSelect(file)
  }

  const handleInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (file) handleFile(file)
  }

  const handleDrop = (e: React.DragEvent<HTMLDivElement>) => {
    e.preventDefault()
    setDragging(false)
    const file = e.dataTransfer.files?.[0]
    if (file) handleFile(file)
  }

  const displaySrc = preview ?? currentLogoUrl

  return (
    <div className="space-y-2">
      <div
        role="button"
        tabIndex={disabled ? -1 : 0}
        aria-label="Upload logo"
        onClick={() => !disabled && inputRef.current?.click()}
        onKeyDown={(e) => {
          if (!disabled && (e.key === 'Enter' || e.key === ' ')) {
            inputRef.current?.click()
          }
        }}
        onDragOver={(e) => {
          e.preventDefault()
          if (!disabled) setDragging(true)
        }}
        onDragLeave={() => setDragging(false)}
        onDrop={handleDrop}
        className={cn(
          'flex cursor-pointer flex-col items-center justify-center rounded-xl border-2 border-dashed p-6 transition-colors',
          dragging ? 'border-indigo-400 bg-indigo-50' : 'border-slate-300 hover:border-indigo-400',
          disabled && 'cursor-not-allowed opacity-50',
        )}
      >
        {displaySrc ? (
          <img
            src={displaySrc}
            alt="Logo preview"
            className="h-24 w-auto rounded object-contain"
          />
        ) : (
          <>
            <Upload className="h-8 w-8 text-slate-400" />
            <p className="mt-2 text-sm text-slate-500">
              Drag & drop or <span className="text-indigo-600 underline">browse</span>
            </p>
            <p className="mt-1 text-xs text-slate-400">PNG, JPG up to 2 MB</p>
          </>
        )}
      </div>

      {error && <p className="text-xs text-red-600">{error}</p>}

      <input
        ref={inputRef}
        type="file"
        accept=".png,.jpg,.jpeg"
        className="sr-only"
        onChange={handleInputChange}
        disabled={disabled}
        aria-hidden="true"
      />
    </div>
  )
}

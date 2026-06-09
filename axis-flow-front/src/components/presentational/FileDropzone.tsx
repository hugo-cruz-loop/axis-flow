import { useRef, useState, useCallback } from 'react'
import { cn } from '@/lib/utils'

interface FileDropzoneProps {
  accept: string
  maxSizeMB: number
  onFile: (file: File) => void
  label?: string
  disabled?: boolean
}

export function FileDropzone({
  accept,
  maxSizeMB,
  onFile,
  label = 'Drop file here or click to browse',
  disabled = false,
}: FileDropzoneProps) {
  const inputRef = useRef<HTMLInputElement>(null)
  const [isDragging, setIsDragging] = useState(false)
  const [selectedFile, setSelectedFile] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)

  const acceptedTypes = accept
    .split(',')
    .map((s) => s.trim())
    .filter(Boolean)

  const validate = useCallback(
    (file: File): string | null => {
      const maxBytes = maxSizeMB * 1024 * 1024
      if (file.size > maxBytes) {
        return `File exceeds ${maxSizeMB} MB limit`
      }
      // MIME check: if accept contains extensions, map them; if MIME types, check directly
      const mimeExtMap: Record<string, string[]> = {
        'application/pdf': ['.pdf'],
        'image/png': ['.png'],
        'image/jpeg': ['.jpg', '.jpeg'],
        'image/gif': ['.gif'],
        'image/webp': ['.webp'],
      }
      const allowed = acceptedTypes.some((a) => {
        if (a.startsWith('.')) {
          return file.name.toLowerCase().endsWith(a.toLowerCase())
        }
        if (file.type === a) return true
        // wildcard like image/*
        if (a.endsWith('/*')) {
          return file.type.startsWith(a.replace('/*', '/'))
        }
        return false
      })
      if (!allowed) {
        return `File type not accepted. Allowed: ${accept}`
      }
      // Additional magic-bytes check via file.type
      const mapped = mimeExtMap[file.type]
      if (mapped) {
        const nameOk = mapped.some((ext) => file.name.toLowerCase().endsWith(ext))
        if (!nameOk) {
          return 'File extension does not match its type'
        }
      }
      return null
    },
    [accept, acceptedTypes, maxSizeMB],
  )

  const handleFile = useCallback(
    (file: File) => {
      const err = validate(file)
      if (err) {
        setError(err)
        setSelectedFile(null)
        return
      }
      setError(null)
      setSelectedFile(file.name)
      onFile(file)
    },
    [validate, onFile],
  )

  const handleDrop = (e: React.DragEvent<HTMLDivElement>) => {
    e.preventDefault()
    setIsDragging(false)
    if (disabled) return
    const file = e.dataTransfer.files[0]
    if (file) handleFile(file)
  }

  const handleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (file) handleFile(file)
    e.target.value = ''
  }

  const handleKeyDown = (e: React.KeyboardEvent<HTMLDivElement>) => {
    if (e.key === 'Enter' || e.key === ' ') {
      e.preventDefault()
      inputRef.current?.click()
    }
  }

  return (
    <div className="w-full">
      <div
        role="button"
        tabIndex={disabled ? -1 : 0}
        aria-label={label}
        aria-disabled={disabled}
        onClick={() => !disabled && inputRef.current?.click()}
        onKeyDown={handleKeyDown}
        onDragOver={(e) => {
          e.preventDefault()
          if (!disabled) setIsDragging(true)
        }}
        onDragLeave={() => setIsDragging(false)}
        onDrop={handleDrop}
        className={cn(
          'flex cursor-pointer flex-col items-center justify-center rounded-lg border-2 border-dashed px-4 py-6 text-center transition-colors',
          disabled
            ? 'cursor-not-allowed border-slate-700 bg-slate-800/30 opacity-50'
            : isDragging
              ? 'border-indigo-400 bg-indigo-500/10'
              : 'border-slate-600 bg-slate-800/20 hover:border-indigo-400 hover:bg-indigo-500/5',
        )}
      >
        <p className="text-sm text-slate-400">{label}</p>
        <p className="mt-1 text-xs text-slate-500">
          {accept} &mdash; max {maxSizeMB} MB
        </p>
        {selectedFile && (
          <p className="mt-2 text-xs font-medium text-indigo-400">{selectedFile}</p>
        )}
      </div>
      {error && <p className="mt-1 text-xs text-red-400">{error}</p>}
      <input
        ref={inputRef}
        type="file"
        accept={accept}
        className="hidden"
        disabled={disabled}
        onChange={handleChange}
      />
    </div>
  )
}

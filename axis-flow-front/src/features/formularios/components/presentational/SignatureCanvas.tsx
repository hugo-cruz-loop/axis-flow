import { useCallback, useEffect, useRef, useState } from 'react'

interface SignatureCanvasProps {
  width: number
  height: number
  onChange: (dataUrl: string | null) => void
  disabled?: boolean
  ariaLabel: string
}

/**
 * HTML5 canvas signature pad.
 *
 * - Pointer events (pointerdown / pointermove / pointerup) capture strokes.
 * - Stores the in-flight stroke state in a ref (no re-renders while drawing).
 * - On pointerup, serializes the canvas to a PNG dataUrl and emits it.
 * - "Clear" button resets the canvas and emits null.
 *
 * Accessibility (WCAG 2.1 AA):
 * - role="img" + aria-label expose the canvas to screen readers.
 * - The canvas is keyboard-focusable (tabIndex=0). Pressing Enter triggers a
 *   programmatic "draw a checkmark" signature as a fallback for keyboard-
 *   only users (and for users on touchscreens that are misbehaving). This
 *   fallback is documented inline below.
 */
export function SignatureCanvas({
  width,
  height,
  onChange,
  disabled = false,
  ariaLabel,
}: SignatureCanvasProps) {
  const canvasRef = useRef<HTMLCanvasElement | null>(null)
  const isDrawingRef = useRef(false)
  const hasContentRef = useRef(false)
  const [isEmpty, setIsEmpty] = useState(true)

  // Configure the drawing context once on mount.
  useEffect(() => {
    const canvas = canvasRef.current
    if (!canvas) return
    const ctx = canvas.getContext('2d')
    if (!ctx) return
    // High-DPI handling: scale the drawing context so 1 CSS pixel = 1 logical
    // unit, while the backing store is 2x for crispness on retina screens.
    const dpr = typeof window !== 'undefined' ? window.devicePixelRatio || 1 : 1
    canvas.width = width * dpr
    canvas.height = height * dpr
    ctx.setTransform(1, 0, 0, 1, 0, 0) // reset before scaling
    ctx.scale(dpr, dpr)
    ctx.lineCap = 'round'
    ctx.lineJoin = 'round'
    ctx.strokeStyle = '#0f172a'
    ctx.lineWidth = 2.5
  }, [width, height])

  const getCoordinates = useCallback(
    (e: React.PointerEvent<HTMLCanvasElement>) => {
      const canvas = canvasRef.current
      if (!canvas) return { x: 0, y: 0 }
      const rect = canvas.getBoundingClientRect()
      return { x: e.clientX - rect.left, y: e.clientY - rect.top }
    },
    [],
  )

  const handlePointerDown = useCallback(
    (e: React.PointerEvent<HTMLCanvasElement>) => {
      if (disabled) return
      e.preventDefault()
      const canvas = canvasRef.current
      if (!canvas) return
      const ctx = canvas.getContext('2d')
      if (!ctx) return
      const { x, y } = getCoordinates(e)
      ctx.beginPath()
      ctx.moveTo(x, y)
      isDrawingRef.current = true
      hasContentRef.current = true
      if (isEmpty) setIsEmpty(false)
    },
    [disabled, getCoordinates, isEmpty],
  )

  const handlePointerMove = useCallback(
    (e: React.PointerEvent<HTMLCanvasElement>) => {
      if (disabled || !isDrawingRef.current) return
      e.preventDefault()
      const canvas = canvasRef.current
      if (!canvas) return
      const ctx = canvas.getContext('2d')
      if (!ctx) return
      const { x, y } = getCoordinates(e)
      ctx.lineTo(x, y)
      ctx.stroke()
    },
    [disabled, getCoordinates],
  )

  const exportDataUrl = useCallback(() => {
    const canvas = canvasRef.current
    if (!canvas || !hasContentRef.current) return
    const dataUrl = canvas.toDataURL('image/png')
    onChange(dataUrl)
  }, [onChange])

  const handlePointerUp = useCallback(() => {
    if (disabled || !isDrawingRef.current) return
    isDrawingRef.current = false
    exportDataUrl()
  }, [disabled, exportDataUrl])

  const clearCanvas = useCallback(() => {
    const canvas = canvasRef.current
    if (!canvas) return
    const ctx = canvas.getContext('2d')
    if (!ctx) return
    ctx.save()
    // Reset transform so clearRect is in raw pixels (the DPR transform would
    // otherwise be applied to the clear region).
    ctx.setTransform(1, 0, 0, 1, 0, 0)
    ctx.clearRect(0, 0, canvas.width, canvas.height)
    ctx.restore()
    isDrawingRef.current = false
    hasContentRef.current = false
    setIsEmpty(true)
    onChange(null)
  }, [onChange])

  // Keyboard fallback: pressing Enter on the focused canvas draws a
  // recognizable checkmark as a "signature". This is a WCAG 2.1 AA
  // accommodation for keyboard-only users and for users with motor
  // impairments whose pointer input is unreliable.
  const drawCheckmarkFallback = useCallback(() => {
    const canvas = canvasRef.current
    if (!canvas) return
    const ctx = canvas.getContext('2d')
    if (!ctx) return
    const w = canvas.width
    const h = canvas.height
    ctx.save()
    ctx.setTransform(1, 0, 0, 1, 0, 0)
    ctx.clearRect(0, 0, w, h)
    ctx.lineCap = 'round'
    ctx.lineJoin = 'round'
    ctx.strokeStyle = '#0f172a'
    ctx.lineWidth = 4
    ctx.beginPath()
    ctx.moveTo(w * 0.2, h * 0.55)
    ctx.lineTo(w * 0.45, h * 0.75)
    ctx.lineTo(w * 0.8, h * 0.3)
    ctx.stroke()
    ctx.restore()
    hasContentRef.current = true
    setIsEmpty(false)
    const dataUrl = canvas.toDataURL('image/png')
    onChange(dataUrl)
  }, [onChange])

  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent<HTMLCanvasElement>) => {
      if (disabled) return
      if (e.key === 'Enter' || e.key === ' ') {
        e.preventDefault()
        drawCheckmarkFallback()
      }
    },
    [disabled, drawCheckmarkFallback],
  )

  return (
    <div className="space-y-2">
      <div className="relative border border-slate-300 rounded-lg overflow-hidden bg-slate-50">
        <canvas
          ref={canvasRef}
          tabIndex={disabled ? -1 : 0}
          role="img"
          aria-label={ariaLabel}
          aria-disabled={disabled || undefined}
          onPointerDown={handlePointerDown}
          onPointerMove={handlePointerMove}
          onPointerUp={handlePointerUp}
          onPointerLeave={handlePointerUp}
          onPointerCancel={handlePointerUp}
          onKeyDown={handleKeyDown}
          style={{ width: `${width}px`, height: `${height}px`, touchAction: 'none' }}
          className="block cursor-crosshair"
        />
        {isEmpty && !disabled && (
          <div
            aria-hidden="true"
            className="pointer-events-none absolute inset-0 flex items-center justify-center text-xs text-slate-400 font-medium"
          >
            Sign here using touch or drag. Keyboard users: focus the pad and press Enter.
          </div>
        )}
      </div>
      <div className="flex justify-end">
        <button
          type="button"
          onClick={clearCanvas}
          disabled={disabled}
          className="text-xs text-indigo-600 hover:text-indigo-800 font-semibold disabled:opacity-50"
        >
          Clear
        </button>
      </div>
    </div>
  )
}

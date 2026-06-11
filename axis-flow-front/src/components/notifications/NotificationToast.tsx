import { useEffect, useRef } from 'react'

export interface NotificationToastProps {
  id: string
  title: string
  message?: string
  durationMs?: number
  onClose: (id: string) => void
}

export function NotificationToast({
  id,
  title,
  message,
  durationMs = 5000,
  onClose,
}: NotificationToastProps) {
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const remainingRef = useRef(durationMs)
  const startedAtRef = useRef<number>(Date.now())

  const startTimer = () => {
    startedAtRef.current = Date.now()
    timerRef.current = setTimeout(() => onClose(id), remainingRef.current)
  }

  const pauseTimer = () => {
    if (timerRef.current !== null) {
      clearTimeout(timerRef.current)
      timerRef.current = null
      remainingRef.current = Math.max(
        0,
        remainingRef.current - (Date.now() - startedAtRef.current),
      )
    }
  }

  useEffect(() => {
    startTimer()
    return () => {
      if (timerRef.current !== null) clearTimeout(timerRef.current)
    }
  }, []) // eslint-disable-line react-hooks/exhaustive-deps

  return (
    <div
      role="alert"
      aria-live="assertive"
      aria-atomic="true"
      className="flex items-start gap-3 rounded-lg bg-white p-4 shadow-lg ring-1 ring-black/5 w-80"
      onMouseEnter={pauseTimer}
      onMouseLeave={startTimer}
    >
      <div className="flex-1 min-w-0">
        <p className="text-sm font-semibold text-gray-900">{title}</p>
        {message && <p className="mt-1 text-sm text-gray-600">{message}</p>}
      </div>
      <button
        type="button"
        aria-label="Cerrar notificación"
        onClick={() => onClose(id)}
        className="flex-shrink-0 rounded p-1 text-gray-400 hover:text-gray-600 focus:outline-none focus:ring-2 focus:ring-indigo-500"
      >
        <svg
          xmlns="http://www.w3.org/2000/svg"
          className="h-4 w-4"
          viewBox="0 0 20 20"
          fill="currentColor"
          aria-hidden="true"
        >
          <path
            fillRule="evenodd"
            d="M4.293 4.293a1 1 0 011.414 0L10 8.586l4.293-4.293a1 1 0 111.414 1.414L11.414 10l4.293 4.293a1 1 0 01-1.414 1.414L10 11.414l-4.293 4.293a1 1 0 01-1.414-1.414L8.586 10 4.293 5.707a1 1 0 010-1.414z"
            clipRule="evenodd"
          />
        </svg>
      </button>
    </div>
  )
}

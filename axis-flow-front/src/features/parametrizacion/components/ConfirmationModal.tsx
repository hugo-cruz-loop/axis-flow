import { useEffect, useRef } from 'react'

interface ConfirmationModalProps {
  open: boolean
  title: string
  message: string
  confirmLabel: string
  cancelLabel: string
  onConfirm: () => void
  onCancel: () => void
  returnFocusRef: React.RefObject<HTMLElement | null>
}

export function ConfirmationModal({
  open,
  title,
  message,
  confirmLabel,
  cancelLabel,
  onConfirm,
  onCancel,
  returnFocusRef,
}: ConfirmationModalProps) {
  const dialogRef = useRef<HTMLDivElement>(null)
  const confirmRef = useRef<HTMLButtonElement>(null)

  useEffect(() => {
    if (!open) return
    confirmRef.current?.focus()
  }, [open])

  if (!open) return null

  function handleCancel() {
    onCancel()
    window.setTimeout(() => returnFocusRef.current?.focus(), 0)
  }

  function handleKeyDown(event: React.KeyboardEvent<HTMLDivElement>) {
    if (event.key === 'Escape') {
      event.preventDefault()
      handleCancel()
      return
    }
    if (event.key !== 'Tab') return

    const focusable = Array.from(
      dialogRef.current?.querySelectorAll<HTMLButtonElement>('button:not([disabled])') ?? [],
    )
    const first = focusable.at(0)
    const last = focusable.at(-1)
    if (!first || !last) return

    if (event.shiftKey && document.activeElement === first) {
      event.preventDefault()
      last.focus()
    } else if (!event.shiftKey && document.activeElement === last) {
      event.preventDefault()
      first.focus()
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/50 px-4">
      <div
        ref={dialogRef}
        role="dialog"
        aria-modal="true"
        aria-labelledby="service-evaluation-confirm-title"
        className="w-full max-w-lg rounded-xl bg-white p-6 shadow-2xl"
        onKeyDown={handleKeyDown}
      >
        <h2 id="service-evaluation-confirm-title" className="text-lg font-semibold text-slate-900">
          {title}
        </h2>
        <p className="mt-3 text-sm leading-6 text-slate-600">{message}</p>
        <div className="mt-6 flex justify-end gap-3">
          <button
            ref={confirmRef}
            type="button"
            onClick={onConfirm}
            className="rounded-md bg-indigo-600 px-4 py-2 text-sm font-semibold text-white hover:bg-indigo-700 focus:outline-none focus-visible:ring-2 focus-visible:ring-indigo-500 focus-visible:ring-offset-2"
          >
            {confirmLabel}
          </button>
          <button
            type="button"
            onClick={handleCancel}
            className="rounded-md border border-slate-300 px-4 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-50 focus:outline-none focus-visible:ring-2 focus-visible:ring-indigo-500 focus-visible:ring-offset-2"
          >
            {cancelLabel}
          </button>
        </div>
      </div>
    </div>
  )
}

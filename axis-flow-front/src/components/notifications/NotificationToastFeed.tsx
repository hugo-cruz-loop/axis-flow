import { useCallback, useState } from 'react'
import { createPortal } from 'react-dom'
import {
  NotificationToast,
  type NotificationToastProps,
} from './NotificationToast'

export type ToastInput = Omit<NotificationToastProps, 'onClose'>

export interface NotificationToastFeedProps {
  toasts: ToastInput[]
  onClose: (id: string) => void
}

export function NotificationToastFeed({ toasts, onClose }: NotificationToastFeedProps) {
  return createPortal(
    <div
      aria-label="Alertas operativas"
      className="fixed bottom-4 right-4 z-50 flex flex-col gap-2 items-end"
    >
      {toasts.map((toast) => (
        <NotificationToast key={toast.id} {...toast} onClose={onClose} />
      ))}
    </div>,
    document.body,
  )
}

// Convenience hook for managing the toast queue with deduplication
export function useToastFeed() {
  const [toasts, setToasts] = useState<ToastInput[]>([])

  const push = useCallback((toast: ToastInput) => {
    setToasts((prev) => {
      if (prev.some((t) => t.id === toast.id)) return prev
      return [...prev, toast]
    })
  }, [])

  const remove = useCallback((id: string) => {
    setToasts((prev) => prev.filter((t) => t.id !== id))
  }, [])

  return { toasts, push, remove }
}

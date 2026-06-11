import { useEffect, useRef, useCallback } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { ChatMessage } from './ChatMessage'
import type { ChatMessageInput } from '@/schemas/notificaciones-schemas'

const messageSchema = z.object({
  message: z.string().min(1).max(1000),
})
type MessageFormValues = z.infer<typeof messageSchema>

export interface SupportChatDrawerProps {
  open: boolean
  onClose: () => void
  title?: string
  description?: string
  messages: ChatMessageInput[]
  currentUserId: string
  onSendMessage: (message: string) => Promise<void> | void
  isSending?: boolean
}

const FOCUSABLE_SELECTORS =
  'a[href], button:not([disabled]), textarea, input, select, [tabindex]:not([tabindex="-1"])'

export function SupportChatDrawer({
  open,
  onClose,
  title = 'Soporte en línea',
  description = 'Chatea con un agente de soporte',
  messages,
  currentUserId,
  onSendMessage,
  isSending = false,
}: SupportChatDrawerProps) {
  const drawerRef = useRef<HTMLDivElement>(null)
  const previousFocusRef = useRef<HTMLElement | null>(null)
  const messagesEndRef = useRef<HTMLDivElement>(null)
  const titleId = 'support-chat-title'
  const descId = 'support-chat-desc'

  const {
    register,
    handleSubmit,
    reset,
    formState: { errors },
  } = useForm<MessageFormValues>({
    resolver: zodResolver(messageSchema),
  })

  // Save and restore focus
  useEffect(() => {
    if (open) {
      previousFocusRef.current = document.activeElement as HTMLElement
      // Focus first focusable element inside drawer
      requestAnimationFrame(() => {
        const first = drawerRef.current?.querySelector<HTMLElement>(FOCUSABLE_SELECTORS)
        first?.focus()
      })
    } else {
      previousFocusRef.current?.focus()
    }
  }, [open])

  // Body scroll lock
  useEffect(() => {
    if (open) {
      document.body.style.overflow = 'hidden'
    }
    return () => {
      document.body.style.overflow = ''
    }
  }, [open])

  // Escape key
  useEffect(() => {
    if (!open) return
    const handler = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }
    document.addEventListener('keydown', handler)
    return () => document.removeEventListener('keydown', handler)
  }, [open, onClose])

  // Focus trap (Tab / Shift+Tab circular)
  const handleKeyDown = useCallback((e: React.KeyboardEvent<HTMLDivElement>) => {
    if (e.key !== 'Tab' || !drawerRef.current) return
    const focusable = Array.from(
      drawerRef.current.querySelectorAll<HTMLElement>(FOCUSABLE_SELECTORS),
    ).filter((el) => !el.closest('[aria-hidden="true"]'))

    if (focusable.length === 0) return

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
  }, [])

  // Auto-scroll to bottom on new messages
  useEffect(() => {
    if (open) {
      messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' })
    }
  }, [messages, open])

  const onSubmit = async (values: MessageFormValues) => {
    await onSendMessage(values.message)
    reset()
  }

  if (!open) return null

  return (
    <>
      {/* Backdrop */}
      <div
        className="fixed inset-0 bg-black/40 z-40"
        aria-hidden="true"
        onClick={onClose}
      />

      {/* Drawer panel */}
      <div
        ref={drawerRef}
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleId}
        aria-describedby={descId}
        onKeyDown={handleKeyDown}
        className="fixed right-0 top-0 bottom-0 z-50 flex flex-col w-full max-w-md bg-gray-50 shadow-2xl"
      >
        {/* Header */}
        <div className="flex items-center justify-between px-4 py-3 bg-white border-b border-gray-200">
          <div>
            <h2 id={titleId} className="text-base font-semibold text-gray-900">
              {title}
            </h2>
            <p id={descId} className="text-xs text-gray-500">
              {description}
            </p>
          </div>
          <button
            type="button"
            aria-label="Cerrar chat"
            onClick={onClose}
            className="rounded p-1.5 text-gray-400 hover:text-gray-600 hover:bg-gray-100 focus:outline-none focus:ring-2 focus:ring-indigo-500"
          >
            <svg
              xmlns="http://www.w3.org/2000/svg"
              className="h-5 w-5"
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

        {/* Messages */}
        <div
          className="flex-1 overflow-y-auto px-4 py-4 flex flex-col gap-4"
          aria-label="Mensajes del chat"
        >
          {messages.map((msg, idx) => (
            <ChatMessage
              key={msg.id ?? `msg-${idx}`}
              message={msg}
              currentUserId={currentUserId}
            />
          ))}
          <div ref={messagesEndRef} aria-hidden="true" />
        </div>

        {/* Message input */}
        <form
          onSubmit={handleSubmit(onSubmit)}
          className="border-t border-gray-200 bg-white px-4 py-3 flex gap-2 items-end"
          aria-label="Formulario de mensaje"
        >
          <div className="flex-1">
            <label htmlFor="chat-message-input" className="sr-only">
              Escribe un mensaje
            </label>
            <textarea
              id="chat-message-input"
              rows={2}
              placeholder="Escribe un mensaje..."
              aria-invalid={errors.message ? 'true' : undefined}
              aria-describedby={errors.message ? 'chat-message-error' : undefined}
              className="w-full resize-none rounded-lg border border-gray-300 px-3 py-2 text-sm text-gray-900 placeholder-gray-400 focus:outline-none focus:ring-2 focus:ring-indigo-500 disabled:opacity-50"
              disabled={isSending}
              onKeyDown={(e) => {
                if (e.key === 'Enter' && !e.shiftKey) {
                  e.preventDefault()
                  void handleSubmit(onSubmit)()
                }
              }}
              {...register('message')}
            />
            {errors.message && (
              <p id="chat-message-error" role="alert" className="mt-1 text-xs text-red-600">
                {errors.message.message}
              </p>
            )}
          </div>
          <button
            type="submit"
            disabled={isSending}
            aria-label="Enviar mensaje"
            className="flex-shrink-0 rounded-lg bg-indigo-600 px-4 py-2 text-sm font-semibold text-white hover:bg-indigo-700 focus:outline-none focus:ring-2 focus:ring-indigo-500 disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {isSending ? 'Enviando…' : 'Enviar'}
          </button>
        </form>
      </div>
    </>
  )
}

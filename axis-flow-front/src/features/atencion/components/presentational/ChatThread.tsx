import { useEffect, useRef, useState } from 'react'
import type { RespuestaQueja, RespuestaServicio } from '../../types'

type AnyRespuesta = RespuestaQueja | RespuestaServicio

interface ChatThreadProps {
  mensajes: AnyRespuesta[]
  currentUserRole: number
  onSend: (msg: string) => void
}

export function ChatThread({ mensajes, currentUserRole, onSend }: ChatThreadProps) {
  const bottomRef = useRef<HTMLDivElement>(null)
  const [input, setInput] = useState('')

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [mensajes])

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    const trimmed = input.trim()
    if (!trimmed) return
    onSend(trimmed)
    setInput('')
  }

  function handleKeyDown(e: React.KeyboardEvent<HTMLTextAreaElement>) {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      const trimmed = input.trim()
      if (!trimmed) return
      onSend(trimmed)
      setInput('')
    }
  }

  return (
    <div className="flex flex-col h-full">
      <ul
        aria-live="polite"
        aria-label="Chat messages"
        className="flex-1 overflow-y-auto space-y-3 p-4"
      >
        {mensajes.map((m) => {
          const isMine = m.rol_respuesta === currentUserRole
          return (
            <li
              key={m.id}
              className={`flex ${isMine ? 'justify-end' : 'justify-start'}`}
            >
              <div
                className={`max-w-xs rounded-lg px-3 py-2 text-sm ${
                  isMine
                    ? 'bg-blue-600 text-white'
                    : 'bg-gray-100 text-gray-900'
                }`}
              >
                <p>{m.mensaje}</p>
                <time
                  className={`mt-1 block text-xs ${isMine ? 'text-blue-200' : 'text-gray-400'}`}
                  dateTime={m.created_at}
                >
                  {new Date(m.created_at).toLocaleTimeString()}
                </time>
              </div>
            </li>
          )
        })}
        <div ref={bottomRef} />
      </ul>

      <form onSubmit={handleSubmit} className="border-t border-gray-200 p-3">
        <div className="flex items-end gap-2">
          <textarea
            value={input}
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={handleKeyDown}
            rows={2}
            placeholder="Type a message… (Enter to send)"
            aria-label="Message input"
            className="flex-1 resize-none rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
          />
          <button
            type="submit"
            disabled={!input.trim()}
            className="rounded-md bg-blue-600 px-3 py-2 text-sm font-medium text-white hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-50"
          >
            Send
          </button>
        </div>
      </form>
    </div>
  )
}

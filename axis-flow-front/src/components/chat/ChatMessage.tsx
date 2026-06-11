import type { ChatMessageInput } from '@/schemas/notificaciones-schemas'

export interface ChatMessageProps {
  message: ChatMessageInput
  currentUserId: string
}

export function ChatMessage({ message, currentUserId }: ChatMessageProps) {
  const isOwn = message.sender_id === currentUserId

  const formattedTime = new Intl.DateTimeFormat('es-MX', {
    hour: '2-digit',
    minute: '2-digit',
  }).format(new Date(message.timestamp))

  return (
    <div className={`flex flex-col gap-1 ${isOwn ? 'items-end' : 'items-start'}`}>
      {!isOwn && (
        <span className="text-xs text-gray-500 px-1">
          {message.sender_name} · {message.sender_role}
        </span>
      )}
      <div
        className={`max-w-[75%] rounded-2xl px-4 py-2 text-sm break-words ${
          isOwn
            ? 'bg-indigo-600 text-white rounded-br-sm'
            : 'bg-white text-gray-900 shadow-sm ring-1 ring-black/5 rounded-bl-sm'
        }`}
      >
        {message.message}
      </div>
      <time
        dateTime={message.timestamp}
        className="text-xs text-gray-400 px-1"
        aria-label={`Enviado a las ${formattedTime}`}
      >
        {formattedTime}
      </time>
    </div>
  )
}

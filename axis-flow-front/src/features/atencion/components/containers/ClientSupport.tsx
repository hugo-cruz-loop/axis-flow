import { useState } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useAuthStore } from '@/store/authStore'
import { TicketCard } from '../presentational/TicketCard'
import { ChatThread } from '../presentational/ChatThread'
import { TicketSchema } from '../../schemas/validation'
import type { TicketInput } from '../../schemas/validation'
import {
  useTicketsByCliente,
  useCreateTicket,
  useMensajesTicket,
  useCreateMensajeTicket,
} from '../../api/queries'
import type { TicketServicio } from '../../types'

export function ClientSupport() {
  const user = useAuthStore((s) => s.user)
  const clienteId = (user as Record<string, string> | null)?.id ?? ''
  const [selectedTicket, setSelectedTicket] = useState<TicketServicio | null>(null)
  const [showModal, setShowModal] = useState(false)

  const { data, isLoading } = useTicketsByCliente(clienteId)
  const createTicket = useCreateTicket()

  const { data: mensajes = [] } = useMensajesTicket(selectedTicket?.id ?? '')
  const createMensaje = useCreateMensajeTicket(selectedTicket?.id ?? '')

  const {
    register,
    handleSubmit,
    reset,
    formState: { errors },
  } = useForm<TicketInput>({ resolver: zodResolver(TicketSchema) })

  function handleCreateTicket(data: TicketInput) {
    createTicket.mutate(data, {
      onSuccess: () => {
        reset()
        setShowModal(false)
      },
    })
  }

  if (isLoading) {
    return <p className="p-4 text-sm text-gray-500">Loading tickets…</p>
  }

  return (
    <div className="flex h-full gap-4 p-4">
      {/* Left pane: ticket list */}
      <div className="w-80 flex-shrink-0 space-y-2">
        <div className="mb-3 flex items-center justify-between">
          <h2 className="text-base font-semibold text-gray-900">Support Tickets</h2>
          <button
            type="button"
            onClick={() => setShowModal(true)}
            className="rounded-md bg-blue-600 px-3 py-1.5 text-xs font-medium text-white hover:bg-blue-700"
          >
            New Ticket
          </button>
        </div>

        {data?.data.map((ticket) => (
          <TicketCard
            key={ticket.id}
            ticket={ticket}
            onClick={() => setSelectedTicket(ticket)}
          />
        ))}
      </div>

      {/* Right pane: chat thread */}
      {selectedTicket && (
        <div className="flex-1 rounded-lg border border-gray-200 bg-white">
          <div className="border-b border-gray-200 px-4 py-3">
            <h3 className="text-sm font-semibold text-gray-900">{selectedTicket.asunto}</h3>
          </div>
          <div className="h-[calc(100%-57px)]">
            <ChatThread
              mensajes={mensajes}
              currentUserRole={1}
              onSend={(msg) => createMensaje.mutate({ mensaje: msg })}
            />
          </div>
        </div>
      )}

      {/* New ticket modal */}
      {showModal && (
        <div
          role="dialog"
          aria-modal="true"
          aria-labelledby="ticket-modal-title"
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"
        >
          <div className="w-full max-w-md rounded-lg bg-white p-6">
            <h2 id="ticket-modal-title" className="mb-4 text-base font-semibold text-gray-900">
              New Ticket
            </h2>
            <form onSubmit={handleSubmit(handleCreateTicket)} className="space-y-3" noValidate>
              <div>
                <label className="block text-sm font-medium text-gray-700">Location ID</label>
                <input
                  type="text"
                  {...register('localidad_id')}
                  className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm"
                />
                {errors.localidad_id && (
                  <p className="mt-1 text-xs text-red-600">{errors.localidad_id.message}</p>
                )}
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700">Subject</label>
                <input
                  type="text"
                  {...register('asunto')}
                  className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm"
                />
                {errors.asunto && (
                  <p className="mt-1 text-xs text-red-600">{errors.asunto.message}</p>
                )}
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700">Description</label>
                <textarea
                  rows={4}
                  {...register('descripcion')}
                  className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm"
                />
                {errors.descripcion && (
                  <p className="mt-1 text-xs text-red-600">{errors.descripcion.message}</p>
                )}
              </div>
              <div className="flex justify-end gap-2">
                <button
                  type="button"
                  onClick={() => { setShowModal(false); reset() }}
                  className="rounded-md border border-gray-300 px-3 py-1.5 text-sm"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={createTicket.isPending}
                  className="rounded-md bg-blue-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-50"
                >
                  Submit
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  )
}

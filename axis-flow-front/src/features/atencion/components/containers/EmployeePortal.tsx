import { useState } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useAuthStore } from '@/store/authStore'
import { ComplaintCard } from '../presentational/ComplaintCard'
import { ChatThread } from '../presentational/ChatThread'
import { QuejaSchema } from '../../schemas/validation'
import type { QuejaInput } from '../../schemas/validation'
import {
  useQuejasByEmpresa,
  useCreateQueja,
  useMensajesQueja,
  useCreateMensajeQueja,
} from '../../api/queries'
import type { SolicitudQueja } from '../../types'

export function EmployeePortal() {
  const user = useAuthStore((s) => s.user)
  // empresa_id comes from JWT claims via Zustand user state
  const empresaId = (user as Record<string, string> | null)?.empresa_id ?? ''
  const [selectedQueja, setSelectedQueja] = useState<SolicitudQueja | null>(null)
  const [showModal, setShowModal] = useState(false)

  const { data, isLoading } = useQuejasByEmpresa(empresaId)
  const createQueja = useCreateQueja()

  const { data: mensajes = [] } = useMensajesQueja(selectedQueja?.id ?? '')
  const createMensaje = useCreateMensajeQueja(selectedQueja?.id ?? '')

  const {
    register,
    handleSubmit,
    reset,
    formState: { errors },
  } = useForm<QuejaInput>({ resolver: zodResolver(QuejaSchema) })

  function handleCreateQueja(data: QuejaInput) {
    createQueja.mutate(data, {
      onSuccess: () => {
        reset()
        setShowModal(false)
      },
    })
  }

  if (isLoading) {
    return <p className="p-4 text-sm text-gray-500">Loading complaints…</p>
  }

  return (
    <div className="flex h-full gap-4 p-4">
      {/* Left pane: complaint list */}
      <div className="w-80 flex-shrink-0 space-y-2">
        <div className="mb-3 flex items-center justify-between">
          <h2 className="text-base font-semibold text-gray-900">My Complaints</h2>
          <button
            type="button"
            onClick={() => setShowModal(true)}
            className="rounded-md bg-blue-600 px-3 py-1.5 text-xs font-medium text-white hover:bg-blue-700"
          >
            New Complaint
          </button>
        </div>

        {data?.data.map((queja) => (
          <ComplaintCard
            key={queja.id}
            queja={queja}
            onClick={() => setSelectedQueja(queja)}
          />
        ))}
      </div>

      {/* Right pane: chat thread */}
      {selectedQueja && (
        <div className="flex-1 rounded-lg border border-gray-200 bg-white">
          <div className="border-b border-gray-200 px-4 py-3">
            <h3 className="text-sm font-semibold text-gray-900">{selectedQueja.titulo}</h3>
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

      {/* New complaint modal */}
      {showModal && (
        <div
          role="dialog"
          aria-modal="true"
          aria-labelledby="modal-title"
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"
        >
          <div className="w-full max-w-md rounded-lg bg-white p-6">
            <h2 id="modal-title" className="mb-4 text-base font-semibold text-gray-900">
              New Complaint
            </h2>
            <form onSubmit={handleSubmit(handleCreateQueja)} className="space-y-3" noValidate>
              <div>
                <label className="block text-sm font-medium text-gray-700">
                  Complaint Type ID
                </label>
                <input
                  type="text"
                  {...register('tipo_queja_id')}
                  className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm"
                />
                {errors.tipo_queja_id && (
                  <p className="mt-1 text-xs text-red-600">{errors.tipo_queja_id.message}</p>
                )}
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700">Title</label>
                <input
                  type="text"
                  {...register('titulo')}
                  className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm"
                />
                {errors.titulo && (
                  <p className="mt-1 text-xs text-red-600">{errors.titulo.message}</p>
                )}
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700">Description</label>
                <textarea
                  rows={3}
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
                  disabled={createQueja.isPending}
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

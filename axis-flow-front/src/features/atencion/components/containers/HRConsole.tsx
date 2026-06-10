import { useState } from 'react'
import { useAuthStore } from '@/store/authStore'
import { ComplaintCard } from '../presentational/ComplaintCard'
import { ChatThread } from '../presentational/ChatThread'
import { useQuejasByEmpresa, useMensajesQueja, useCreateMensajeQueja } from '../../api/queries'
import type { SolicitudQueja } from '../../types'

const ESTATUS_OPTIONS = [
  { label: 'All', value: undefined },
  { label: 'Pendiente', value: 1 },
  { label: 'En Proceso', value: 2 },
  { label: 'Finalizado', value: 3 },
] as const

export function HRConsole() {
  const user = useAuthStore((s) => s.user)
  const empresaId = (user as Record<string, string> | null)?.empresa_id ?? ''
  const [estatus, setEstatus] = useState<number | undefined>(undefined)
  const [selectedQueja, setSelectedQueja] = useState<SolicitudQueja | null>(null)

  const { data, isLoading } = useQuejasByEmpresa(empresaId, estatus)
  const { data: mensajes = [] } = useMensajesQueja(selectedQueja?.id ?? '')
  const createMensaje = useCreateMensajeQueja(selectedQueja?.id ?? '')

  return (
    <div className="flex h-full gap-4 p-4">
      {/* Left pane: filtered list */}
      <div className="w-80 flex-shrink-0">
        <div className="mb-3">
          <h2 className="mb-2 text-base font-semibold text-gray-900">Complaints</h2>
          <div className="flex flex-wrap gap-1">
            {ESTATUS_OPTIONS.map((opt) => (
              <button
                key={String(opt.value)}
                type="button"
                onClick={() => setEstatus(opt.value)}
                className={`rounded-full px-2 py-0.5 text-xs font-medium ${
                  estatus === opt.value
                    ? 'bg-blue-600 text-white'
                    : 'bg-gray-100 text-gray-700 hover:bg-gray-200'
                }`}
              >
                {opt.label}
              </button>
            ))}
          </div>
        </div>

        {isLoading ? (
          <p className="text-sm text-gray-500">Loading…</p>
        ) : (
          <div className="space-y-2">
            {data?.data.map((queja) => (
              <ComplaintCard
                key={queja.id}
                queja={queja}
                onClick={() => setSelectedQueja(queja)}
              />
            ))}
          </div>
        )}
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
              currentUserRole={2}
              onSend={(msg) => createMensaje.mutate({ mensaje: msg })}
            />
          </div>
        </div>
      )}
    </div>
  )
}

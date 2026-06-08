import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Plus, Search, Eye, Trash2 } from 'lucide-react'
import { DashboardLayout } from '@/layouts/DashboardLayout'
import { ClientWizard } from '@/components/containers/ClientWizard'
import { useClienteList, useClienteStats } from '@/hooks/api/useClientes'
import { useAuthStore } from '@/store/authStore'
import { ClienteStatus } from '@/types/clientes'
import type { Cliente } from '@/types/clientes'
import { cn } from '@/lib/utils'

function EstatusBadge({ estatus }: { estatus: number }) {
  const isActivo = estatus === ClienteStatus.ACTIVO
  return (
    <span
      className={cn(
        'inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium',
        isActivo ? 'bg-green-100 text-green-700' : 'bg-amber-100 text-amber-700',
      )}
    >
      {isActivo ? 'Activo' : 'Incompleto'}
    </span>
  )
}

export function ClientesPage() {
  const navigate = useNavigate()
  const user = useAuthStore((s) => s.user)
  // Use empresa_id from user context — fallback to 1 for dev
  const empresaId = (user as unknown as { empresa_id?: number })?.empresa_id ?? 1

  const [search, setSearch] = useState('')
  const [wizardOpen, setWizardOpen] = useState(false)

  const { data: listData, isLoading } = useClienteList(empresaId)
  const { data: stats } = useClienteStats(empresaId)

  const clientes: Cliente[] = listData?.data ?? []
  const filtered = clientes.filter((c) =>
    c.nombre_comercial.toLowerCase().includes(search.toLowerCase()),
  )

  return (
    <DashboardLayout>
      <div className="space-y-6">
        {/* Header */}
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-2xl font-bold text-gray-900">Clientes</h1>
            {stats && (
              <p className="mt-1 text-sm text-gray-500">
                {stats.active} activos · {stats.inactive} incompletos
              </p>
            )}
          </div>
          <button
            onClick={() => setWizardOpen(true)}
            className="flex items-center gap-2 rounded-lg bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-700"
          >
            <Plus className="h-4 w-4" />
            Nuevo Cliente
          </button>
        </div>

        {/* Search */}
        <div className="relative max-w-sm">
          <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-gray-400" />
          <input
            type="text"
            placeholder="Buscar por nombre comercial..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="w-full rounded-lg border border-gray-300 pl-9 pr-3 py-2 text-sm focus:border-indigo-500 focus:outline-none"
          />
        </div>

        {/* Table */}
        <div className="rounded-xl border border-gray-200 bg-white shadow-sm">
          {isLoading ? (
            <div className="flex items-center justify-center py-16 text-sm text-gray-500">
              Cargando clientes...
            </div>
          ) : filtered.length === 0 ? (
            <div className="flex items-center justify-center py-16 text-sm text-gray-500">
              {search ? 'Sin resultados para la búsqueda' : 'No hay clientes registrados'}
            </div>
          ) : (
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-gray-200 bg-gray-50 text-left text-xs font-semibold uppercase tracking-wider text-gray-500">
                  <th className="px-4 py-3">Nombre Comercial</th>
                  <th className="px-4 py-3">Razón Social</th>
                  <th className="px-4 py-3">Estatus</th>
                  <th className="px-4 py-3">Inicio de Contrato</th>
                  <th className="px-4 py-3 text-right">Acciones</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-100">
                {filtered.map((c) => (
                  <tr key={c.id} className="hover:bg-gray-50">
                    <td className="px-4 py-3 font-medium text-gray-900">{c.nombre_comercial}</td>
                    <td className="px-4 py-3 text-gray-500">{c.razon_social ?? '—'}</td>
                    <td className="px-4 py-3">
                      <EstatusBadge estatus={c.estatus} />
                    </td>
                    <td className="px-4 py-3 text-gray-500">
                      {c.fecha_inicio_contrato ?? '—'}
                    </td>
                    <td className="px-4 py-3">
                      <div className="flex items-center justify-end gap-2">
                        <button
                          onClick={() => navigate(`/dashboard/clientes/${c.id}`)}
                          className="rounded p-1 text-gray-400 hover:text-indigo-600"
                          title="Ver detalle"
                        >
                          <Eye className="h-4 w-4" />
                        </button>
                        <button
                          className="rounded p-1 text-gray-400 hover:text-red-600"
                          title="Eliminar"
                        >
                          <Trash2 className="h-4 w-4" />
                        </button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      </div>

      {/* Wizard modal */}
      {wizardOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4">
          <div className="w-full max-w-md rounded-xl bg-white p-6 shadow-xl">
            <ClientWizard empresaId={empresaId} onClose={() => setWizardOpen(false)} />
          </div>
        </div>
      )}
    </DashboardLayout>
  )
}

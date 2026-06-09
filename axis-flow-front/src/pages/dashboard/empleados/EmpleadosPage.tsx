import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Plus, Search, Eye, Trash2 } from 'lucide-react'
import { DashboardLayout } from '@/layouts/DashboardLayout'
import { EmployeeWizard } from '@/components/containers/EmployeeWizard'
import { useEmpleados, useEmpleadoKPIs, useSoftDeleteEmpleado } from '@/hooks/api/useEmpleados'
import { useAuthStore } from '@/store/authStore'
import { EmpleadoStatus } from '@/types/empleados'
import type { Empleado } from '@/types/empleados'
import { cn } from '@/lib/utils'

function StatusBadge({ status }: { status: number }) {
  const config: Record<number, { label: string; className: string }> = {
    [EmpleadoStatus.ACTIVO]: {
      label: 'Active',
      className: 'bg-green-100 text-green-700',
    },
    [EmpleadoStatus.INCOMPLETO]: {
      label: 'Incomplete',
      className: 'bg-amber-100 text-amber-700',
    },
    [EmpleadoStatus.BAJA]: {
      label: 'Inactive',
      className: 'bg-red-100 text-red-700',
    },
  }
  const { label, className } = config[status] ?? {
    label: 'Unknown',
    className: 'bg-slate-100 text-slate-600',
  }
  return (
    <span
      className={cn(
        'inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium',
        className,
      )}
    >
      {label}
    </span>
  )
}

function KPICard({ label, value, color }: { label: string; value: number; color: string }) {
  return (
    <div className={cn('rounded-lg border p-4', color)}>
      <p className="text-2xl font-bold">{value}</p>
      <p className="mt-1 text-sm opacity-80">{label}</p>
    </div>
  )
}

export function EmpleadosPage() {
  const navigate = useNavigate()
  const user = useAuthStore((s) => s.user)
  const empresaId = (user as unknown as { empresa_id?: number })?.empresa_id ?? 1

  const [search, setSearch] = useState('')
  const [wizardOpen, setWizardOpen] = useState(false)

  const { data: listData, isLoading } = useEmpleados(empresaId)
  const { data: kpis } = useEmpleadoKPIs(empresaId)
  const { mutate: softDelete } = useSoftDeleteEmpleado()

  const empleados: Empleado[] = listData?.data ?? []
  const filtered = empleados.filter(
    (e) =>
      e.nombre.toLowerCase().includes(search.toLowerCase()) ||
      e.apellido_paterno.toLowerCase().includes(search.toLowerCase()) ||
      e.id_empleado.toLowerCase().includes(search.toLowerCase()),
  )

  const handleDelete = (e: React.MouseEvent, numEmpleado: number) => {
    e.stopPropagation()
    if (!window.confirm('Are you sure you want to deactivate this employee?')) return
    softDelete({ numEmpleado, empresaId })
  }

  return (
    <DashboardLayout>
      <div className="space-y-6">
        {/* Header */}
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-2xl font-bold text-gray-900">Employees</h1>
            <p className="mt-1 text-sm text-gray-500">
              {listData?.total ?? 0} total employees
            </p>
          </div>
          <button
            onClick={() => setWizardOpen(true)}
            className="flex items-center gap-2 rounded-lg bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-700"
          >
            <Plus className="h-4 w-4" />
            New Employee
          </button>
        </div>

        {/* KPI Cards */}
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
          <KPICard
            label="Complete Dossier"
            value={kpis.complete}
            color="border-green-200 bg-green-50 text-green-800"
          />
          <KPICard
            label="Missing Contract"
            value={kpis.lack}
            color="border-amber-200 bg-amber-50 text-amber-800"
          />
          <KPICard
            label="Pending Validation"
            value={kpis.pendiente}
            color="border-blue-200 bg-blue-50 text-blue-800"
          />
        </div>

        {/* Search */}
        <div className="relative">
          <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-gray-400" />
          <input
            type="text"
            placeholder="Search by name or employee ID..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="w-full rounded-lg border border-gray-300 py-2 pl-9 pr-4 text-sm focus:border-indigo-500 focus:outline-none"
          />
        </div>

        {/* Table */}
        {isLoading ? (
          <p className="text-sm text-gray-500">Loading employees...</p>
        ) : filtered.length === 0 ? (
          <p className="text-sm text-gray-500">No employees found.</p>
        ) : (
          <div className="overflow-hidden rounded-xl border border-gray-200 bg-white shadow-sm">
            <table className="w-full text-sm">
              <thead className="bg-gray-50 text-xs uppercase text-gray-500">
                <tr>
                  <th className="px-4 py-3 text-left">Name</th>
                  <th className="px-4 py-3 text-left">Employee ID</th>
                  <th className="px-4 py-3 text-left">Status</th>
                  <th className="px-4 py-3 text-left">Created</th>
                  <th className="px-4 py-3 text-right">Actions</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-100">
                {filtered.map((emp) => (
                  <tr
                    key={emp.num_empleado}
                    className="cursor-pointer hover:bg-gray-50"
                    onClick={() => navigate(`/dashboard/empleados/${emp.num_empleado}`)}
                  >
                    <td className="px-4 py-3 font-medium text-gray-900">
                      {emp.nombre} {emp.apellido_paterno}{' '}
                      {emp.apellido_materno ?? ''}
                    </td>
                    <td className="px-4 py-3 text-gray-600">{emp.id_empleado}</td>
                    <td className="px-4 py-3">
                      <StatusBadge status={emp.status} />
                    </td>
                    <td className="px-4 py-3 text-gray-500">
                      {new Date(emp.created_at).toLocaleDateString()}
                    </td>
                    <td className="px-4 py-3 text-right">
                      <div className="flex items-center justify-end gap-2">
                        <button
                          onClick={(e) => {
                            e.stopPropagation()
                            navigate(`/dashboard/empleados/${emp.num_empleado}`)
                          }}
                          className="rounded p-1 text-indigo-600 hover:bg-indigo-50"
                          aria-label="View employee"
                        >
                          <Eye className="h-4 w-4" />
                        </button>
                        <button
                          onClick={(e) => handleDelete(e, emp.num_empleado)}
                          className="rounded p-1 text-red-500 hover:bg-red-50"
                          aria-label="Deactivate employee"
                        >
                          <Trash2 className="h-4 w-4" />
                        </button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}

        {/* Wizard Modal */}
        {wizardOpen && (
          <div
            className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 backdrop-blur-sm"
            onClick={() => setWizardOpen(false)}
          >
            <div
              className="w-full max-w-md rounded-xl bg-white p-6 shadow-xl"
              onClick={(e) => e.stopPropagation()}
            >
              <EmployeeWizard empresaId={empresaId} onClose={() => setWizardOpen(false)} />
            </div>
          </div>
        )}
      </div>
    </DashboardLayout>
  )
}

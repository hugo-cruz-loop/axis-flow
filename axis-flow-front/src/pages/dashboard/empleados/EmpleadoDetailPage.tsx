import { useParams, useSearchParams } from 'react-router-dom'
import { DashboardLayout } from '@/layouts/DashboardLayout'
import { EmployeeDossier } from '@/components/containers/EmployeeDossier'
import { AttendanceMonitor } from '@/components/containers/AttendanceMonitor'
import { FileDropzone } from '@/components/presentational/FileDropzone'
import { useEmpleado, useFotologin, useDevices, useUploadFotologin } from '@/hooks/api/useEmpleados'
import { useAuthStore } from '@/store/authStore'
import { EmpleadoStatus } from '@/types/empleados'
import { cn } from '@/lib/utils'

type Tab = 'expediente' | 'asistencias' | 'fotos' | 'dispositivos'

function StatusBadge({ status }: { status: number }) {
  const config: Record<number, { label: string; className: string }> = {
    [EmpleadoStatus.ACTIVO]: { label: 'Active', className: 'bg-green-100 text-green-700' },
    [EmpleadoStatus.INCOMPLETO]: { label: 'Incomplete', className: 'bg-amber-100 text-amber-700' },
    [EmpleadoStatus.BAJA]: { label: 'Inactive', className: 'bg-red-100 text-red-700' },
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

export function EmpleadoDetailPage() {
  const { numEmpleado: numEmpleadoParam } = useParams<{ numEmpleado: string }>()
  const numEmpleado = Number(numEmpleadoParam)

  const [searchParams, setSearchParams] = useSearchParams()
  const activeTab = (searchParams.get('tab') as Tab) ?? 'expediente'

  const user = useAuthStore((s) => s.user)
  const empresaId = (user as unknown as { empresa_id?: number })?.empresa_id ?? 1

  const { data: empleado, isLoading } = useEmpleado(numEmpleado)

  const setTab = (tab: Tab) => {
    setSearchParams({ tab }, { replace: true })
  }

  if (isLoading) {
    return (
      <DashboardLayout>
        <p className="text-sm text-gray-500">Loading employee...</p>
      </DashboardLayout>
    )
  }

  if (!empleado) {
    return (
      <DashboardLayout>
        <p className="text-sm text-red-500">Employee not found.</p>
      </DashboardLayout>
    )
  }

  return (
    <DashboardLayout>
      <div className="space-y-6">
        {/* Header */}
        <div className="flex items-start gap-4">
          <div className="flex h-14 w-14 shrink-0 items-center justify-center rounded-full bg-indigo-100 text-lg font-bold text-indigo-600">
            {empleado.nombre.charAt(0)}
            {empleado.apellido_paterno.charAt(0)}
          </div>
          <div>
            <h1 className="text-xl font-bold text-gray-900">
              {empleado.nombre} {empleado.apellido_paterno}{' '}
              {empleado.apellido_materno ?? ''}
            </h1>
            <div className="mt-1 flex items-center gap-3">
              <span className="text-sm text-gray-500">ID: {empleado.id_empleado}</span>
              <StatusBadge status={empleado.status} />
            </div>
          </div>
        </div>

        {/* Tabs */}
        <div className="flex gap-1 border-b border-gray-200">
          {(
            [
              { id: 'expediente', label: 'Dossier' },
              { id: 'asistencias', label: 'Attendance' },
              { id: 'fotos', label: 'Base Photos' },
              { id: 'dispositivos', label: 'Devices' },
            ] as Array<{ id: Tab; label: string }>
          ).map(({ id, label }) => (
            <button
              key={id}
              onClick={() => setTab(id)}
              className={cn(
                'px-4 py-2 text-sm font-medium transition-colors',
                activeTab === id
                  ? 'border-b-2 border-indigo-600 text-indigo-600'
                  : 'text-gray-500 hover:text-gray-900',
              )}
            >
              {label}
            </button>
          ))}
        </div>

        {/* Tab content */}
        {activeTab === 'expediente' && <EmployeeDossier numEmpleado={numEmpleado} />}
        {activeTab === 'asistencias' && (
          <AttendanceMonitor empresaId={empresaId} empleadoId={numEmpleado} />
        )}
        {activeTab === 'fotos' && <FotosTab numEmpleado={numEmpleado} empresaId={empresaId} />}
        {activeTab === 'dispositivos' && <DispositivosTab numEmpleado={numEmpleado} empresaId={empresaId} />}
      </div>
    </DashboardLayout>
  )
}

// — Fotos base tab —

function FotosTab({ numEmpleado, empresaId }: { numEmpleado: number; empresaId: number }) {
  const { data: fotos = [], isLoading } = useFotologin(numEmpleado, empresaId)
  const { mutate: upload, isPending } = useUploadFotologin()

  const handleFile = (file: File) => {
    upload({ numEmpleado, empresaId, file })
  }

  return (
    <div className="flex flex-col gap-4">
      <div className="flex items-center justify-between">
        <h3 className="text-sm font-semibold text-gray-700">Base Biometric Photos</h3>
        <div className="w-60">
          <FileDropzone
            accept=".jpg,.jpeg,.png"
            maxSizeMB={10}
            onFile={handleFile}
            label="Upload photo"
            disabled={isPending}
          />
        </div>
      </div>

      {isLoading ? (
        <p className="text-sm text-gray-500">Loading...</p>
      ) : fotos.length === 0 ? (
        <p className="text-sm text-gray-500">No base photos registered.</p>
      ) : (
        <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
          {fotos.map((foto) => (
            <div key={foto.id} className="overflow-hidden rounded-lg border border-gray-200">
              <img
                src={foto.foto_base_url}
                alt="Base biometric photo"
                className="h-32 w-full object-cover"
              />
              <p className="px-2 py-1 text-xs text-gray-400">
                {new Date(foto.created_at).toLocaleDateString()}
              </p>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}

// — Dispositivos tab —

function DispositivosTab({ numEmpleado, empresaId }: { numEmpleado: number; empresaId: number }) {
  const { data: devices = [], isLoading } = useDevices(numEmpleado, empresaId)

  return (
    <div className="flex flex-col gap-4">
      <h3 className="text-sm font-semibold text-gray-700">Authorized Devices</h3>
      {isLoading ? (
        <p className="text-sm text-gray-500">Loading...</p>
      ) : devices.length === 0 ? (
        <p className="text-sm text-gray-500">No devices registered.</p>
      ) : (
        <div className="overflow-hidden rounded-xl border border-gray-200 bg-white">
          <table className="w-full text-sm">
            <thead className="bg-gray-50 text-xs uppercase text-gray-500">
              <tr>
                <th className="px-4 py-3 text-left">Model</th>
                <th className="px-4 py-3 text-left">OS Version</th>
                <th className="px-4 py-3 text-left">Status</th>
                <th className="px-4 py-3 text-left">Registered</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100">
              {devices.map((device) => (
                <tr key={device.id} className="hover:bg-gray-50">
                  <td className="px-4 py-3 text-gray-900">{device.device_model ?? '—'}</td>
                  <td className="px-4 py-3 text-gray-600">{device.os_version ?? '—'}</td>
                  <td className="px-4 py-3">
                    <span
                      className={cn(
                        'rounded-full px-2 py-0.5 text-xs font-medium',
                        device.is_active
                          ? 'bg-green-100 text-green-700'
                          : 'bg-red-100 text-red-700',
                      )}
                    >
                      {device.is_active ? 'Active' : 'Inactive'}
                    </span>
                  </td>
                  <td className="px-4 py-3 text-gray-500">
                    {new Date(device.created_at).toLocaleDateString()}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}

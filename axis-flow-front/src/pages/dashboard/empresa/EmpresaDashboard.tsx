import { useState } from 'react'
import { useParams, Navigate } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { Pencil, X, Check } from 'lucide-react'
import { DashboardLayout } from '@/layouts/DashboardLayout'
import { FiscalConfigPanel } from '@/components/containers/FiscalConfigPanel'
import { ServicesManager } from '@/components/containers/ServicesManager'
import { ApoderadosDrawer } from '@/components/presentational/ApoderadosDrawer'
import {
  getEmpresa,
  updateEmpresa,
  listApoderados,
  createApoderado,
  updateApoderado,
  deleteApoderado,
} from '@/api/empresasClient'
import type { EmpresaStatus, Apoderado } from '@/types/empresas'
import { cn } from '@/lib/utils'

type Tab = 'profile' | 'fiscal' | 'apoderados' | 'services'

const TABS: { id: Tab; label: string }[] = [
  { id: 'profile', label: 'Profile' },
  { id: 'fiscal', label: 'Fiscal' },
  { id: 'apoderados', label: 'Apoderados' },
  { id: 'services', label: 'Services' },
]

const STATUS_STYLES: Record<EmpresaStatus, string> = {
  ACTIVE: 'bg-green-100 text-green-600',
  PENDING_PAYMENT: 'bg-amber-100 text-amber-500',
  INACTIVE: 'bg-slate-100 text-slate-500',
  SUSPENDED: 'bg-red-100 text-red-600',
}

const profileSchema = z.object({
  nombre: z.string().min(3, 'Min 3 characters').max(150, 'Max 150 characters'),
  direccion: z.string().min(5, 'Min 5 characters').max(255, 'Max 255 characters'),
  telefono: z.string().regex(/^\d{10}$/, 'Must be exactly 10 digits'),
})

type ProfileFormValues = z.infer<typeof profileSchema>

function EmpresaStatusBadge({ status }: { status: EmpresaStatus }) {
  return (
    <span
      className={cn(
        'rounded-full px-2.5 py-0.5 text-xs font-semibold',
        STATUS_STYLES[status],
      )}
    >
      {status.replace('_', ' ')}
    </span>
  )
}

export function EmpresaDashboard() {
  const { id: idParam } = useParams<{ id: string }>()
  const empresaId = idParam ? parseInt(idParam, 10) : NaN
  const [activeTab, setActiveTab] = useState<Tab>('profile')
  const [editingProfile, setEditingProfile] = useState(false)
  const [drawerOpen, setDrawerOpen] = useState(false)
  const [editingApoderado, setEditingApoderado] = useState<Apoderado | null>(null)
  const [deletingApoderadoId, setDeletingApoderadoId] = useState<number | null>(null)
  const queryClient = useQueryClient()

  const { data: empresa, isLoading: empresaLoading } = useQuery({
    queryKey: ['empresas', empresaId],
    queryFn: () => getEmpresa(empresaId),
    staleTime: 5 * 60 * 1000,
    enabled: !isNaN(empresaId),
  })

  const { data: apoderados, isLoading: apoderadosLoading } = useQuery({
    queryKey: ['empresas', 'apoderados', empresaId],
    queryFn: () => listApoderados(empresaId),
    enabled: activeTab === 'apoderados' && !isNaN(empresaId),
  })

  const updateEmpresaMutation = useMutation({
    mutationFn: (data: ProfileFormValues) => updateEmpresa(empresaId, data),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['empresas', empresaId] })
      setEditingProfile(false)
    },
  })

  const createApoderadoMutation = useMutation({
    mutationFn: (data: Omit<Apoderado, 'id' | 'empresa_id'>) =>
      createApoderado(empresaId, data),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['empresas', 'apoderados', empresaId] })
      setDrawerOpen(false)
    },
  })

  const updateApoderadoMutation = useMutation({
    mutationFn: ({ id, data }: { id: number; data: Partial<Apoderado> }) =>
      updateApoderado(empresaId, id, data),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['empresas', 'apoderados', empresaId] })
      setDrawerOpen(false)
      setEditingApoderado(null)
    },
  })

  const deleteApoderadoMutation = useMutation({
    mutationFn: (id: number) => deleteApoderado(empresaId, id),
    onSuccess: () =>
      queryClient.invalidateQueries({ queryKey: ['empresas', 'apoderados', empresaId] }),
  })

  const {
    register,
    handleSubmit,
    reset: resetProfile,
    formState: { errors: profileErrors },
  } = useForm<ProfileFormValues>({
    resolver: zodResolver(profileSchema),
    defaultValues: empresa
      ? { nombre: empresa.nombre, direccion: empresa.direccion, telefono: empresa.telefono }
      : undefined,
  })

  if (isNaN(empresaId)) {
    return <Navigate to="/dashboard" replace />
  }

  const handleApoderadoSubmit = async (data: Omit<Apoderado, 'id' | 'empresa_id'>) => {
    if (editingApoderado) {
      await updateApoderadoMutation.mutateAsync({ id: editingApoderado.id, data })
    } else {
      await createApoderadoMutation.mutateAsync(data)
    }
  }

  const handleDeleteApoderado = async (id: number) => {
    setDeletingApoderadoId(id)
    try {
      await deleteApoderadoMutation.mutateAsync(id)
    } finally {
      setDeletingApoderadoId(null)
    }
  }

  return (
    <DashboardLayout>
      {/* Header */}
      <div className="mb-6 flex flex-wrap items-center justify-between gap-4">
        <div>
          {empresaLoading ? (
            <div className="h-8 w-48 animate-pulse rounded-lg bg-slate-200" />
          ) : (
            <div className="flex items-center gap-3">
              <h1 className="text-2xl font-bold text-slate-900">
                {empresa?.nombre ?? 'Company'}
              </h1>
              {empresa && <EmpresaStatusBadge status={empresa.status} />}
            </div>
          )}
          <p className="mt-1 text-sm text-slate-500">Company management</p>
        </div>
      </div>

      {/* Tabs */}
      <div className="mb-6 border-b border-slate-200">
        <div className="flex gap-1">
          {TABS.map((tab) => (
            <button
              key={tab.id}
              onClick={() => setActiveTab(tab.id)}
              className={cn(
                'px-4 py-2.5 text-sm font-medium transition-colors',
                activeTab === tab.id
                  ? 'border-b-2 border-indigo-600 text-indigo-600'
                  : 'text-slate-500 hover:text-slate-900',
              )}
            >
              {tab.label}
            </button>
          ))}
        </div>
      </div>

      {/* Tab content */}
      {activeTab === 'profile' && (
        <div className="rounded-xl border border-slate-200 bg-white p-6 shadow-sm">
          <div className="mb-4 flex items-center justify-between">
            <h2 className="font-semibold text-slate-900">Company Profile</h2>
            {!editingProfile && empresa && (
              <button
                onClick={() => {
                  resetProfile({
                    nombre: empresa.nombre,
                    direccion: empresa.direccion,
                    telefono: empresa.telefono,
                  })
                  setEditingProfile(true)
                }}
                className="flex items-center gap-1.5 rounded-lg border border-slate-300 px-3 py-1.5 text-sm text-slate-700 hover:bg-slate-50"
              >
                <Pencil className="h-3.5 w-3.5" />
                Edit
              </button>
            )}
          </div>

          {editingProfile ? (
            <form
              onSubmit={(e) => void handleSubmit((data) => updateEmpresaMutation.mutate(data))(e)}
              className="space-y-4"
            >
              <div>
                <label className="block text-sm font-medium text-slate-700">
                  Name <span className="text-red-500">*</span>
                </label>
                <input
                  {...register('nombre')}
                  className="mt-1 h-9 w-full rounded-lg border border-slate-300 px-3 text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
                />
                {profileErrors.nombre && (
                  <p className="mt-1 text-xs text-red-500">{profileErrors.nombre.message}</p>
                )}
              </div>
              <div>
                <label className="block text-sm font-medium text-slate-700">
                  Address <span className="text-red-500">*</span>
                </label>
                <input
                  {...register('direccion')}
                  className="mt-1 h-9 w-full rounded-lg border border-slate-300 px-3 text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
                />
                {profileErrors.direccion && (
                  <p className="mt-1 text-xs text-red-500">{profileErrors.direccion.message}</p>
                )}
              </div>
              <div>
                <label className="block text-sm font-medium text-slate-700">
                  Phone <span className="text-red-500">*</span>
                </label>
                <input
                  {...register('telefono')}
                  type="tel"
                  maxLength={10}
                  className="mt-1 h-9 w-full rounded-lg border border-slate-300 px-3 text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
                />
                {profileErrors.telefono && (
                  <p className="mt-1 text-xs text-red-500">{profileErrors.telefono.message}</p>
                )}
              </div>
              <div className="flex gap-3">
                <button
                  type="button"
                  onClick={() => setEditingProfile(false)}
                  className="flex items-center gap-1.5 rounded-lg border border-slate-300 px-4 py-2 text-sm font-medium text-slate-700 hover:bg-slate-50"
                >
                  <X className="h-3.5 w-3.5" />
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={updateEmpresaMutation.isPending}
                  className="flex items-center gap-1.5 rounded-lg bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-700 disabled:opacity-50"
                >
                  <Check className="h-3.5 w-3.5" />
                  {updateEmpresaMutation.isPending ? 'Saving…' : 'Save'}
                </button>
              </div>
            </form>
          ) : (
            <dl className="grid grid-cols-1 gap-4 sm:grid-cols-2 text-sm">
              <div>
                <dt className="text-slate-500">Name</dt>
                <dd className="font-medium text-slate-900">{empresa?.nombre ?? '—'}</dd>
              </div>
              <div>
                <dt className="text-slate-500">Phone</dt>
                <dd className="font-medium text-slate-900">{empresa?.telefono ?? '—'}</dd>
              </div>
              <div className="sm:col-span-2">
                <dt className="text-slate-500">Address</dt>
                <dd className="font-medium text-slate-900">{empresa?.direccion ?? '—'}</dd>
              </div>
            </dl>
          )}
        </div>
      )}

      {activeTab === 'fiscal' && <FiscalConfigPanel empresaId={empresaId} />}

      {activeTab === 'apoderados' && (
        <div>
          <div className="mb-4 flex justify-end">
            <button
              onClick={() => {
                setEditingApoderado(null)
                setDrawerOpen(true)
              }}
              className="flex items-center gap-2 rounded-lg bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-700"
            >
              Add Apoderado
            </button>
          </div>

          <div className="overflow-x-auto rounded-xl border border-slate-200 bg-white shadow-sm">
            <table className="min-w-full">
              <thead>
                <tr className="border-b border-slate-200 bg-slate-50">
                  <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-slate-500">
                    Name
                  </th>
                  <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-slate-500">
                    RFC
                  </th>
                  <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-slate-500">
                    CURP
                  </th>
                  <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-slate-500">
                    Email
                  </th>
                  <th className="w-20 px-4 py-3" />
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-100">
                {apoderadosLoading
                  ? Array.from({ length: 3 }).map((_, i) => (
                      <tr key={i}>
                        {Array.from({ length: 5 }).map((__, j) => (
                          <td key={j} className="px-4 py-3">
                            <div className="h-4 animate-pulse rounded bg-slate-100" />
                          </td>
                        ))}
                      </tr>
                    ))
                  : (apoderados ?? []).map((a) => (
                      <tr key={a.id} className="hover:bg-slate-50 transition-colors">
                        <td className="px-4 py-3 text-sm font-medium text-slate-900">
                          {a.nombre}
                        </td>
                        <td className="px-4 py-3 font-mono text-sm text-slate-700">{a.rfc}</td>
                        <td className="px-4 py-3 font-mono text-sm text-slate-700">{a.curp}</td>
                        <td className="px-4 py-3 text-sm text-slate-700">{a.email}</td>
                        <td className="px-4 py-3">
                          <div className="flex items-center gap-1">
                            <button
                              onClick={() => {
                                setEditingApoderado(a)
                                setDrawerOpen(true)
                              }}
                              className="rounded p-1 text-slate-400 hover:bg-slate-100 hover:text-slate-600"
                              aria-label={`Edit ${a.nombre}`}
                            >
                              <Pencil className="h-4 w-4" />
                            </button>
                            <button
                              onClick={() => void handleDeleteApoderado(a.id)}
                              disabled={deletingApoderadoId === a.id}
                              className="rounded p-1 text-slate-400 hover:bg-red-50 hover:text-red-600 disabled:opacity-40"
                              aria-label={`Delete ${a.nombre}`}
                            >
                              <Pencil className="h-4 w-4" />
                            </button>
                          </div>
                        </td>
                      </tr>
                    ))}
                {!apoderadosLoading && (apoderados ?? []).length === 0 && (
                  <tr>
                    <td colSpan={5} className="px-4 py-8 text-center text-sm text-slate-400">
                      No apoderados registered.
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>

          <ApoderadosDrawer
            open={drawerOpen}
            onClose={() => {
              setDrawerOpen(false)
              setEditingApoderado(null)
            }}
            apoderado={editingApoderado}
            onSubmit={handleApoderadoSubmit}
            isSubmitting={createApoderadoMutation.isPending || updateApoderadoMutation.isPending}
          />
        </div>
      )}

      {activeTab === 'services' && <ServicesManager empresaId={empresaId} />}
    </DashboardLayout>
  )
}

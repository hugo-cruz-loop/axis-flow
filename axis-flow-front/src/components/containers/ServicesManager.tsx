import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { Plus, Pencil, Trash2, X } from 'lucide-react'
import { listServicios, createServicio, updateServicio, deleteServicio } from '@/api/empresasClient'
import type { Servicio } from '@/types/empresas'

interface ServicesManagerProps {
  empresaId: number
}

const servicioSchema = z.object({
  nombre: z.string().min(1, 'Name is required'),
  descripcion: z.string().optional(),
  precio: z
    .string()
    .min(1, 'Price is required')
    .refine((v) => !isNaN(parseFloat(v)) && parseFloat(v) > 0, {
      message: 'Price must be greater than 0',
    }),
  status_activo: z.boolean(),
})

type ServicioFormValues = z.infer<typeof servicioSchema>

interface ServiceModalProps {
  servicio?: Servicio | null
  onClose: () => void
  onSubmit: (data: ServicioFormValues) => Promise<void>
  isSubmitting: boolean
}

function ServiceModal({ servicio, onClose, onSubmit, isSubmitting }: ServiceModalProps) {
  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useForm<ServicioFormValues>({
    resolver: zodResolver(servicioSchema),
    defaultValues: servicio
      ? {
          nombre: servicio.nombre,
          descripcion: servicio.descripcion ?? '',
          precio: String(servicio.precio),
          status_activo: servicio.status_activo,
        }
      : { nombre: '', descripcion: '', precio: '', status_activo: true },
  })

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center">
      <div
        className="absolute inset-0 bg-slate-900/40 backdrop-blur-sm"
        onClick={onClose}
        aria-hidden="true"
      />
      <div className="relative w-full max-w-md rounded-xl bg-white p-6 shadow-xl">
        <div className="mb-4 flex items-center justify-between">
          <h3 className="font-semibold text-slate-900">
            {servicio ? 'Edit Service' : 'Add Service'}
          </h3>
          <button
            onClick={onClose}
            className="rounded-lg p-1.5 text-slate-400 hover:bg-slate-100"
            aria-label="Close"
          >
            <X className="h-5 w-5" />
          </button>
        </div>
        <form onSubmit={(e) => void handleSubmit(onSubmit)(e)} className="space-y-4">
          <div>
            <label className="block text-sm font-medium text-slate-700">
              Name <span className="text-red-500">*</span>
            </label>
            <input
              {...register('nombre')}
              className="mt-1 h-9 w-full rounded-lg border border-slate-300 px-3 text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
            />
            {errors.nombre && (
              <p className="mt-1 text-xs text-red-500">{errors.nombre.message}</p>
            )}
          </div>
          <div>
            <label className="block text-sm font-medium text-slate-700">Description</label>
            <input
              {...register('descripcion')}
              className="mt-1 h-9 w-full rounded-lg border border-slate-300 px-3 text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
            />
          </div>
          <div>
            <label className="block text-sm font-medium text-slate-700">
              Price <span className="text-red-500">*</span>
            </label>
            <input
              {...register('precio')}
              type="number"
              step="0.01"
              min="0.01"
              className="mt-1 h-9 w-full rounded-lg border border-slate-300 px-3 text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
            />
            {errors.precio && (
              <p className="mt-1 text-xs text-red-500">{errors.precio.message}</p>
            )}
          </div>
          <div className="flex items-center gap-2">
            <input
              {...register('status_activo')}
              type="checkbox"
              id="status_activo"
              className="h-4 w-4 rounded border-slate-300 text-indigo-600"
            />
            <label htmlFor="status_activo" className="text-sm text-slate-700">
              Active
            </label>
          </div>
          <div className="flex gap-3 pt-2">
            <button
              type="button"
              onClick={onClose}
              className="flex-1 rounded-lg border border-slate-300 px-4 py-2 text-sm font-medium text-slate-700 hover:bg-slate-50"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={isSubmitting}
              className="flex-1 rounded-lg bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-700 disabled:opacity-50"
            >
              {isSubmitting ? 'Saving…' : servicio ? 'Update' : 'Create'}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}

export function ServicesManager({ empresaId }: ServicesManagerProps) {
  const queryClient = useQueryClient()
  const [modalOpen, setModalOpen] = useState(false)
  const [editingServicio, setEditingServicio] = useState<Servicio | null>(null)
  const [deletingId, setDeletingId] = useState<number | null>(null)

  const { data: servicios, isLoading } = useQuery({
    queryKey: ['empresas', 'servicios', empresaId],
    queryFn: () => listServicios(empresaId),
    staleTime: 5 * 60 * 1000,
  })

  const invalidate = () =>
    queryClient.invalidateQueries({ queryKey: ['empresas', 'servicios', empresaId] })

  const createMutation = useMutation({
    mutationFn: (data: Omit<Servicio, 'id' | 'empresa_id'>) =>
      createServicio(empresaId, data),
    onSuccess: () => {
      void invalidate()
      setModalOpen(false)
    },
  })

  const updateMutation = useMutation({
    mutationFn: ({ id, data }: { id: number; data: Partial<Servicio> }) =>
      updateServicio(empresaId, id, data),
    onSuccess: () => {
      void invalidate()
      setModalOpen(false)
      setEditingServicio(null)
    },
  })

  const deleteMutation = useMutation({
    mutationFn: (id: number) => deleteServicio(empresaId, id),
    onSuccess: () => void invalidate(),
  })

  const handleSubmit = async (data: ServicioFormValues) => {
    const normalized: Omit<Servicio, 'id' | 'empresa_id'> = {
      nombre: data.nombre,
      descripcion: data.descripcion,
      precio: parseFloat(data.precio),
      status_activo: data.status_activo,
    }
    if (editingServicio) {
      await updateMutation.mutateAsync({ id: editingServicio.id, data: normalized })
    } else {
      await createMutation.mutateAsync(normalized)
    }
  }

  const handleDelete = async (id: number) => {
    setDeletingId(id)
    try {
      await deleteMutation.mutateAsync(id)
    } finally {
      setDeletingId(null)
    }
  }

  const handleToggle = (servicio: Servicio) => {
    void updateMutation.mutateAsync({
      id: servicio.id,
      data: { status_activo: !servicio.status_activo },
    })
  }

  return (
    <div>
      <div className="mb-4 flex justify-end">
        <button
          onClick={() => {
            setEditingServicio(null)
            setModalOpen(true)
          }}
          className="flex items-center gap-2 rounded-lg bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-700"
        >
          <Plus className="h-4 w-4" />
          Add Service
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
                Price
              </th>
              <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-slate-500">
                Status
              </th>
              <th className="w-24 px-4 py-3" />
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-100">
            {isLoading
              ? Array.from({ length: 3 }).map((_, i) => (
                  <tr key={i}>
                    {Array.from({ length: 4 }).map((__, j) => (
                      <td key={j} className="px-4 py-3">
                        <div className="h-4 animate-pulse rounded bg-slate-100" />
                      </td>
                    ))}
                  </tr>
                ))
              : (servicios ?? []).map((s) => (
                  <tr key={s.id} className="hover:bg-slate-50 transition-colors">
                    <td className="px-4 py-3 text-sm font-medium text-slate-900">
                      {s.nombre}
                      {s.descripcion && (
                        <p className="text-xs text-slate-500">{s.descripcion}</p>
                      )}
                    </td>
                    <td className="px-4 py-3 text-sm text-slate-700">
                      {new Intl.NumberFormat('en-US', {
                        style: 'currency',
                        currency: 'MXN',
                      }).format(s.precio)}
                    </td>
                    <td className="px-4 py-3">
                      <button
                        onClick={() => handleToggle(s)}
                        className={`rounded-full px-2.5 py-0.5 text-xs font-medium transition-colors ${
                          s.status_activo
                            ? 'bg-green-100 text-green-700 hover:bg-green-200'
                            : 'bg-slate-100 text-slate-500 hover:bg-slate-200'
                        }`}
                      >
                        {s.status_activo ? 'Active' : 'Inactive'}
                      </button>
                    </td>
                    <td className="px-4 py-3">
                      <div className="flex items-center gap-1">
                        <button
                          onClick={() => {
                            setEditingServicio(s)
                            setModalOpen(true)
                          }}
                          className="rounded p-1 text-slate-400 hover:bg-slate-100 hover:text-slate-600"
                          aria-label={`Edit ${s.nombre}`}
                        >
                          <Pencil className="h-4 w-4" />
                        </button>
                        <button
                          onClick={() => void handleDelete(s.id)}
                          disabled={deletingId === s.id}
                          className="rounded p-1 text-slate-400 hover:bg-red-50 hover:text-red-600 disabled:opacity-40"
                          aria-label={`Delete ${s.nombre}`}
                        >
                          <Trash2 className="h-4 w-4" />
                        </button>
                      </div>
                    </td>
                  </tr>
                ))}
            {!isLoading && (servicios ?? []).length === 0 && (
              <tr>
                <td colSpan={4} className="px-4 py-8 text-center text-sm text-slate-400">
                  No services configured yet.
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>

      {modalOpen && (
        <ServiceModal
          servicio={editingServicio}
          onClose={() => {
            setModalOpen(false)
            setEditingServicio(null)
          }}
          onSubmit={handleSubmit}
          isSubmitting={createMutation.isPending || updateMutation.isPending}
        />
      )}
    </div>
  )
}

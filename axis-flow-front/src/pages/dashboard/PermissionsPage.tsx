import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { Pencil, Trash2, Check, X, Plus } from 'lucide-react'
import {
  listPermissions,
  createPermission,
  updatePermission,
  deletePermission,
} from '@/api/permissionsClient'
import type { Permission } from '@/types/rbac'
import { DashboardLayout } from '@/layouts/DashboardLayout'
import { useAuthStore } from '@/store/authStore'

type ToastType = 'success' | 'error'

interface Toast {
  message: string
  type: ToastType
}

function useToast() {
  const [toast, setToast] = useState<Toast | null>(null)
  const show = (message: string, type: ToastType) => {
    setToast({ message, type })
    setTimeout(() => setToast(null), 3000)
  }
  return { toast, show }
}

const permissionSchema = z.object({
  code: z
    .string()
    .min(3, 'Code is required')
    .max(120)
    .regex(/^[a-z][a-z0-9_-]*:[a-z][a-z0-9_-]*$/, 'Use module:action format'),
  name: z.string().min(3, 'Name is required').max(100),
  module: z.string().min(2, 'Module is required').max(80),
  description: z.string().max(500).optional(),
})

const editSchema = permissionSchema.pick({ name: true, module: true, description: true })

type CreateValues = z.infer<typeof permissionSchema>
type EditValues = z.infer<typeof editSchema>

interface PermissionFormProps {
  onSubmit: (values: CreateValues) => Promise<void>
  onCancel: () => void
}

function PermissionCreateForm({ onSubmit, onCancel }: PermissionFormProps) {
  const {
    register,
    handleSubmit,
    formState: { isSubmitting, errors },
  } = useForm<CreateValues>({
    resolver: zodResolver(permissionSchema),
    defaultValues: { code: '', name: '', module: '', description: '' },
  })

  return (
    <form
      onSubmit={handleSubmit(onSubmit)}
      className="mb-4 rounded-xl border border-indigo-100 bg-white p-4 shadow-sm"
    >
      <h2 className="mb-4 text-base font-semibold text-slate-900">New Permission</h2>
      <div className="grid gap-4 md:grid-cols-2">
        <div>
          <label htmlFor="permission-code" className="block text-sm font-medium text-slate-700">
            Code
          </label>
          <input
            id="permission-code"
            {...register('code')}
            placeholder="module:action"
            className="mt-1 w-full rounded border border-slate-300 px-3 py-2 text-sm focus:border-indigo-500 focus:outline-none"
          />
          {errors.code && <p className="mt-1 text-xs text-red-500">{errors.code.message}</p>}
        </div>
        <div>
          <label htmlFor="permission-name" className="block text-sm font-medium text-slate-700">
            Name
          </label>
          <input
            id="permission-name"
            {...register('name')}
            className="mt-1 w-full rounded border border-slate-300 px-3 py-2 text-sm focus:border-indigo-500 focus:outline-none"
          />
          {errors.name && <p className="mt-1 text-xs text-red-500">{errors.name.message}</p>}
        </div>
        <div>
          <label htmlFor="permission-module" className="block text-sm font-medium text-slate-700">
            Module
          </label>
          <input
            id="permission-module"
            {...register('module')}
            placeholder="roles"
            className="mt-1 w-full rounded border border-slate-300 px-3 py-2 text-sm focus:border-indigo-500 focus:outline-none"
          />
          {errors.module && <p className="mt-1 text-xs text-red-500">{errors.module.message}</p>}
        </div>
        <div>
          <label
            htmlFor="permission-description"
            className="block text-sm font-medium text-slate-700"
          >
            Description
          </label>
          <input
            id="permission-description"
            {...register('description')}
            className="mt-1 w-full rounded border border-slate-300 px-3 py-2 text-sm focus:border-indigo-500 focus:outline-none"
          />
        </div>
      </div>
      <div className="mt-4 flex justify-end gap-2">
        <button
          type="button"
          onClick={onCancel}
          className="rounded-lg border border-slate-300 px-4 py-2 text-sm text-slate-600 hover:bg-slate-50"
        >
          Cancel
        </button>
        <button
          type="submit"
          disabled={isSubmitting}
          className="rounded-lg bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-700 disabled:opacity-50"
        >
          Create Permission
        </button>
      </div>
    </form>
  )
}

interface InlineEditRowProps {
  permission: Permission
  onSave: (id: string, values: EditValues) => Promise<void>
  onCancel: () => void
}

function InlineEditRow({ permission, onSave, onCancel }: InlineEditRowProps) {
  const {
    register,
    handleSubmit,
    formState: { isSubmitting, errors },
  } = useForm<EditValues>({
    resolver: zodResolver(editSchema),
    defaultValues: {
      name: permission.name,
      module: permission.module,
      description: permission.description ?? '',
    },
  })

  return (
    <tr data-testid="permission-edit-row" className="bg-indigo-50">
      <td className="px-4 py-2 font-mono text-xs text-slate-500">{permission.code}</td>
      <td className="px-4 py-2">
        <input
          {...register('name')}
          className="w-full rounded border border-slate-300 px-2 py-1 text-sm focus:border-indigo-500 focus:outline-none"
        />
        {errors.name && <p className="text-xs text-red-500">{errors.name.message}</p>}
      </td>
      <td className="px-4 py-2">
        <input
          {...register('module')}
          className="w-full rounded border border-slate-300 px-2 py-1 text-sm focus:border-indigo-500 focus:outline-none"
        />
        {errors.module && <p className="text-xs text-red-500">{errors.module.message}</p>}
      </td>
      <td className="px-4 py-2">
        <input
          {...register('description')}
          className="w-full rounded border border-slate-300 px-2 py-1 text-sm focus:border-indigo-500 focus:outline-none"
        />
      </td>
      <td className="px-4 py-2">
        <div className="flex items-center justify-end gap-1">
          <button
            type="button"
            onClick={handleSubmit((v) => onSave(permission.id, v))}
            disabled={isSubmitting}
            className="rounded p-1 text-green-600 hover:bg-green-50"
            aria-label="Save"
          >
            <Check className="h-4 w-4" />
          </button>
          <button
            type="button"
            onClick={onCancel}
            className="rounded p-1 text-slate-500 hover:bg-slate-100"
            aria-label="Cancel"
          >
            <X className="h-4 w-4" />
          </button>
        </div>
      </td>
    </tr>
  )
}

export function PermissionsPage() {
  const qc = useQueryClient()
  const { toast, show: showToast } = useToast()
  const role = useAuthStore((s) => s.user?.role)
  const canMutatePermissions = role === 'ADMIN_CHECK_ON'
  const [selectedModule, setSelectedModule] = useState<string>('')
  const [editingId, setEditingId] = useState<string | null>(null)
  const [creating, setCreating] = useState(false)

  const { data: permissions = [], isLoading } = useQuery({
    queryKey: ['v1-permissions', selectedModule || undefined],
    queryFn: () => listPermissions(selectedModule || undefined),
  })

  const createMutation = useMutation({
    mutationFn: createPermission,
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['v1-permissions'] })
      showToast('Permission created', 'success')
      setCreating(false)
    },
    onError: (err: unknown) => {
      const error = err as { response?: { status?: number; data?: { error?: string } } }
      if (error?.response?.status === 409) {
        showToast('Permission code already exists', 'error')
      } else {
        showToast('Failed to create permission', 'error')
      }
    },
  })

  const deleteMutation = useMutation({
    mutationFn: deletePermission,
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['v1-permissions'] })
      showToast('Permission deleted', 'success')
    },
    onError: (err: unknown) => {
      const error = err as { response?: { status?: number; data?: { error?: string } } }
      if (error?.response?.status === 409) {
        showToast('Permission is assigned to one or more roles and cannot be deleted', 'error')
      } else {
        showToast('Failed to delete permission', 'error')
      }
    },
  })

  const updateMutation = useMutation({
    mutationFn: ({ id, values }: { id: string; values: EditValues }) =>
      updatePermission(id, values),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['v1-permissions'] })
      showToast('Permission updated', 'success')
      setEditingId(null)
    },
    onError: () => {
      showToast('Failed to update permission', 'error')
    },
  })

  const allModules = Array.from(new Set(permissions.map((p) => p.module))).sort()

  const handleDelete = (p: Permission) => {
    if (!canMutatePermissions) return
    if (!window.confirm(`Delete permission "${p.code}"?`)) return
    deleteMutation.mutate(p.id)
  }

  const handleSave = async (id: string, values: EditValues) => {
    if (!canMutatePermissions) return
    await updateMutation.mutateAsync({ id, values })
  }

  const handleCreate = async (values: CreateValues) => {
    if (!canMutatePermissions) return
    await createMutation.mutateAsync(values)
  }

  return (
    <DashboardLayout>
      {toast && (
        <div
          className={`fixed left-1/2 top-4 z-50 -translate-x-1/2 rounded-lg px-5 py-3 text-sm font-medium text-white shadow-lg ${
            toast.type === 'success' ? 'bg-green-600' : 'bg-red-600'
          }`}
          role="alert"
        >
          {toast.message}
        </div>
      )}

      <div className="mb-6 flex items-start justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold text-slate-900">Permissions</h1>
          <p className="mt-1 text-sm text-slate-500">Manage system permission catalog</p>
        </div>
        {canMutatePermissions && (
          <button
            type="button"
            data-testid="new-permission-button"
            onClick={() => setCreating(true)}
            className="flex items-center gap-2 rounded-lg bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-700"
          >
            <Plus className="h-4 w-4" />
            New Permission
          </button>
        )}
      </div>

      {creating && (
        <PermissionCreateForm onSubmit={handleCreate} onCancel={() => setCreating(false)} />
      )}

      <div className="mb-4 flex items-center gap-3">
        <label className="text-sm font-medium text-slate-700">Module:</label>
        <select
          value={selectedModule}
          onChange={(e) => setSelectedModule(e.target.value)}
          className="rounded-lg border border-slate-300 px-3 py-1.5 text-sm focus:border-indigo-500 focus:outline-none"
          data-testid="module-filter"
        >
          <option value="">All</option>
          {allModules.map((m) => (
            <option key={m} value={m}>
              {m}
            </option>
          ))}
        </select>
      </div>

      <div className="overflow-x-auto rounded-xl border border-slate-200 bg-white shadow-sm">
        <table className="min-w-full divide-y divide-slate-200">
          <thead>
            <tr className="bg-slate-50">
              <th className="px-4 py-3 text-left text-xs font-semibold uppercase tracking-wider text-slate-500">
                Code
              </th>
              <th className="px-4 py-3 text-left text-xs font-semibold uppercase tracking-wider text-slate-500">
                Name
              </th>
              <th className="px-4 py-3 text-left text-xs font-semibold uppercase tracking-wider text-slate-500">
                Module
              </th>
              <th className="px-4 py-3 text-left text-xs font-semibold uppercase tracking-wider text-slate-500">
                Description
              </th>
              <th className="px-4 py-3 text-right text-xs font-semibold uppercase tracking-wider text-slate-500">
                Actions
              </th>
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-100">
            {isLoading
              ? Array.from({ length: 5 }).map((_, i) => (
                  <tr key={i}>
                    {Array.from({ length: 5 }).map((_, j) => (
                      <td key={j} className="px-4 py-3">
                        <div className="h-4 animate-pulse rounded bg-slate-100" />
                      </td>
                    ))}
                  </tr>
                ))
              : permissions.map((p) =>
                  editingId === p.id ? (
                    <InlineEditRow
                      key={p.id}
                      permission={p}
                      onSave={handleSave}
                      onCancel={() => setEditingId(null)}
                    />
                  ) : (
                    <tr key={p.id} data-testid="permission-row" className="hover:bg-slate-50">
                      <td className="px-4 py-3 font-mono text-xs text-slate-500">{p.code}</td>
                      <td className="px-4 py-3 text-sm text-slate-900">{p.name}</td>
                      <td className="px-4 py-3">
                        <span className="inline-flex rounded-full bg-slate-100 px-2 py-0.5 text-xs font-medium text-slate-600">
                          {p.module}
                        </span>
                      </td>
                      <td className="px-4 py-3 text-sm text-slate-500">{p.description ?? '—'}</td>
                      <td className="px-4 py-3">
                        <div className="flex items-center justify-end gap-2">
                          <button
                            data-testid="permission-edit-button"
                            onClick={() => setEditingId(p.id)}
                            disabled={!canMutatePermissions}
                            className={`rounded-md border border-slate-300 p-1.5 text-slate-600 hover:bg-slate-50 ${
                              canMutatePermissions ? '' : 'cursor-not-allowed opacity-50'
                            }`}
                            title={
                              canMutatePermissions
                                ? 'Edit'
                                : 'Only ADMIN_CHECK_ON can edit permissions'
                            }
                          >
                            <Pencil className="h-3.5 w-3.5" />
                          </button>
                          <button
                            data-testid="permission-delete-button"
                            onClick={() => handleDelete(p)}
                            disabled={!canMutatePermissions}
                            className={`rounded-md border border-red-200 p-1.5 text-red-500 hover:bg-red-50 ${
                              canMutatePermissions ? '' : 'cursor-not-allowed opacity-50'
                            }`}
                            title={
                              canMutatePermissions
                                ? 'Delete'
                                : 'Only ADMIN_CHECK_ON can delete permissions'
                            }
                          >
                            <Trash2 className="h-3.5 w-3.5" />
                          </button>
                        </div>
                      </td>
                    </tr>
                  ),
                )}
          </tbody>
        </table>

        {!isLoading && permissions.length === 0 && (
          <div className="px-4 py-10 text-center text-sm text-slate-400">No permissions found</div>
        )}
      </div>
    </DashboardLayout>
  )
}

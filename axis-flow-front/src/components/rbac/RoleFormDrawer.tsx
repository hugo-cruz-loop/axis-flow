import { useEffect, useMemo } from 'react'
import { useForm, Controller, useWatch } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { X } from 'lucide-react'
import { useMutation } from '@tanstack/react-query'
import type { RoleWithCount, Permission } from '@/types/rbac'
import {
  createRole,
  updateRole,
  assignPermissionToRole,
  revokePermissionFromRole,
  listRolePermissionsById,
} from '@/api/rolesClient'

const schema = z.object({
  code: z
    .string()
    .min(3)
    .max(50)
    .regex(/^[A-Z0-9_]+$/, 'Must be UPPERCASE_SNAKE_CASE'),
  name: z.string().min(3).max(100),
  description: z.string().max(500).optional(),
  scope: z.enum(['GLOBAL', 'TENANT']),
  permissionIds: z.array(z.string()).min(1, 'Select at least one permission'),
})

type FormValues = z.infer<typeof schema>

interface RoleFormDrawerProps {
  open: boolean
  onClose: () => void
  role?: RoleWithCount | null
  permissions: Permission[]
  onSuccess: () => void
}

function groupByModule(permissions: Permission[]): Record<string, Permission[]> {
  return permissions.reduce<Record<string, Permission[]>>((acc, p) => {
    const mod = p.module || 'general'
    if (!acc[mod]) acc[mod] = []
    acc[mod].push(p)
    return acc
  }, {})
}

export function RoleFormDrawer({ open, onClose, role, permissions, onSuccess }: RoleFormDrawerProps) {
  const isEdit = role != null

  const {
    register,
    handleSubmit,
    control,
    reset,
    setValue,
    formState: { errors, isSubmitting },
  } = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: {
      code: '',
      name: '',
      description: '',
      scope: 'TENANT',
      permissionIds: [],
    },
  })

  // When role changes, reset form with role data
  useEffect(() => {
    if (!open) return
    if (role) {
      // Fetch existing permissions for the role
      listRolePermissionsById(role.id).then((perms) => {
        reset({
          code: role.code,
          name: role.name,
          description: role.description ?? '',
          scope: role.scope,
          permissionIds: perms.map((p) => p.id),
        })
      }).catch(() => {
        reset({
          code: role.code,
          name: role.name,
          description: role.description ?? '',
          scope: role.scope,
          permissionIds: [],
        })
      })
    } else {
      reset({
        code: '',
        name: '',
        description: '',
        scope: 'TENANT',
        permissionIds: [],
      })
    }
  }, [open, role, reset])

  const mutation = useMutation({
    mutationFn: async (values: FormValues) => {
      if (isEdit && role) {
        // Update role metadata
        await updateRole(role.id, { name: values.name, description: values.description })

        // Diff permissions
        const existing = await listRolePermissionsById(role.id)
        const existingIds = new Set(existing.map((p) => p.id))
        const newIds = new Set(values.permissionIds)

        const toAssign = values.permissionIds.filter((id) => !existingIds.has(id))
        const toRevoke = existing.filter((p) => !newIds.has(p.id)).map((p) => p.id)

        await Promise.all([
          ...toAssign.map((pid) => assignPermissionToRole(role.id, pid)),
          ...toRevoke.map((pid) => revokePermissionFromRole(role.id, pid)),
        ])
      } else {
        const newRole = await createRole({
          code: values.code,
          name: values.name,
          description: values.description,
          scope: values.scope,
        })
        await Promise.all(
          values.permissionIds.map((pid) => assignPermissionToRole(newRole.id, pid)),
        )
      }
    },
    onSuccess: () => {
      onSuccess()
      onClose()
    },
  })

  const permissionIds = useWatch({ control, name: 'permissionIds' }) ?? []
  const grouped = useMemo(() => groupByModule(permissions), [permissions])

  const togglePermission = (id: string) => {
    if (permissionIds.includes(id)) {
      setValue('permissionIds', permissionIds.filter((p) => p !== id), { shouldValidate: true })
    } else {
      setValue('permissionIds', [...permissionIds, id], { shouldValidate: true })
    }
  }

  const toggleModule = (modulePerms: Permission[]) => {
    const moduleIds = modulePerms.map((p) => p.id)
    const allSelected = moduleIds.every((id) => permissionIds.includes(id))
    if (allSelected) {
      setValue(
        'permissionIds',
        permissionIds.filter((id) => !moduleIds.includes(id)),
        { shouldValidate: true },
      )
    } else {
      const merged = Array.from(new Set([...permissionIds, ...moduleIds]))
      setValue('permissionIds', merged, { shouldValidate: true })
    }
  }

  if (!open) return null

  return (
    <>
      {/* Backdrop */}
      <div
        className="fixed inset-0 z-40 bg-slate-900/40 backdrop-blur-sm"
        onClick={onClose}
        aria-hidden="true"
      />

      {/* Drawer */}
      <div className="fixed inset-y-0 right-0 z-50 flex w-96 flex-col bg-white shadow-xl">
        {/* Header */}
        <div className="flex items-center justify-between border-b border-slate-200 px-5 py-4">
          <h2 className="text-base font-semibold text-slate-900">
            {isEdit ? 'Edit Role' : 'New Role'}
          </h2>
          <button
            onClick={onClose}
            className="rounded-md p-1 text-slate-400 hover:bg-slate-100 hover:text-slate-600"
            aria-label="Close"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        {/* Body */}
        <form
          id="role-form"
          onSubmit={handleSubmit((v) => mutation.mutate(v))}
          className="flex-1 overflow-y-auto px-5 py-4 space-y-4"
        >
          {/* Code */}
          {!isEdit && (
            <div>
              <label className="mb-1 block text-sm font-medium text-slate-700">
                Code <span className="text-red-500">*</span>
              </label>
              <input
                {...register('code')}
                data-testid="input-code"
                placeholder="UPPERCASE_SNAKE_CASE"
                className="w-full rounded-lg border border-slate-300 px-3 py-2 font-mono text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
              />
              <p className="mt-1 text-xs text-slate-400">Use UPPERCASE_SNAKE_CASE</p>
              {errors.code && (
                <p className="mt-1 text-xs text-red-500">{errors.code.message}</p>
              )}
            </div>
          )}

          {/* Name */}
          <div>
            <label className="mb-1 block text-sm font-medium text-slate-700">
              Name <span className="text-red-500">*</span>
            </label>
            <input
              {...register('name')}
              data-testid="input-name"
              placeholder="Role name"
              className="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
            />
            {errors.name && (
              <p className="mt-1 text-xs text-red-500">{errors.name.message}</p>
            )}
          </div>

          {/* Description */}
          <div>
            <label className="mb-1 block text-sm font-medium text-slate-700">Description</label>
            <textarea
              {...register('description')}
              data-testid="input-description"
              rows={3}
              placeholder="Optional description"
              className="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
            />
            {errors.description && (
              <p className="mt-1 text-xs text-red-500">{errors.description.message}</p>
            )}
          </div>

          {/* Scope */}
          <div>
            <label className="mb-1 block text-sm font-medium text-slate-700">Scope</label>
            <Controller
              name="scope"
              control={control}
              render={({ field }) => (
                <select
                  {...field}
                  data-testid="input-scope"
                  disabled={isEdit && role?.is_system}
                  className={`w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500 ${
                    isEdit && role?.is_system ? 'cursor-not-allowed opacity-50' : ''
                  }`}
                >
                  <option value="TENANT">TENANT</option>
                  <option value="GLOBAL">GLOBAL</option>
                </select>
              )}
            />
          </div>

          {/* Permissions */}
          <div>
            <label className="mb-2 block text-sm font-medium text-slate-700">
              Permissions <span className="text-red-500">*</span>
            </label>
            {errors.permissionIds && (
              <p className="mb-2 text-xs text-red-500">{errors.permissionIds.message}</p>
            )}
            {Object.entries(grouped).map(([mod, modPerms]) => {
              const moduleIds = modPerms.map((p) => p.id)
              const allSelected = moduleIds.every((id) => permissionIds.includes(id))
              return (
                <div key={mod} className="mb-3">
                  <div className="mb-1 flex items-center justify-between">
                    <span className="text-xs font-semibold uppercase tracking-wider text-slate-400">
                      {mod}
                    </span>
                    <button
                      type="button"
                      onClick={() => toggleModule(modPerms)}
                      className="text-xs text-indigo-600 hover:underline"
                    >
                      {allSelected ? 'Deselect all' : 'Select all'}
                    </button>
                  </div>
                  <div className="space-y-1 rounded-lg border border-slate-100 p-2">
                    {modPerms.map((perm) => (
                      <label
                        key={perm.id}
                        className="flex cursor-pointer items-start gap-2 rounded p-1 hover:bg-slate-50"
                      >
                        <input
                          type="checkbox"
                          checked={permissionIds.includes(perm.id)}
                          onChange={() => togglePermission(perm.id)}
                          className="mt-0.5 h-4 w-4 rounded border-slate-300 text-indigo-600"
                        />
                        <div>
                          <span className="font-mono text-xs text-slate-600">{perm.code}</span>
                          <p className="text-xs text-slate-400">{perm.name}</p>
                        </div>
                      </label>
                    ))}
                  </div>
                </div>
              )
            })}
          </div>
        </form>

        {/* Footer */}
        <div className="flex items-center justify-end gap-2 border-t border-slate-200 px-5 py-4">
          <button
            type="button"
            onClick={onClose}
            className="rounded-lg border border-slate-300 px-4 py-2 text-sm font-medium text-slate-700 hover:bg-slate-50"
          >
            Cancel
          </button>
          <button
            type="submit"
            form="role-form"
            disabled={isSubmitting || mutation.isPending}
            className="rounded-lg bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-700 disabled:cursor-not-allowed disabled:opacity-50"
          >
            {isSubmitting || mutation.isPending ? 'Saving...' : 'Save'}
          </button>
        </div>
      </div>
    </>
  )
}

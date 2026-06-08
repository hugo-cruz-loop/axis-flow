import { Lock, Pencil, Trash2 } from 'lucide-react'
import type { RoleWithCount } from '@/types/rbac'

interface RoleTableProps {
  roles: RoleWithCount[]
  onEdit: (role: RoleWithCount) => void
  onDelete: (role: RoleWithCount) => void
  isLoading: boolean
  canEdit: boolean
  canDelete: boolean
}

function SkeletonRow() {
  return (
    <tr>
      {Array.from({ length: 5 }).map((_, i) => (
        <td key={i} className="px-4 py-3">
          <div className="h-4 animate-pulse rounded bg-slate-100" />
        </td>
      ))}
    </tr>
  )
}

export function RoleTable({ roles, onEdit, onDelete, isLoading, canEdit, canDelete }: RoleTableProps) {
  return (
    <div className="overflow-x-auto rounded-xl border border-slate-200 bg-white shadow-sm">
      <table className="min-w-full divide-y divide-slate-200">
        <thead>
          <tr className="bg-slate-50">
            <th className="px-4 py-3 text-left text-xs font-semibold uppercase tracking-wider text-slate-500">
              Name
            </th>
            <th className="px-4 py-3 text-left text-xs font-semibold uppercase tracking-wider text-slate-500">
              Code
            </th>
            <th className="px-4 py-3 text-left text-xs font-semibold uppercase tracking-wider text-slate-500">
              Scope
            </th>
            <th className="px-4 py-3 text-left text-xs font-semibold uppercase tracking-wider text-slate-500">
              Permissions
            </th>
            <th className="px-4 py-3 text-right text-xs font-semibold uppercase tracking-wider text-slate-500">
              Actions
            </th>
          </tr>
        </thead>
        <tbody className="divide-y divide-slate-100">
          {isLoading
            ? Array.from({ length: 5 }).map((_, i) => <SkeletonRow key={i} />)
            : roles.map((role) => (
                <tr key={role.id} data-testid="role-row" className="hover:bg-slate-50">
                  <td className="px-4 py-3">
                    <div className="flex items-center gap-2">
                      <span className="font-medium text-slate-900">{role.name}</span>
                      {role.is_system && (
                        <Lock
                          className="h-3.5 w-3.5 text-slate-400"
                          aria-label="System role"
                          data-testid="lock-icon"
                        />
                      )}
                    </div>
                  </td>
                  <td className="px-4 py-3">
                    <span className="font-mono text-xs text-slate-500">{role.code}</span>
                  </td>
                  <td className="px-4 py-3">
                    <span
                      className={`inline-flex rounded-full px-2 py-0.5 text-xs font-medium ${
                        role.scope === 'GLOBAL'
                          ? 'bg-indigo-100 text-indigo-700'
                          : 'bg-amber-100 text-amber-700'
                      }`}
                    >
                      {role.scope}
                    </span>
                  </td>
                  <td className="px-4 py-3 text-sm text-slate-600">
                    {role.permission_count}
                  </td>
                  <td className="px-4 py-3">
                    <div className="flex items-center justify-end gap-2">
                      <button
                        data-testid="edit-button"
                        onClick={() => onEdit(role)}
                        disabled={role.is_system || !canEdit}
                        title={
                          role.is_system
                            ? 'System roles cannot be modified'
                            : canEdit
                              ? 'Edit role'
                              : 'Missing permission: roles:update'
                        }
                        className={`rounded-md border border-slate-300 p-1.5 text-slate-600 transition-colors hover:bg-slate-50 ${
                          role.is_system || !canEdit ? 'cursor-not-allowed opacity-50' : ''
                        }`}
                      >
                        <Pencil className="h-3.5 w-3.5" />
                      </button>
                      <button
                        data-testid="delete-button"
                        onClick={() => onDelete(role)}
                        disabled={role.is_system || !canDelete}
                        title={
                          role.is_system
                            ? 'System roles cannot be modified'
                            : canDelete
                              ? 'Delete role'
                              : 'Missing permission: roles:delete'
                        }
                        className={`rounded-md border border-red-200 p-1.5 text-red-500 transition-colors hover:bg-red-50 ${
                          role.is_system || !canDelete ? 'cursor-not-allowed opacity-50' : ''
                        }`}
                      >
                        <Trash2 className="h-3.5 w-3.5" />
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
        </tbody>
      </table>

      {!isLoading && roles.length === 0 && (
        <div className="px-4 py-10 text-center text-sm text-slate-400">No roles found</div>
      )}
    </div>
  )
}

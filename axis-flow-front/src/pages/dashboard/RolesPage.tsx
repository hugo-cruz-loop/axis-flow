import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Plus } from 'lucide-react'
import { listV1Roles, deleteRole } from '@/api/rolesClient'
import { listPermissions } from '@/api/permissionsClient'
import type { RoleWithCount } from '@/types/rbac'
import { DashboardLayout } from '@/layouts/DashboardLayout'
import { RoleTable } from '@/components/rbac/RoleTable'
import { RoleFormDrawer } from '@/components/rbac/RoleFormDrawer'
import { PermissionGate } from '@/components/rbac/PermissionGate'
import { useHasPermission } from '@/hooks/usePermissions'

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

export function RolesPage() {
  const qc = useQueryClient()
  const { toast, show: showToast } = useToast()

  const [drawerOpen, setDrawerOpen] = useState(false)
  const [editingRole, setEditingRole] = useState<RoleWithCount | null>(null)
  const canUpdateRoles = useHasPermission('roles:update')
  const canDeleteRoles = useHasPermission('roles:delete')

  const {
    data: roles = [],
    isLoading: rolesLoading,
  } = useQuery({
    queryKey: ['v1-roles'],
    queryFn: listV1Roles,
  })

  const { data: permissions = [] } = useQuery({
    queryKey: ['v1-permissions'],
    queryFn: () => listPermissions(),
  })

  const deleteMutation = useMutation({
    mutationFn: deleteRole,
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['v1-roles'] })
      showToast('Role deleted successfully', 'success')
    },
    onError: (err: unknown) => {
      const error = err as { response?: { status?: number; data?: { error?: string } } }
      const code = error?.response?.data?.error
      const status = error?.response?.status

      if (status === 403) {
        showToast('System roles cannot be deleted', 'error')
      } else if (code === 'role_has_users') {
        showToast('Cannot delete: users are assigned to this role', 'error')
      } else {
        showToast('Failed to delete role', 'error')
      }
    },
  })

  const handleNewRole = () => {
    setEditingRole(null)
    setDrawerOpen(true)
  }

  const handleEdit = (role: RoleWithCount) => {
    setEditingRole(role)
    setDrawerOpen(true)
  }

  const handleDelete = (role: RoleWithCount) => {
    if (!window.confirm(`Delete role "${role.name}"? This action cannot be undone.`)) return
    deleteMutation.mutate(role.id)
  }

  const handleDrawerSuccess = () => {
    void qc.invalidateQueries({ queryKey: ['v1-roles'] })
    showToast(editingRole ? 'Role updated successfully' : 'Role created successfully', 'success')
  }

  return (
    <DashboardLayout>
      {/* Toast */}
      {toast && (
        <div
          className={`fixed left-1/2 top-4 z-50 -translate-x-1/2 rounded-lg px-5 py-3 text-sm font-medium text-white shadow-lg transition-all ${
            toast.type === 'success' ? 'bg-green-600' : 'bg-red-600'
          }`}
          role="alert"
        >
          {toast.message}
        </div>
      )}

      {/* Header */}
      <div className="mb-6 flex items-start justify-between">
        <div>
          <h1 className="text-2xl font-bold text-slate-900">Roles</h1>
          <p className="mt-1 text-sm text-slate-500">Manage roles and their permissions</p>
        </div>
        <PermissionGate permission="roles:create">
          <button
            data-testid="new-role-button"
            onClick={handleNewRole}
            className="flex items-center gap-2 rounded-lg bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-700"
          >
            <Plus className="h-4 w-4" />
            New Role
          </button>
        </PermissionGate>
      </div>

      {/* Table */}
      <RoleTable
        roles={roles}
        onEdit={handleEdit}
        onDelete={handleDelete}
        isLoading={rolesLoading}
        canEdit={canUpdateRoles}
        canDelete={canDeleteRoles}
      />

      {/* Drawer */}
      <RoleFormDrawer
        open={drawerOpen}
        onClose={() => setDrawerOpen(false)}
        role={editingRole}
        permissions={permissions}
        onSuccess={handleDrawerSuccess}
      />
    </DashboardLayout>
  )
}

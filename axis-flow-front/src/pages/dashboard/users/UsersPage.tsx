import { useState, useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useNavigate } from 'react-router-dom'
import { Plus, ChevronLeft, ChevronRight, MoreHorizontal, X } from 'lucide-react'
import * as usersClient from '@/api/usersClient'
import { DashboardLayout } from '@/layouts/DashboardLayout'
import { RoleBadge } from '@/components/RoleBadge'
import { StatusBadge } from '@/components/StatusBadge'
import { EmptyState } from '@/components/EmptyState'
import { ErrorState } from '@/components/ErrorState'
import type { UserRole, UserStatus } from '@/types/users'

const USER_ROLES: UserRole[] = [
  'ADMIN_CHECK_ON',
  'CLIENTE',
  'EMPLEADO',
  'SUPERVISOR',
  'OPERACIONES',
  'GESTOR',
  'ADMINISTRADOR',
  'RH',
]

const USER_STATUSES: UserStatus[] = [
  'ACTIVE',
  'PENDING_ACTIVATION',
  'INACTIVE',
  'SUSPENDED',
  'LOCKED',
  'DELETED',
]

const PAGE_SIZE = 10

const STATUS_LABELS: Record<UserStatus, string> = {
  ACTIVE: 'Active',
  PENDING_ACTIVATION: 'Pending Activation',
  INACTIVE: 'Inactive',
  SUSPENDED: 'Suspended',
  LOCKED: 'Locked',
  DELETED: 'Deleted',
}

export function UsersPage() {
  const navigate = useNavigate()
  const [search, setSearch] = useState('')
  const [roleFilter, setRoleFilter] = useState<UserRole | ''>('')
  const [statusFilter, setStatusFilter] = useState<UserStatus | ''>('')
  const [page, setPage] = useState(1)
  const [openMenuId, setOpenMenuId] = useState<string | null>(null)

  const { data: users, isLoading, isError, refetch } = useQuery({
    queryKey: ['users'],
    queryFn: () => usersClient.listAllUsers(),
  })

  const filtered = useMemo(() => {
    if (!users) return []
    const q = search.toLowerCase()
    return users.filter((u) => {
      const matchSearch =
        !q ||
        u.first_name.toLowerCase().includes(q) ||
        u.last_name.toLowerCase().includes(q) ||
        u.email.toLowerCase().includes(q)
      const matchRole = !roleFilter || u.role === roleFilter
      const matchStatus = !statusFilter || u.status === statusFilter
      return matchSearch && matchRole && matchStatus
    })
  }, [users, search, roleFilter, statusFilter])

  const totalPages = Math.max(1, Math.ceil(filtered.length / PAGE_SIZE))
  const currentPage = Math.min(page, totalPages)
  const paginated = filtered.slice(
    (currentPage - 1) * PAGE_SIZE,
    currentPage * PAGE_SIZE,
  )

  const handleReset = () => {
    setSearch('')
    setRoleFilter('')
    setStatusFilter('')
    setPage(1)
  }

  const hasFilters = search || roleFilter || statusFilter

  return (
    <DashboardLayout>
      {/* Header */}
      <div className="mb-6 flex flex-wrap items-start justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold text-slate-900">Users</h1>
          <p className="mt-1 text-sm text-slate-500">Manage platform users</p>
        </div>
        <button
          onClick={() => navigate('/dashboard/users/new')}
          className="flex items-center gap-2 rounded-lg bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-500 focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:ring-offset-2"
        >
          <Plus className="h-4 w-4" />
          New User
        </button>
      </div>

      {/* Filter bar */}
      <div className="mb-4 flex flex-wrap items-center gap-2">
        <input
          type="text"
          placeholder="Search by name or email..."
          value={search}
          onChange={(e) => {
            setSearch(e.target.value)
            setPage(1)
          }}
          className="h-9 rounded-lg border border-slate-300 bg-white px-3 text-sm placeholder-slate-400 focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500 sm:w-64"
        />
        <select
          value={roleFilter}
          onChange={(e) => {
            setRoleFilter(e.target.value as UserRole | '')
            setPage(1)
          }}
          className="h-9 rounded-lg border border-slate-300 bg-white px-3 text-sm text-slate-700 focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
        >
          <option value="">All Roles</option>
          {USER_ROLES.map((r) => (
            <option key={r} value={r}>
              {r}
            </option>
          ))}
        </select>
        <select
          value={statusFilter}
          onChange={(e) => {
            setStatusFilter(e.target.value as UserStatus | '')
            setPage(1)
          }}
          className="h-9 rounded-lg border border-slate-300 bg-white px-3 text-sm text-slate-700 focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
        >
          <option value="">All Statuses</option>
          {USER_STATUSES.map((s) => (
            <option key={s} value={s}>
              {STATUS_LABELS[s]}
            </option>
          ))}
        </select>
        {hasFilters && (
          <button
            onClick={handleReset}
            className="flex h-9 items-center gap-1 rounded-lg border border-slate-300 bg-white px-3 text-sm text-slate-600 hover:bg-slate-50"
          >
            <X className="h-3.5 w-3.5" />
            Reset
          </button>
        )}
      </div>

      {/* Table card */}
      <div className="rounded-xl border border-slate-200 bg-white shadow-sm">
        {isError ? (
          <div className="p-8">
            <ErrorState
              message="Failed to load users."
              onRetry={() => void refetch()}
            />
          </div>
        ) : (
          <>
            <div className="overflow-x-auto">
              <table className="min-w-full">
                <thead>
                  <tr className="border-b border-slate-200 bg-slate-50">
                    <th className="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-slate-500">
                      User
                    </th>
                    <th className="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-slate-500">
                      Role
                    </th>
                    <th className="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-slate-500">
                      Status
                    </th>
                    <th className="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-slate-500">
                      Actions
                    </th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-100">
                  {isLoading
                    ? Array.from({ length: 6 }).map((_, i) => (
                        <tr key={i}>
                          {Array.from({ length: 4 }).map((__, j) => (
                            <td key={j} className="px-6 py-4">
                              <div className="h-4 animate-pulse rounded bg-slate-100" />
                            </td>
                          ))}
                        </tr>
                      ))
                    : paginated.map((user) => (
                        <tr
                          key={user.id}
                          data-testid="user-row"
                          className="hover:bg-slate-50"
                        >
                          <td className="px-6 py-4">
                            <div className="flex items-center gap-3">
                              <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-indigo-100 text-xs font-semibold text-indigo-600">
                                {user.first_name.charAt(0)}
                                {user.last_name.charAt(0)}
                              </div>
                              <div>
                                <p className="text-sm font-medium text-slate-900">
                                  {user.first_name} {user.last_name}
                                </p>
                                <p className="text-xs text-slate-500">
                                  {user.email}
                                </p>
                              </div>
                            </div>
                          </td>
                          <td className="px-6 py-4">
                            <RoleBadge role={user.role} />
                          </td>
                          <td className="px-6 py-4">
                            <StatusBadge status={user.status} />
                          </td>
                          <td className="relative px-6 py-4">
                            <button
                              onClick={() =>
                                setOpenMenuId(
                                  openMenuId === user.id ? null : user.id,
                                )
                              }
                              className="rounded p-1 text-slate-400 hover:bg-slate-100 hover:text-slate-600"
                              aria-label="User actions"
                            >
                              <MoreHorizontal className="h-4 w-4" />
                            </button>
                            {openMenuId === user.id && (
                              <div className="absolute right-4 top-10 z-10 w-40 rounded-lg border border-slate-200 bg-white py-1 shadow-lg">
                                <button
                                  onClick={() => setOpenMenuId(null)}
                                  className="block w-full px-4 py-2 text-left text-sm text-slate-700 hover:bg-slate-50"
                                >
                                  View
                                </button>
                                <button
                                  onClick={() => setOpenMenuId(null)}
                                  className="block w-full px-4 py-2 text-left text-sm text-slate-700 hover:bg-slate-50"
                                >
                                  Edit
                                </button>
                                <button
                                  onClick={() => setOpenMenuId(null)}
                                  className="block w-full px-4 py-2 text-left text-sm text-red-600 hover:bg-slate-50"
                                >
                                  Deactivate
                                </button>
                              </div>
                            )}
                          </td>
                        </tr>
                      ))}
                </tbody>
              </table>
            </div>

            {/* Empty state */}
            {!isLoading && paginated.length === 0 && (
              <div className="py-12">
                <EmptyState message="No users match the current filters." />
              </div>
            )}

            {/* Pagination */}
            {!isLoading && filtered.length > 0 && (
              <div className="flex items-center justify-between border-t border-slate-200 px-6 py-3">
                <p className="text-sm text-slate-500">
                  Showing{' '}
                  {Math.min((currentPage - 1) * PAGE_SIZE + 1, filtered.length)}–
                  {Math.min(currentPage * PAGE_SIZE, filtered.length)} of{' '}
                  {filtered.length} users
                </p>
                <div className="flex items-center gap-1">
                  <button
                    onClick={() => setPage((p) => Math.max(1, p - 1))}
                    disabled={currentPage === 1}
                    className="rounded p-1.5 text-slate-500 hover:bg-slate-100 disabled:cursor-not-allowed disabled:opacity-40"
                    aria-label="Previous page"
                  >
                    <ChevronLeft className="h-4 w-4" />
                  </button>
                  <span className="px-2 text-sm text-slate-700">
                    {currentPage} / {totalPages}
                  </span>
                  <button
                    onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
                    disabled={currentPage === totalPages}
                    className="rounded p-1.5 text-slate-500 hover:bg-slate-100 disabled:cursor-not-allowed disabled:opacity-40"
                    aria-label="Next page"
                  >
                    <ChevronRight className="h-4 w-4" />
                  </button>
                </div>
              </div>
            )}
          </>
        )}
      </div>
    </DashboardLayout>
  )
}

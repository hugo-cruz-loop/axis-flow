import { useQuery } from '@tanstack/react-query'
import { Users, UserCheck, Clock, Shield } from 'lucide-react'
import { useNavigate } from 'react-router-dom'
import * as usersClient from '@/api/usersClient'
import { DashboardLayout } from '@/layouts/DashboardLayout'
import { StatCard } from '@/components/StatCard'
import { RoleBadge } from '@/components/RoleBadge'
import { StatusBadge } from '@/components/StatusBadge'
import type { User } from '@/types/users'

const ADMIN_ROLES = ['ADMIN_CHECK_ON', 'ADMINISTRADOR']

const ACTIVITY_ITEMS = [
  { id: 1, text: 'New user registered', time: '2 min ago' },
  { id: 2, text: 'Role updated for john@example.com', time: '15 min ago' },
  { id: 3, text: 'User activated account', time: '1 hour ago' },
  { id: 4, text: 'Password reset completed', time: '2 hours ago' },
  { id: 5, text: 'New admin assigned', time: '5 hours ago' },
]

export function DashboardPage() {
  const navigate = useNavigate()
  const { data: users, isLoading } = useQuery({
    queryKey: ['users'],
    queryFn: () => usersClient.listAllUsers(),
  })

  const totalUsers = users?.length ?? 0
  const activeUsers = users?.filter((u) => u.status === 'ACTIVE').length ?? 0
  const pendingUsers =
    users?.filter((u) => u.status === 'PENDING_ACTIVATION').length ?? 0
  const adminUsers =
    users?.filter((u) => ADMIN_ROLES.includes(u.role)).length ?? 0

  const recentUsers: User[] = users ? [...users].slice(0, 10) : []

  return (
    <DashboardLayout>
      <div className="mb-6">
        <h1 className="text-2xl font-bold text-slate-900">Dashboard</h1>
        <p className="mt-1 text-sm text-slate-500">
          Welcome back — here's what's happening.
        </p>
      </div>

      {/* Stat cards */}
      <div className="mb-6 grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <StatCard
          icon={Users}
          iconBg="bg-indigo-100"
          iconColor="text-indigo-600"
          label="Total Users"
          value={totalUsers}
          loading={isLoading}
        />
        <StatCard
          icon={UserCheck}
          iconBg="bg-green-100"
          iconColor="text-green-600"
          label="Active Users"
          value={activeUsers}
          loading={isLoading}
          trend="+2 today"
          trendPositive
        />
        <StatCard
          icon={Clock}
          iconBg="bg-amber-100"
          iconColor="text-amber-600"
          label="Pending Activation"
          value={pendingUsers}
          loading={isLoading}
        />
        <StatCard
          icon={Shield}
          iconBg="bg-purple-100"
          iconColor="text-purple-600"
          label="Admins"
          value={adminUsers}
          loading={isLoading}
        />
      </div>

      {/* Table + Activity side by side on large screens */}
      <div className="grid grid-cols-1 gap-6 lg:grid-cols-3">
        {/* Recent Users table */}
        <div className="lg:col-span-2">
          <div className="rounded-xl border border-slate-200 bg-white shadow-sm">
            <div className="flex items-center justify-between border-b border-slate-200 px-6 py-4">
              <h2 className="font-semibold text-slate-900">Recent Users</h2>
              <button
                onClick={() => navigate('/dashboard/users')}
                className="text-sm font-medium text-indigo-600 hover:text-indigo-500"
              >
                View all
              </button>
            </div>
            <div className="overflow-x-auto">
              {isLoading ? (
                <div className="space-y-3 p-6">
                  {Array.from({ length: 5 }).map((_, i) => (
                    <div
                      key={i}
                      className="h-8 animate-pulse rounded bg-slate-100"
                    />
                  ))}
                </div>
              ) : (
                <table className="min-w-full">
                  <thead>
                    <tr className="bg-slate-50">
                      <th className="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-slate-500">
                        Name
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
                    {recentUsers.map((user) => (
                      <tr key={user.id} className="hover:bg-slate-50">
                        <td className="px-6 py-4">
                          <div className="flex items-center gap-3">
                            <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-indigo-100 text-xs font-semibold text-indigo-600">
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
                        <td className="px-6 py-4">
                          <button
                            onClick={() => navigate('/dashboard/users')}
                            className="text-sm font-medium text-indigo-600 hover:text-indigo-500"
                          >
                            View
                          </button>
                        </td>
                      </tr>
                    ))}
                    {recentUsers.length === 0 && !isLoading && (
                      <tr>
                        <td
                          colSpan={4}
                          className="px-6 py-10 text-center text-sm text-slate-500"
                        >
                          No users found
                        </td>
                      </tr>
                    )}
                  </tbody>
                </table>
              )}
            </div>
          </div>
        </div>

        {/* Recent Activity placeholder */}
        <div className="rounded-xl border border-slate-200 bg-white shadow-sm">
          <div className="border-b border-slate-200 px-6 py-4">
            <h2 className="font-semibold text-slate-900">Recent Activity</h2>
          </div>
          <ul className="divide-y divide-slate-100">
            {ACTIVITY_ITEMS.map((item) => (
              <li key={item.id} className="px-6 py-4">
                <p className="text-sm text-slate-700">{item.text}</p>
                <p className="mt-0.5 text-xs text-slate-400">{item.time}</p>
              </li>
            ))}
          </ul>
        </div>
      </div>
    </DashboardLayout>
  )
}

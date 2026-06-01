import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Shield, ChevronDown, ChevronUp } from 'lucide-react'
import * as rolesClient from '@/api/rolesClient'
import type { RoleWithCount } from '@/api/rolesClient'
import { DashboardLayout } from '@/layouts/DashboardLayout'
import { ErrorState } from '@/components/ErrorState'

// ── Permission panel ──────────────────────────────────────────────────────────

interface PermissionsPanelProps {
  roleCode: string
}

function PermissionsPanel({ roleCode }: PermissionsPanelProps) {
  const { data: permissions, isLoading } = useQuery({
    queryKey: ['role-permissions', roleCode],
    queryFn: () => rolesClient.listRolePermissions(roleCode),
  })

  if (isLoading) {
    return (
      <div className="mt-3 space-y-2 border-t border-slate-100 pt-3">
        {Array.from({ length: 3 }).map((_, i) => (
          <div key={i} className="h-4 animate-pulse rounded bg-slate-100" />
        ))}
      </div>
    )
  }

  if (!permissions || permissions.length === 0) {
    return (
      <p className="mt-3 border-t border-slate-100 pt-3 text-sm text-slate-400">
        No permissions assigned
      </p>
    )
  }

  // Group by module
  const byModule = permissions.reduce<Record<string, typeof permissions>>(
    (acc, p) => {
      const mod = p.module || 'general'
      if (!acc[mod]) acc[mod] = []
      acc[mod].push(p)
      return acc
    },
    {},
  )

  return (
    <div className="mt-3 space-y-3 border-t border-slate-100 pt-3">
      {Object.entries(byModule).map(([mod, perms]) => (
        <div key={mod}>
          <p className="mb-1 text-xs font-semibold uppercase tracking-wider text-slate-400">
            {mod}
          </p>
          <ul className="space-y-1">
            {perms.map((p) => (
              <li key={p.id} className="flex items-start gap-2">
                <span className="mt-0.5 h-1.5 w-1.5 shrink-0 rounded-full bg-indigo-400" />
                <span className="font-mono text-xs text-slate-500">{p.code}</span>
                <span className="text-xs text-slate-600">— {p.name}</span>
              </li>
            ))}
          </ul>
        </div>
      ))}
    </div>
  )
}

// ── Role card ─────────────────────────────────────────────────────────────────

interface RoleCardProps {
  role: RoleWithCount
}

function RoleCard({ role }: RoleCardProps) {
  const [expanded, setExpanded] = useState(false)

  return (
    <div className="rounded-xl border border-slate-200 bg-white p-5 shadow-sm transition-shadow hover:shadow-md">
      {/* Top row */}
      <div className="flex items-start justify-between gap-2">
        <span className="font-semibold text-slate-900">{role.name}</span>
        {role.is_system && (
          <span className="shrink-0 rounded-full bg-slate-100 px-2 py-0.5 text-xs text-slate-500">
            System
          </span>
        )}
      </div>

      {/* Code */}
      <p className="mt-1 font-mono text-xs text-slate-500">{role.code}</p>

      {/* Scope badge */}
      <div className="mt-2">
        <span
          className={`inline-flex rounded-full px-2 py-0.5 text-xs font-medium ${
            role.scope === 'GLOBAL'
              ? 'bg-indigo-100 text-indigo-700'
              : 'bg-amber-100 text-amber-700'
          }`}
        >
          {role.scope}
        </span>
      </div>

      {/* Permission count */}
      <p className="mt-2 text-sm text-slate-600">{role.permission_count} permissions</p>

      {/* Toggle button */}
      <button
        onClick={() => setExpanded((v) => !v)}
        className="mt-3 flex items-center gap-1.5 rounded-md border border-slate-300 px-3 py-1.5 text-xs font-medium text-slate-600 hover:bg-slate-50"
      >
        <Shield className="h-3.5 w-3.5" />
        View permissions
        {expanded ? (
          <ChevronUp className="h-3.5 w-3.5" />
        ) : (
          <ChevronDown className="h-3.5 w-3.5" />
        )}
      </button>

      {/* Inline permissions */}
      {expanded && <PermissionsPanel roleCode={role.code} />}
    </div>
  )
}

// ── Page ──────────────────────────────────────────────────────────────────────

export function RolesPage() {
  const { data: roles, isLoading, isError, refetch } = useQuery({
    queryKey: ['roles'],
    queryFn: rolesClient.listRoles,
  })

  return (
    <DashboardLayout>
      {/* Header */}
      <div className="mb-6">
        <h1 className="text-2xl font-bold text-slate-900">Roles</h1>
        <p className="mt-1 text-sm text-slate-500">
          Manage roles and their permissions
        </p>
      </div>

      {isError ? (
        <ErrorState
          message="Failed to load roles."
          onRetry={() => void refetch()}
        />
      ) : (
        <div className="grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-3">
          {isLoading
            ? Array.from({ length: 6 }).map((_, i) => (
                <div
                  key={i}
                  className="h-40 animate-pulse rounded-xl border border-slate-200 bg-white"
                />
              ))
            : roles?.map((role) => <RoleCard key={role.id} role={role} />)}
        </div>
      )}
    </DashboardLayout>
  )
}

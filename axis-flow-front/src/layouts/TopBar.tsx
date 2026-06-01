import { Menu, Bell } from 'lucide-react'
import { useLocation } from 'react-router-dom'
import { useAuthStore } from '@/store/authStore'

const ROUTE_LABELS: Record<string, string> = {
  '/dashboard': 'Dashboard',
  '/dashboard/analytics': 'Analytics',
  '/dashboard/users': 'Users',
  '/dashboard/users/new': 'Create User',
  '/dashboard/roles': 'Roles',
  '/dashboard/settings': 'Settings',
  '/dashboard/help': 'Help',
}

function getPageLabel(pathname: string): string {
  return ROUTE_LABELS[pathname] ?? 'Dashboard'
}

function getInitials(firstName: string, lastName: string): string {
  return `${firstName.charAt(0)}${lastName.charAt(0)}`.toUpperCase()
}

interface TopBarProps {
  onMenuClick: () => void
}

export function TopBar({ onMenuClick }: TopBarProps) {
  const location = useLocation()
  const user = useAuthStore((s) => s.user)
  const pageLabel = getPageLabel(location.pathname)

  return (
    <header className="fixed right-0 top-0 z-20 flex h-16 items-center justify-between border-b border-slate-200 bg-white px-6 shadow-sm lg:left-64">
      {/* Left: hamburger + breadcrumb */}
      <div className="flex items-center gap-4">
        <button
          onClick={onMenuClick}
          className="rounded-md p-1.5 text-slate-500 hover:bg-slate-100 hover:text-slate-700 lg:hidden"
          aria-label="Open navigation"
        >
          <Menu className="h-5 w-5" />
        </button>
        <div className="flex items-center gap-1 text-sm">
          <span className="text-slate-400">AxisFlow</span>
          <span className="text-slate-400">/</span>
          <span className="font-medium text-slate-800">{pageLabel}</span>
        </div>
      </div>

      {/* Right: notifications + user */}
      <div className="flex items-center gap-3">
        <button
          className="relative rounded-md p-1.5 text-slate-500 hover:bg-slate-100 hover:text-slate-700"
          aria-label="Notifications"
        >
          <Bell className="h-5 w-5" />
          <span className="absolute right-1 top-1 h-2 w-2 rounded-full bg-indigo-500" />
        </button>

        <div className="h-6 w-px bg-slate-200" aria-hidden="true" />

        {user && (
          <div className="flex items-center gap-2">
            <div className="flex h-8 w-8 items-center justify-center rounded-full bg-indigo-500 text-xs font-semibold text-white">
              {getInitials(user.first_name, user.last_name)}
            </div>
            <div className="hidden sm:block">
              <p className="text-sm font-medium text-slate-800">
                {user.first_name} {user.last_name}
              </p>
              <p className="text-xs text-slate-500">{user.role}</p>
            </div>
          </div>
        )}
      </div>
    </header>
  )
}

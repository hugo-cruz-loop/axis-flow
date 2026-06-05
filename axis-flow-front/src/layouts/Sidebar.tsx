import { useNavigate, useLocation } from 'react-router-dom'
import {
  LayoutDashboard,
  BarChart3,
  Users,
  Shield,
  Settings,
  HelpCircle,
  LogOut,
  Zap,
  Key,
} from 'lucide-react'
import { useAuthStore } from '@/store/authStore'
import { cn } from '@/lib/utils'

interface NavItem {
  label: string
  icon: React.ElementType
  to: string
}

interface NavGroup {
  title: string
  items: NavItem[]
}

const NAV_GROUPS: NavGroup[] = [
  {
    title: 'Overview',
    items: [
      { label: 'Dashboard', icon: LayoutDashboard, to: '/dashboard' },
      { label: 'Analytics', icon: BarChart3, to: '/dashboard/analytics' },
    ],
  },
  {
    title: 'Management',
    items: [
      { label: 'Users', icon: Users, to: '/dashboard/users' },
      { label: 'Roles', icon: Shield, to: '/dashboard/roles' },
      { label: 'Permissions', icon: Key, to: '/dashboard/permissions' },
    ],
  },
  {
    title: 'System',
    items: [
      { label: 'Settings', icon: Settings, to: '/dashboard/settings' },
      { label: 'Help', icon: HelpCircle, to: '/dashboard/help' },
    ],
  },
]

function getInitials(firstName: string, lastName: string): string {
  return `${firstName.charAt(0)}${lastName.charAt(0)}`.toUpperCase()
}

interface SidebarProps {
  mobileOpen: boolean
  onClose: () => void
}

export function Sidebar({ mobileOpen, onClose }: SidebarProps) {
  const location = useLocation()
  const navigate = useNavigate()
  const { user, clearAuth } = useAuthStore()

  const handleLogout = () => {
    clearAuth()
    navigate('/login', { replace: true })
  }

  const isActive = (to: string) => {
    if (to === '/dashboard') {
      return location.pathname === '/dashboard'
    }
    return location.pathname.startsWith(to)
  }

  const sidebarContent = (
    <div className="flex h-screen w-64 flex-col bg-slate-900">
      {/* Logo */}
      <div className="flex h-16 items-center gap-3 border-b border-slate-800 px-5">
        <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-indigo-500">
          <Zap className="h-5 w-5 text-white" />
        </div>
        <span className="text-xl font-bold text-white">AxisFlow</span>
      </div>

      {/* Navigation */}
      <nav className="flex-1 overflow-y-auto py-4">
        {NAV_GROUPS.map((group) => (
          <div key={group.title} className="mb-4">
            <p className="mb-1 px-4 text-xs font-semibold uppercase tracking-wider text-slate-500">
              {group.title}
            </p>
            {group.items.map((item) => {
              const Icon = item.icon
              const active = isActive(item.to)
              return (
                <button
                  key={item.to}
                  onClick={() => {
                    navigate(item.to)
                    onClose()
                  }}
                  className={cn(
                    'mx-2 flex w-[calc(100%-1rem)] items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium transition-colors',
                    active
                      ? 'bg-indigo-600 text-white'
                      : 'text-slate-400 hover:bg-slate-800 hover:text-white',
                  )}
                >
                  <Icon className="h-4 w-4 shrink-0" />
                  {item.label}
                </button>
              )
            })}
          </div>
        ))}
      </nav>

      {/* User profile */}
      {user && (
        <div className="border-t border-slate-800 p-4">
          <div className="mb-3 flex items-center gap-3">
            <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-indigo-500 text-xs font-semibold text-white">
              {getInitials(user.first_name, user.last_name)}
            </div>
            <div className="min-w-0 flex-1">
              <p className="truncate text-sm font-medium text-white">
                {user.first_name} {user.last_name}
              </p>
              <p className="truncate text-xs text-slate-400">{user.role}</p>
            </div>
          </div>
          <button
            onClick={handleLogout}
            className="flex w-full items-center gap-2 rounded-lg px-3 py-2 text-sm font-medium text-red-400 transition-colors hover:bg-slate-800 hover:text-red-300"
          >
            <LogOut className="h-4 w-4" />
            Sign out
          </button>
        </div>
      )}
    </div>
  )

  return (
    <>
      {/* Desktop sidebar */}
      <aside className="fixed inset-y-0 left-0 z-30 hidden lg:block">
        {sidebarContent}
      </aside>

      {/* Mobile overlay */}
      {mobileOpen && (
        <div className="fixed inset-0 z-40 lg:hidden">
          <div
            className="absolute inset-0 bg-slate-900/60 backdrop-blur-sm"
            onClick={onClose}
            aria-hidden="true"
          />
          <aside className="absolute inset-y-0 left-0 z-50">{sidebarContent}</aside>
        </div>
      )}
    </>
  )
}

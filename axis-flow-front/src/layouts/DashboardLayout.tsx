import { useState } from 'react'
import { Sidebar } from './Sidebar'
import { TopBar } from './TopBar'

interface DashboardLayoutProps {
  children: React.ReactNode
}

export function DashboardLayout({ children }: DashboardLayoutProps) {
  const [mobileOpen, setMobileOpen] = useState(false)

  return (
    <div className="min-h-screen bg-slate-50">
      <Sidebar mobileOpen={mobileOpen} onClose={() => setMobileOpen(false)} />
      <TopBar onMenuClick={() => setMobileOpen(true)} />
      <main className="pt-16 lg:ml-64">
        <div className="px-6 py-6">{children}</div>
      </main>
    </div>
  )
}

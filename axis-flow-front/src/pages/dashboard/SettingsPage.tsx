import { Settings } from 'lucide-react'
import { DashboardLayout } from '@/layouts/DashboardLayout'

export function SettingsPage() {
  return (
    <DashboardLayout>
      <div className="flex min-h-[60vh] items-center justify-center">
        <div className="rounded-xl border border-slate-200 bg-white px-12 py-16 text-center shadow-sm">
          <div className="mx-auto mb-4 flex h-14 w-14 items-center justify-center rounded-full bg-slate-100">
            <Settings className="h-7 w-7 text-slate-600" />
          </div>
          <h2 className="text-xl font-semibold text-slate-900">Settings</h2>
          <p className="mt-2 text-sm text-slate-500">
            Application settings are coming soon.
          </p>
        </div>
      </div>
    </DashboardLayout>
  )
}
